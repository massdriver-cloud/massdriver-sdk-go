// Package instances provides operations for Massdriver instances and their
// sub-resources (secrets, alarms, output resources).
//
// An instance is a deployed bundle within an [environment] — the runtime
// realization of a [component]. Instances are not directly created or
// deleted by SDK callers: they appear when a component is added to a
// project's blueprint and an environment is created, and they are torn down
// via deployment actions (DECOMMISSION). The mutations exposed here change
// configuration that takes effect on the next deployment.
//
// Sub-resources fold into this package by file: alarms.go, secrets.go,
// resources.go. They share the same client and follow the same wrapper
// shape as the core instance operations.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Instances] field on the top-level SDK client.
package instances

import (
	"context"
	"fmt"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Instance is a Massdriver instance — alias of [types.Instance].
type Instance = types.Instance

// Service is the receiver for instance operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Instances] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Status is an instance's lifecycle state.
type Status string

const (
	// StatusInitialized means the instance has been created but not yet deployed.
	StatusInitialized Status = "INITIALIZED"
	// StatusProvisioned means infrastructure is deployed and running.
	StatusProvisioned Status = "PROVISIONED"
	// StatusDecommissioned means infrastructure has been torn down (record retained).
	StatusDecommissioned Status = "DECOMMISSIONED"
	// StatusFailed means the most recent deployment failed.
	StatusFailed Status = "FAILED"
)

// SortField is the field a [Service.List] result can be ordered by.
type SortField string

const (
	SortByName      SortField = "NAME"
	SortByCreatedAt SortField = "CREATED_AT"
)

// SortOrder is the direction of a sort.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ListInput controls a [Service.List] call. Zero value lists every instance the
// caller can see.
//
// All set fields are AND'd server-side. The common shapes are
// "all instances in environment X" (set EnvironmentID) or "all failed
// instances of bundle Y" (set Status + OciRepoName).
type ListInput struct {
	// ProjectID limits results to one project.
	ProjectID string
	// EnvironmentID limits results to one environment.
	EnvironmentID string
	// Status limits results to instances in that lifecycle state.
	Status Status
	// OciRepoName limits results to instances of bundles from one repo.
	OciRepoName string

	// SortBy controls sort field. Empty = NAME.
	SortBy SortField
	// SortOrder controls sort direction. Empty = ASC.
	SortOrder SortOrder

	// PageSize sets the cursor page size (1..100). Zero uses the server
	// default.
	PageSize int
}

// UpdateInput is the input for [Service.Update]. Only Version is mutable
// on an instance directly — everything else (params, secrets, connections)
// is changed through deployments or sub-resource mutations.
type UpdateInput struct {
	Version string
}

// CopyInput is the input for [Service.Copy].
type CopyInput struct {
	// Overrides are deep-merged onto the source params before writing
	// to the destination. Useful for environment-specific tweaks.
	Overrides map[string]any
	// Message is attached to the plan deployment created on the
	// destination, similar to a commit message.
	Message string
	// CopySecrets, when true, copies secret values from the source
	// instance to the destination.
	CopySecrets bool
	// CopyRemoteReferences, when true, copies remote resource references
	// from the source instance to the destination.
	CopyRemoteReferences bool
}

// Get retrieves an instance by ID. The returned [Instance] includes
// params, statePaths, the environment/bundle/component refs, and the
// instance's produced [types.Resource]s flattened into Instance.Resources.
//
// The wire shape for resources is a list of `InstanceResource` wrappers
// (each pairing a bundle output handle with the produced resource); the
// wrapper unwraps them to a flat `[]Resource` for ergonomic access.
// Callers who need bundle-handle metadata (e.g. the Required flag) can
// introspect the bundle via platform/bundles.Get.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// instance with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*Instance, error) {
	resp, err := gen.GetInstance(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get instance %s: %w", id, err))
	}
	if resp.Instance.Id == "" {
		return nil, fmt.Errorf("get instance %s: %w", id, gql.ErrNotFound)
	}
	inst, err := toInstance(resp.Instance)
	if err != nil {
		return nil, err
	}

	// The resources wire shape nests the actual Resource under an
	// InstanceResource wrapper (`resources[i].resource`). The mapstructure
	// pass on toInstance can't see through that nesting — flatten here.
	if n := len(resp.Instance.Resources); n > 0 {
		inst.Resources = make([]types.Resource, 0, n)
		for _, ir := range resp.Instance.Resources {
			r := types.Resource{}
			if derr := decode.Decode(ir.Resource, &r); derr != nil {
				return nil, fmt.Errorf("decode instance resource: %w", derr)
			}
			inst.Resources = append(inst.Resources, r)
		}
	}
	return inst, nil
}

// Iter returns a [iter.Seq2] over instances matching the supplied
// filters, fetching pages lazily on demand. Use this for large or
// potentially-unbounded result sets where buffering every match in
// memory ([Service.List]) is impractical.
//
// The yielded error is non-nil exactly when the underlying transport
// or decode failed. Stop iterating after observing one — subsequent
// items are not produced.
//
// Cancellation is via ctx; break out of the range loop to stop
// requesting further pages.
//
// Example:
//
//	for inst, err := range svc.Iter(ctx, instances.ListInput{EnvironmentID: "ecomm-prod"}) {
//	    if err != nil { return err }
//	    process(inst)
//	}
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[Instance, error] {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	return func(yield func(Instance, error) bool) {
		var cursor *scalars.Cursor
		if input.PageSize > 0 {
			cursor = &scalars.Cursor{Limit: input.PageSize}
		}
		for {
			resp, err := gen.ListInstances(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
			if err != nil {
				yield(Instance{}, gql.ClassifyError(fmt.Errorf("list instances: %w", err)))
				return
			}
			for _, item := range resp.Instances.Items {
				inst, ierr := toInstance(item)
				if ierr != nil {
					yield(Instance{}, fmt.Errorf("decode instance: %w", ierr))
					return
				}
				if !yield(*inst, nil) {
					return
				}
			}
			next := resp.Instances.Cursor.Next
			if next == "" {
				return
			}
			cursor = &scalars.Cursor{Next: next}
			if input.PageSize > 0 {
				cursor.Limit = input.PageSize
			}
		}
	}
}

// List returns every instance the caller can see, applying the supplied
// filters and following pagination cursors automatically and buffering
// every match into a single slice. The returned [Instance]s carry slim
// environment/bundle/component refs but no params or statePaths — call
// [Service.Get] on a specific instance to fetch those.
//
// For large result sets — anything where the full match could be tens
// of thousands of rows — prefer [Service.Iter], which yields one
// instance at a time. Cancel ctx to stop early.
func (s *Service) List(ctx context.Context, input ListInput) ([]Instance, error) {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	var (
		out    []Instance
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListInstances(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list instances: %w", err))
		}
		for _, item := range resp.Instances.Items {
			inst, ierr := toInstance(item)
			if ierr != nil {
				return nil, fmt.Errorf("decode instance: %w", ierr)
			}
			out = append(out, *inst)
		}
		next := resp.Instances.Cursor.Next
		if next == "" {
			break
		}
		cursor = &scalars.Cursor{Next: next}
		if input.PageSize > 0 {
			cursor.Limit = input.PageSize
		}
	}
	return out, nil
}

// Update updates an instance's version constraint. Changes take effect
// on the next deployment — the instance's `ResolvedVersion` will reflect
// the new constraint immediately, but `DeployedVersion` only changes
// once a deployment runs.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Instance, error) {
	resp, err := gen.UpdateInstance(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateInstanceInput{
		Version: input.Version,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update instance %s: %w", id, err))
	}
	if err := gql.CheckMutation("update instance", resp.UpdateInstance.Successful, resp.UpdateInstance.Messages); err != nil {
		return nil, err
	}
	return toInstance(resp.UpdateInstance.Result)
}

// Copy copies configuration from sourceID to destinationID. Source and
// destination must be instances of the same component. Source params
// (minus any fields marked non-copyable in the bundle) are written to
// the destination, then a plan deployment is created on the destination
// so the changes can be reviewed before applying.
func (s *Service) Copy(ctx context.Context, sourceID, destinationID string, input CopyInput) (*Instance, error) {
	resp, err := gen.CopyInstance(ctx, s.client.GQLv2, s.client.Config.OrganizationID, sourceID, destinationID, gen.CopyInstanceInput{
		Overrides:            input.Overrides,
		Message:              input.Message,
		CopySecrets:          input.CopySecrets,
		CopyRemoteReferences: input.CopyRemoteReferences,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("copy instance %s → %s: %w", sourceID, destinationID, err))
	}
	if err := gql.CheckMutation("copy instance", resp.CopyInstance.Successful, resp.CopyInstance.Messages); err != nil {
		return nil, err
	}
	return toInstance(resp.CopyInstance.Result)
}

func toInstance(v any) (*Instance, error) {
	inst := Instance{}
	if err := decode.Decode(v, &inst); err != nil {
		return nil, fmt.Errorf("decode instance: %w", err)
	}
	return &inst, nil
}

// buildListFilter compiles a ListInput's narrow-by-* fields into the
// genqlient input. Returns nil when no filter fields are set.
func buildListFilter(input ListInput) *gen.InstancesFilter {
	filter := &gen.InstancesFilter{}
	set := false
	if input.ProjectID != "" {
		filter.ProjectId = &gen.IdFilter{Eq: input.ProjectID}
		set = true
	}
	if input.EnvironmentID != "" {
		filter.EnvironmentId = &gen.IdFilter{Eq: input.EnvironmentID}
		set = true
	}
	if input.Status != "" {
		filter.Status = &gen.InstanceStatusFilter{Eq: gen.InstanceStatus(input.Status)}
		set = true
	}
	if input.OciRepoName != "" {
		filter.OciRepoName = &gen.OciRepoNameFilter{Eq: input.OciRepoName}
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildListSort(input ListInput) *gen.InstancesSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.InstancesSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.InstancesSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.InstancesSort{Field: field, Order: order}
}
