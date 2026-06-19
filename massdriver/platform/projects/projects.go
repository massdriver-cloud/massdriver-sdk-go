// Package projects provides CRUD operations for Massdriver projects.
//
// A project is the top-level container for related infrastructure. It owns a
// blueprint (the architecture) and one or more environments (the actual
// deployments). See https://docs.massdriver.cloud for the platform model.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Projects] field on the top-level SDK client.
package projects

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

// Project is a Massdriver project — alias of [types.Project] so that callers
// who imported this package work with the canonical type. The same alias is
// re-exported by every package that returns Projects.
type Project = types.Project

// Service is the receiver for project operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Projects] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

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

// CreateInput is the input for [Service.Create].
type CreateInput struct {
	// ID is a short, memorable identifier (max 20 chars, lowercase alphanumeric)
	// that becomes the first segment of every resource ID inside the project.
	// Immutable after creation.
	ID string
	// Name is the human-readable display name shown in the UI/CLI.
	Name string
	// Description is optional free-text describing what the project is for.
	Description string
	// Attributes are optional key/value tags applied at the project scope.
	// Must conform to the organization's custom-attribute schema.
	Attributes map[string]any
}

// UpdateInput is the input for [Service.Update]. All fields are optional in the
// sense that an empty value sends an empty string; the server treats that as
// "set to empty," not "leave unchanged." If you need merge semantics, fetch
// the project first with [Service.Get] and re-send the unchanged fields.
type UpdateInput struct {
	Name        string
	Description string
	Attributes  map[string]any
}

// ListInput controls a [Service.Iter]/[Service.ListPage] call. The zero value
// lists every project, sorted by name ascending.
type ListInput struct {
	// Attributes filters by the project's effective attributes. Each entry
	// targets one attribute key; multiple entries are AND'd together.
	Attributes []types.AttributeFilter

	// SortBy controls the sort field. Empty = NAME.
	SortBy SortField
	// SortOrder controls sort direction. Empty = ASC.
	SortOrder SortOrder

	// PageSize bounds how many projects each underlying request fetches
	// (1..100). Zero lets the server pick its default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page. For Iter it
	// sets the starting page; for ListPage it selects the single page returned.
	After string
}

// Get retrieves a project by ID. The returned [Project] has its Environments
// slice populated.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no project
// with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*Project, error) {
	resp, err := gen.GetProject(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get project %s: %w", id, err))
	}
	if resp.Project.Id == "" {
		return nil, fmt.Errorf("get project %s: %w", id, gql.ErrNotFound)
	}
	return toProject(resp.Project)
}

// Iter returns a lazy [iter.Seq2] over projects matching input, fetching pages
// on demand. It is the recommended way to list: ranging the sequence streams
// results without buffering the whole match set, and breaking out of the loop
// stops requesting further pages. The yielded error is non-nil exactly once, on
// a failed page fetch, after which iteration stops.
//
// Each yielded [Project] has its Environments slice populated. To buffer every
// match into a slice, wrap with [types.Collect].
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[Project, error] {
	return paging.Iter(ctx, input.After, s.page(input))
}

// ListPage returns a single page of projects matching input. input.PageSize
// bounds the page and input.After (an opaque cursor from a prior page's Next)
// selects which page. Use it for stateless pagination — e.g. a UI or CLI that
// hands the returned [types.Page].Next back to its own client to fetch the next
// page on demand.
func (s *Service) ListPage(ctx context.Context, input ListInput) (types.Page[Project], error) {
	return s.page(input)(ctx, input.After)
}

// page builds the single-page fetcher shared by Iter and ListPage.
func (s *Service) page(input ListInput) paging.FetchFunc[Project] {
	filter := buildListFilter(input)
	sort := buildListSort(input)
	limit := input.PageSize
	return func(ctx context.Context, after string) (types.Page[Project], error) {
		resp, err := gen.ListProjects(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, scalars.NewCursor(limit, after))
		if err != nil {
			return types.Page[Project]{}, gql.ClassifyError(fmt.Errorf("list projects: %w", err))
		}
		items := make([]Project, 0, len(resp.Projects.Items))
		for _, item := range resp.Projects.Items {
			p, perr := toProject(item)
			if perr != nil {
				return types.Page[Project]{}, perr
			}
			items = append(items, *p)
		}
		return types.Page[Project]{
			Items:    items,
			Next:     resp.Projects.Cursor.Next,
			Previous: resp.Projects.Cursor.Previous,
		}, nil
	}
}

// buildListFilter compiles a ListInput's filter fields into the generated
// input. Returns nil when no filter fields are set.
func buildListFilter(input ListInput) *gen.ProjectsFilter {
	if len(input.Attributes) == 0 {
		return nil
	}
	return &gen.ProjectsFilter{Attributes: toGenAttributeFilters(input.Attributes)}
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

// buildListSort maps a ListInput's sort fields onto the generated sort input,
// returning nil when the caller left both unset (so the server applies its
// default of name ascending).
func buildListSort(input ListInput) *gen.ProjectsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.ProjectsSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.ProjectsSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.ProjectsSort{Field: field, Order: order}
}

// Create creates a new project. Returns a [*gql.MutationFailedError] (wrapped) if
// the server reports `successful: false` — use [gql.AsMutationFailedError] to
// inspect per-field validation messages. The returned project does not include
// environments since none exist yet.
func (s *Service) Create(ctx context.Context, input CreateInput) (*Project, error) {
	resp, err := gen.CreateProject(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.CreateProjectInput{
		Id:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Attributes:  input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create project: %w", err))
	}
	if err := gql.CheckMutation("create project", resp.CreateProject.Successful, resp.CreateProject.Messages); err != nil {
		return nil, err
	}
	return toProject(resp.CreateProject.Result)
}

// Update updates a project's mutable fields. Returns a [*gql.MutationFailedError]
// (wrapped) if the server reports `successful: false`.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Project, error) {
	resp, err := gen.UpdateProject(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateProjectInput{
		Name:        input.Name,
		Description: input.Description,
		Attributes:  input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update project %s: %w", id, err))
	}
	if err := gql.CheckMutation("update project", resp.UpdateProject.Successful, resp.UpdateProject.Messages); err != nil {
		return nil, err
	}
	return toProject(resp.UpdateProject.Result)
}

// Delete deletes a project by ID. The project must have no remaining
// environments — query the project's `deletable` field via a custom query
// first if you need to check.
func (s *Service) Delete(ctx context.Context, id string) (*Project, error) {
	resp, err := gen.DeleteProject(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete project %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete project", resp.DeleteProject.Successful, resp.DeleteProject.Messages); err != nil {
		return nil, err
	}
	return toProject(resp.DeleteProject.Result)
}

// toProject decodes a genqlient response into a [*Project]. It performs two
// passes: a primary mapstructure decode populates the project's flat fields
// (including Components and Links, which arrive as flat lists), and a
// secondary decode unwraps the paginated `environments.items` page into the
// type's flat Environments slice. This keeps the public type's shape natural
// (flat slices throughout) while accepting the GraphQL pagination envelope on
// the wire where it occurs.
func toProject(v any) (*Project, error) {
	proj := &Project{}
	if err := decode.Decode(v, proj); err != nil {
		return nil, fmt.Errorf("decode project: %w", err)
	}

	type page[T any] struct {
		Items []T `mapstructure:"items"`
	}
	type wrapper struct {
		Environments *page[types.Environment] `mapstructure:"environments"`
	}
	var w wrapper
	if err := decode.Decode(v, &w); err == nil && w.Environments != nil {
		proj.Environments = w.Environments.Items
	}
	return proj, nil
}
