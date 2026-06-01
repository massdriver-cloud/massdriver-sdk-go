// Package environments provides CRUD operations for Massdriver environments
// and their environment-default resource bindings.
//
// An environment is an isolated deployment context (production, staging, dev,
// etc.) within a project. Each environment can have a set of "default"
// resources whose presence is automatically inherited by every instance in
// the environment whose connection schema matches the resource type.
//
// # Verbs
//
// [Service.SetDefault] / [Service.RemoveDefault] manage the default-resource
// bindings — Set/Remove (rather than Create/Delete) because the binding is
// a pointer between an environment and an existing resource, not a record
// allocated by this call.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Environments] field on the top-level SDK client.
package environments

import (
	"context"
	"fmt"
	"iter"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Service is the receiver for environment operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Environments] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Environment is a Massdriver environment — alias of [types.Environment].
//
// Embedded Project carries the parent project's id, name, description,
// attributes, and timestamps. Nested fields on the project (its environments,
// components, links) are not populated by environment queries — call
// projects.Get to fetch them.
type Environment = types.Environment

// Project is re-exported so callers working through this package can refer to
// the parent type without importing platform/types directly. It is the same
// type as projects.Project — type identity is preserved across the SDK.
type Project = types.Project

// CreateInput is the input for [Service.Create].
type CreateInput struct {
	// ID is a short, memorable identifier (max 20 chars, lowercase
	// alphanumeric) — the second segment of package identifiers like
	// "ecomm-prod-db". Immutable after creation.
	ID string
	// Name is the human-readable display name shown in the UI/CLI.
	Name string
	// Description is optional free-text describing what the environment is for.
	Description string
	// Attributes are optional key/value tags applied at the environment scope.
	Attributes map[string]any
}

// UpdateInput is the input for [Service.Update]. As with projects, an empty value
// sends an empty string; refetch with [Service.Get] and re-send unchanged fields if
// you need merge semantics.
type UpdateInput struct {
	Name        string
	Description string
	Attributes  map[string]any
}

// SortField is the field a [Service.Iter]/[Service.ListPage] result can be
// ordered by.
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

// ListInput narrows what [Service.Iter]/[Service.ListPage] returns. The zero
// value lists every environment, sorted by name ascending.
type ListInput struct {
	// ProjectID limits results to one project.
	ProjectID string
	// IDs limits results to the named environments.
	IDs []string

	// SortBy controls the sort field. Empty = NAME.
	SortBy SortField
	// SortOrder controls sort direction. Empty = ASC.
	SortOrder SortOrder

	// PageSize bounds how many environments each underlying request fetches
	// (1..100). Zero lets the server pick its default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page. For Iter it
	// sets the starting page; for ListPage it selects the single page returned.
	After string
}

// ForkInput is the input for [Service.Fork].
type ForkInput struct {
	// ID is the new environment's identifier (max 20 chars, lowercase
	// alphanumeric). Immutable after creation.
	ID string
	// Name is the human-readable display name shown in the UI/CLI.
	Name string
	// Description is optional free-text describing the fork's purpose.
	Description string
	// Attributes are optional key/value tags applied at the environment scope.
	Attributes map[string]any
	// CopySecrets, when true, copies every package's secret values from
	// the parent into the fork.
	CopySecrets bool
	// CopyRemoteReferences, when true, copies every package's remote
	// resource references from the parent into the fork.
	CopyRemoteReferences bool
	// CopyEnvironmentDefaults, when true, copies the parent's default
	// resource connections into the fork.
	CopyEnvironmentDefaults bool
}

// EnvironmentDefault is a resource pre-assigned to an environment so that
// instances inherit it automatically when their connection schema matches the
// resource type.
//
// Resource carries the slim selection the underlying query returns
// (id, name, resourceType{id, name, icon}); fields not selected (Field,
// Formats, Payload, Attributes, Instance) stay zero. Call
// platform/resources.Get for the full resource shape.
type EnvironmentDefault struct {
	ID        string          `json:"id" mapstructure:"id"`
	CreatedAt time.Time       `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	Resource  *types.Resource `json:"resource,omitempty" mapstructure:"resource,omitempty"`
}

// Get retrieves an environment by ID.
func (s *Service) Get(ctx context.Context, id string) (*Environment, error) {
	resp, err := gen.GetEnvironment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get environment %s: %w", id, err))
	}
	if resp.Environment.Id == "" {
		return nil, fmt.Errorf("get environment %s: %w", id, gql.ErrNotFound)
	}
	return toEnvironment(resp.Environment)
}

// Iter returns a lazy [iter.Seq2] over environments matching input, fetching
// pages on demand. It is the recommended way to list: ranging the sequence
// streams results without buffering the whole match set, and breaking out of
// the loop stops requesting further pages. The yielded error is non-nil exactly
// once, on a failed page fetch, after which iteration stops.
//
// To buffer every match into a slice, wrap with [types.Collect].
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[Environment, error] {
	return paging.Iter(ctx, input.After, s.page(input))
}

// ListPage returns a single page of environments matching input. input.PageSize
// bounds the page and input.After (an opaque cursor from a prior page's Next)
// selects which page. Use it for stateless pagination — e.g. a UI or CLI that
// hands the returned [types.Page].Next back to its own client to fetch the next
// page on demand.
func (s *Service) ListPage(ctx context.Context, input ListInput) (types.Page[Environment], error) {
	return s.page(input)(ctx, input.After)
}

// page builds the single-page fetcher shared by Iter and ListPage.
func (s *Service) page(input ListInput) paging.FetchFunc[Environment] {
	filter := buildListFilter(input)
	sort := buildListSort(input)
	limit := input.PageSize
	return func(ctx context.Context, after string) (types.Page[Environment], error) {
		resp, err := gen.ListEnvironments(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, scalars.NewCursor(limit, after))
		if err != nil {
			return types.Page[Environment]{}, gql.ClassifyError(fmt.Errorf("list environments: %w", err))
		}
		items := make([]Environment, 0, len(resp.Environments.Items))
		for _, item := range resp.Environments.Items {
			e, eerr := toEnvironment(item)
			if eerr != nil {
				return types.Page[Environment]{}, fmt.Errorf("decode environment: %w", eerr)
			}
			items = append(items, *e)
		}
		return types.Page[Environment]{
			Items:    items,
			Next:     resp.Environments.Cursor.Next,
			Previous: resp.Environments.Cursor.Previous,
		}, nil
	}
}

func buildListSort(input ListInput) *gen.EnvironmentsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.EnvironmentsSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.EnvironmentsSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.EnvironmentsSort{Field: field, Order: order}
}

func buildListFilter(input ListInput) *gen.EnvironmentsFilter {
	filter := &gen.EnvironmentsFilter{}
	set := false
	if input.ProjectID != "" {
		filter.ProjectId = &gen.IdFilter{Eq: input.ProjectID}
		set = true
	}
	if len(input.IDs) > 0 {
		filter.Id = &gen.StringFilter{In: input.IDs}
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

// Create creates a new environment under the named project. Returns a
// [*gql.MutationFailedError] (wrapped) if the server reports `successful: false`.
func (s *Service) Create(ctx context.Context, projectID string, input CreateInput) (*Environment, error) {
	resp, err := gen.CreateEnvironment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, projectID, gen.CreateEnvironmentInput{
		Id:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Attributes:  input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create environment in project %s: %w", projectID, err))
	}
	if err := gql.CheckMutation("create environment", resp.CreateEnvironment.Successful, resp.CreateEnvironment.Messages); err != nil {
		return nil, err
	}
	return toEnvironment(resp.CreateEnvironment.Result)
}

// Update updates an environment's mutable fields.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Environment, error) {
	resp, err := gen.UpdateEnvironment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateEnvironmentInput{
		Name:        input.Name,
		Description: input.Description,
		Attributes:  input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update environment %s: %w", id, err))
	}
	if err := gql.CheckMutation("update environment", resp.UpdateEnvironment.Successful, resp.UpdateEnvironment.Messages); err != nil {
		return nil, err
	}
	return toEnvironment(resp.UpdateEnvironment.Result)
}

// Delete deletes an environment. The environment must have no remaining
// instances — query the environment's `deletable` field first if you need
// to check.
func (s *Service) Delete(ctx context.Context, id string) (*Environment, error) {
	resp, err := gen.DeleteEnvironment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete environment %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete environment", resp.DeleteEnvironment.Successful, resp.DeleteEnvironment.Messages); err != nil {
		return nil, err
	}
	return toEnvironment(resp.DeleteEnvironment.Result)
}

// Fork creates a new environment by forking an existing one. Instances
// are initialized from the project's components and seeded with the
// parent's instance params.
//
// Idempotent: re-forking with the same [ForkInput.ID] against the same
// parent returns the existing environment after re-applying the
// [ForkInput.Copy*] flags (so it acts as a desired-state converge).
// Re-forking with the same ID but a different parent is rejected — a
// fork's parent is immutable.
func (s *Service) Fork(ctx context.Context, parentID string, input ForkInput) (*Environment, error) {
	resp, err := gen.ForkEnvironment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, parentID, gen.ForkEnvironmentInput{
		Id:                      input.ID,
		Name:                    input.Name,
		Description:             input.Description,
		Attributes:              input.Attributes,
		CopySecrets:             input.CopySecrets,
		CopyRemoteReferences:    input.CopyRemoteReferences,
		CopyEnvironmentDefaults: input.CopyEnvironmentDefaults,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("fork environment from %s: %w", parentID, err))
	}
	if err := gql.CheckMutation("fork environment", resp.ForkEnvironment.Successful, resp.ForkEnvironment.Messages); err != nil {
		return nil, err
	}
	return toEnvironment(resp.ForkEnvironment.Result)
}

// Deploy schedules a deployment of every instance in the environment in
// dependency order. Cancels any in-flight environment deployment and
// enqueues a fresh provision wave. Returns as soon as the deployment is
// enqueued — the infrastructure changes happen asynchronously.
//
// Idempotent at the deploy layer: calling Deploy repeatedly while a
// previous wave is still pending is safe; the prior pending wave is
// cancelled before the new one is scheduled.
func (s *Service) Deploy(ctx context.Context, id string) (*Environment, error) {
	resp, err := gen.DeployEnvironment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("deploy environment %s: %w", id, err))
	}
	if err := gql.CheckMutation("deploy environment", resp.DeployEnvironment.Successful, resp.DeployEnvironment.Messages); err != nil {
		return nil, err
	}
	return toEnvironment(resp.DeployEnvironment.Result)
}

// Decommission schedules a teardown of every instance in the environment in
// reverse dependency order. The environment shell stays in place so it can
// be redeployed; call [Service.Delete] to remove the empty environment
// afterwards. Cancels any in-flight environment deployment and enqueues a
// fresh decommission wave. Returns as soon as the wave is enqueued — the
// infrastructure changes happen asynchronously.
//
// Blocked when the environment has `decommissionProtection: true` — disable
// it via [Service.Update] before calling.
func (s *Service) Decommission(ctx context.Context, id string) (*Environment, error) {
	resp, err := gen.DecommissionEnvironment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("decommission environment %s: %w", id, err))
	}
	if err := gql.CheckMutation("decommission environment", resp.DecommissionEnvironment.Successful, resp.DecommissionEnvironment.Messages); err != nil {
		return nil, err
	}
	return toEnvironment(resp.DecommissionEnvironment.Result)
}

// SetDefault marks the named resource as the default of its type for the
// environment. Only one resource per type can be the default; remove the
// existing one with [Service.RemoveDefault] before changing it.
func (s *Service) SetDefault(ctx context.Context, environmentID, resourceID string) (*EnvironmentDefault, error) {
	resp, err := gen.SetEnvironmentDefault(ctx, s.client.GQLv2, s.client.Config.OrganizationID, environmentID, resourceID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("set environment %s default to %s: %w", environmentID, resourceID, err))
	}
	if err := gql.CheckMutation("set environment default", resp.SetEnvironmentDefault.Successful, resp.SetEnvironmentDefault.Messages); err != nil {
		return nil, err
	}
	return toEnvironmentDefault(resp.SetEnvironmentDefault.Result)
}

// RemoveDefault removes an environment-default by ID. Instances that depend
// on the cleared resource type will fall back to whatever the next deploy
// resolves — be careful, this can break in-flight deployments.
func (s *Service) RemoveDefault(ctx context.Context, id string) (*EnvironmentDefault, error) {
	resp, err := gen.RemoveEnvironmentDefault(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("remove environment default %s: %w", id, err))
	}
	if err := gql.CheckMutation("remove environment default", resp.RemoveEnvironmentDefault.Successful, resp.RemoveEnvironmentDefault.Messages); err != nil {
		return nil, err
	}
	return toEnvironmentDefault(resp.RemoveEnvironmentDefault.Result)
}

func toEnvironment(v any) (*Environment, error) {
	e := Environment{}
	if err := decode.Decode(v, &e); err != nil {
		return nil, fmt.Errorf("decode environment: %w", err)
	}
	return &e, nil
}

func toEnvironmentDefault(v any) (*EnvironmentDefault, error) {
	d := EnvironmentDefault{}
	if err := decode.Decode(v, &d); err != nil {
		return nil, fmt.Errorf("decode environment default: %w", err)
	}
	return &d, nil
}
