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

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
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

// ListInput controls a [Service.List] call. Reserved for future
// filter/sort/page options — the zero value is the only supported
// shape today.
type ListInput struct{}

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

// List returns every project the caller can see in the configured organization,
// sorted by name ascending. Each returned [Project] has its Environments slice
// populated.
func (s *Service) List(ctx context.Context, _ ListInput) ([]Project, error) {
	resp, err := gen.ListProjects(ctx, s.client.GQLv2, s.client.Config.OrganizationID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("list projects: %w", err))
	}
	out := make([]Project, 0, len(resp.Projects.Items))
	for _, item := range resp.Projects.Items {
		p, perr := toProject(item)
		if perr != nil {
			return nil, perr
		}
		out = append(out, *p)
	}
	return out, nil
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
