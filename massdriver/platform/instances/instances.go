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

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
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

// SortField is the field a [Service.Iter] result can be ordered by.
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

// ListInput controls a [Service.Iter] call. Zero value lists every instance the
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
	// OciRepoName limits results to instances of bundles from one repo
	// (matches every version published to that repo).
	OciRepoName string
	// BundleID limits results to instances pinned to a specific bundle
	// version ("name@version") or release channel ("name@~1", "name@latest").
	// Use OciRepoName instead to match every version of a bundle.
	BundleID string
	// ParamDimensions filters by configuration parameter values. Each entry
	// targets a specific param field by its jq-style path; multiple entries
	// are AND'd together.
	ParamDimensions []ParamDimensionFilter
	// Attributes filters by effective attributes — the instance's own
	// attributes plus those inherited from its component, environment, and
	// project. Each entry targets one attribute key; multiple entries are
	// AND'd together.
	Attributes []types.AttributeFilter

	// SortBy controls sort field. Empty = NAME.
	SortBy SortField
	// SortOrder controls sort direction. Empty = ASC.
	SortOrder SortOrder

	// PageSize sets the cursor page size (1..100). Zero uses the server
	// default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page. For Iter it
	// sets the starting page; for ListPage it selects the single page returned.
	After string
}

// ParamDimensionFilter narrows an instance list by the value of a single
// configuration parameter. Dimension is the jq-style path to the field (e.g.
// ".database.instance_type"); the remaining fields are matched against that
// field's value — set at most one of Eq, In, or Contains.
type ParamDimensionFilter struct {
	// Dimension is the jq-style path to the parameter (required), e.g.
	// ".database.instance_type" or ".containers[0].image".
	Dimension string
	// Eq matches instances whose value exactly equals this string.
	Eq string
	// In matches instances whose value is any of these strings.
	In []string
	// Contains matches instances whose value contains this substring
	// (case-insensitive).
	Contains string
}

// UpdateInput is the input for [Service.Update]. Only Version is mutable
// on an instance directly — everything else (params, secrets, connections)
// is changed through deployments or sub-resource mutations.
type UpdateInput struct {
	Version string
}

// OrphanInput is the input for [Service.Orphan].
type OrphanInput struct {
	// DeleteState, when true, also deletes the remote Terraform/OpenTofu state
	// files. This is irreversible — the next deployment will provision from
	// scratch and may duplicate any resources tracked by the prior state.
	DeleteState bool
}

// CopyInput is the input for [Service.Copy].
type CopyInput struct {
	// Overrides are deep-merged onto the source params before writing
	// to the destination. Useful for environment-specific tweaks.
	Overrides map[string]any
	// CopySecrets, when true, copies secret values from the source
	// instance to the destination.
	CopySecrets bool
	// CopyRemoteReferences, when true, copies remote resource references
	// from the source instance to the destination.
	CopyRemoteReferences bool
}

// Get retrieves an instance by ID. The returned [Instance] includes
// params, paramsSchema, statePaths, the environment/bundle/component refs,
// and the instance's produced [types.Resource]s flattened into
// Instance.Resources.
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

// Iter returns a lazy [iter.Seq2] over instances matching input, fetching pages
// on demand. It is the recommended way to list: ranging the sequence streams
// results without buffering the whole match set, and breaking out of the loop
// stops requesting further pages. The yielded error is non-nil exactly once, on
// a failed page fetch, after which iteration stops.
//
// The yielded [Instance]s carry slim environment/bundle/component refs but no
// params or statePaths — call [Service.Get] for those. To buffer every match
// into a slice, wrap with [types.Collect].
//
// Example:
//
//	for inst, err := range svc.Iter(ctx, instances.ListInput{EnvironmentID: "ecomm-prod"}) {
//	    if err != nil { return err }
//	    process(inst)
//	}
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[Instance, error] {
	return paging.Iter(ctx, input.After, s.page(input))
}

// ListPage returns a single page of instances matching input. input.PageSize
// bounds the page and input.After (an opaque cursor from a prior page's Next)
// selects which page. Use it for stateless pagination — e.g. a UI or CLI that
// hands the returned [types.Page].Next back to its own client to fetch the next
// page on demand.
func (s *Service) ListPage(ctx context.Context, input ListInput) (types.Page[Instance], error) {
	return s.page(input)(ctx, input.After)
}

// page builds the single-page fetcher shared by Iter and ListPage.
func (s *Service) page(input ListInput) paging.FetchFunc[Instance] {
	filter := buildListFilter(input)
	sort := buildListSort(input)
	limit := input.PageSize
	return func(ctx context.Context, after string) (types.Page[Instance], error) {
		resp, err := gen.ListInstances(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, scalars.NewCursor(limit, after))
		if err != nil {
			return types.Page[Instance]{}, gql.ClassifyError(fmt.Errorf("list instances: %w", err))
		}
		items := make([]Instance, 0, len(resp.Instances.Items))
		for _, item := range resp.Instances.Items {
			inst, ierr := toInstance(item)
			if ierr != nil {
				return types.Page[Instance]{}, ierr
			}
			items = append(items, *inst)
		}
		return types.Page[Instance]{
			Items:    items,
			Next:     resp.Instances.Cursor.Next,
			Previous: resp.Instances.Cursor.Previous,
		}, nil
	}
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

// Orphan is a break-glass operation that resets a permanently-stuck instance
// to INITIALIZED, clearing all Terraform/OpenTofu state locks. Active
// RUNNING, PENDING, and APPROVED deployments are bulk-aborted so a late
// worker callback cannot walk the instance status back to PROVISIONED.
//
// Set OrphanInput.DeleteState to also remove the remote IaC state files;
// the next deployment will then provision from scratch, potentially
// duplicating any resources the prior state was tracking. This is
// irreversible — prefer leaving DeleteState false unless the state is
// known to be unrecoverable.
//
// The returned [Instance] is slim (id, name, status only) — call [Service.Get]
// if you need params, statePaths, or resources after orphaning.
func (s *Service) Orphan(ctx context.Context, id string, input OrphanInput) (*Instance, error) {
	resp, err := gen.OrphanInstance(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.OrphanInstanceInput{
		DeleteState: input.DeleteState,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("orphan instance %s: %w", id, err))
	}
	if err := gql.CheckMutation("orphan instance", resp.OrphanInstance.Successful, resp.OrphanInstance.Messages); err != nil {
		return nil, err
	}
	return toInstance(resp.OrphanInstance.Result)
}

// Copy copies configuration from sourceID to destinationID. Source and
// destination must be instances of the same component. Source params
// (minus any fields marked non-copyable in the bundle) are written to
// the destination, then a plan deployment is created on the destination
// so the changes can be reviewed before applying.
func (s *Service) Copy(ctx context.Context, sourceID, destinationID string, input CopyInput) (*Instance, error) {
	resp, err := gen.CopyInstance(ctx, s.client.GQLv2, s.client.Config.OrganizationID, sourceID, destinationID, gen.CopyInstanceInput{
		Overrides:            input.Overrides,
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
	if input.BundleID != "" {
		filter.BundleId = &gen.BundleIdFilter{Eq: input.BundleID}
		set = true
	}
	if len(input.ParamDimensions) > 0 {
		dims := make([]gen.ParamDimensionFilter, 0, len(input.ParamDimensions))
		for _, d := range input.ParamDimensions {
			dims = append(dims, gen.ParamDimensionFilter{
				Dimension: d.Dimension,
				Eq:        d.Eq,
				In:        d.In,
				Contains:  d.Contains,
			})
		}
		filter.ParamDimension = dims
		set = true
	}
	if len(input.Attributes) > 0 {
		filter.Attributes = toGenAttributeFilters(input.Attributes)
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

// toGenAttributeFilters maps the SDK's attribute filters onto the generated
// input type.
func toGenAttributeFilters(in []types.AttributeFilter) []gen.AttributeFilter {
	out := make([]gen.AttributeFilter, 0, len(in))
	for _, a := range in {
		out = append(out, gen.AttributeFilter{Key: a.Key, Eq: a.Eq, In: a.In})
	}
	return out
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
