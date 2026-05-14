// Package components provides CRUD operations for components and links in a
// Massdriver project's blueprint.
//
// A component is a bundle slot in a project — the design-time declaration of
// what infrastructure the project consists of. Components are wired together
// by [Link]s, which declare that one component's output should be connected
// to another component's input. At deploy time, each link is realized as a
// runtime [Connection] in the environment.
//
// To list every component or link in a project, use projects.Get and read the
// embedded Components/Links slices — there is no top-level list query.
//
// # Verbs
//
// This package uses [Service.Add] / [Service.Remove] (rather than the
// generic Create/Delete) because a component is bound to an existing
// project blueprint, not allocated standalone — "adding" a component
// to a project models the relationship better. Likewise [Service.AddLink]
// / [Service.RemoveLink] for the wires between components.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Components] field on the top-level SDK client.
package components

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Service is the receiver for component operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Components] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Component is a Massdriver project component — alias of [types.Component].
type Component = types.Component

// Link is a design-time wire between two components — alias of [types.Link].
type Link = types.Link

// AddInput is the input for [Service.Add].
type AddInput struct {
	// ID is a short, memorable identifier (max 20 chars, lowercase
	// alphanumeric) — the third segment of package identifiers like
	// "ecomm-prod-db". Immutable after creation.
	ID string
	// Name is the human-readable display name shown in the UI.
	Name string
	// Description is optional free-text describing what the component is for.
	Description string
	// Attributes are optional key/value tags applied at the component scope.
	Attributes map[string]any
}

// UpdateInput is the input for [Service.Update]. As with projects/environments, an
// empty value sends an empty string; refetch with [Service.Get] and re-send unchanged
// fields if you need merge semantics.
type UpdateInput struct {
	Name        string
	Description string
	Attributes  map[string]any
}

// AddLinkInput is the input for [Service.AddLink].
type AddLinkInput struct {
	// FromComponentID identifies the source component (the producer).
	FromComponentID string
	// FromField is the output field name on the source component.
	FromField string
	// FromVersion is the version constraint to use for the source bundle
	// (e.g. "~1.0", "1.2.3", "latest").
	FromVersion string
	// ToComponentID identifies the destination component (the consumer).
	ToComponentID string
	// ToField is the input field name on the destination component.
	ToField string
	// ToVersion is the version constraint to use for the destination bundle.
	ToVersion string
}

// Get retrieves a component by ID. The returned component includes its parent
// project and its source OCI repository.
func (s *Service) Get(ctx context.Context, id string) (*Component, error) {
	resp, err := gen.GetComponent(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get component %s: %w", id, err))
	}
	if resp.Component.Id == "" {
		return nil, fmt.Errorf("get component %s: %w", id, gql.ErrNotFound)
	}
	return toComponent(resp.Component)
}

// Add adds a new component to a project's blueprint, sourcing it from the
// named OCI repository.
func (s *Service) Add(ctx context.Context, projectID, ociRepoName string, input AddInput) (*Component, error) {
	resp, err := gen.AddComponent(ctx, s.client.GQLv2, s.client.Config.OrganizationID, projectID, ociRepoName, gen.AddComponentInput{
		Id:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Attributes:  input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("add component: %w", err))
	}
	if err := gql.CheckMutation("add component", resp.AddComponent.Successful, resp.AddComponent.Messages); err != nil {
		return nil, err
	}
	return toComponent(resp.AddComponent.Result)
}

// Update updates a component's mutable fields (name, description,
// attributes). The component ID and underlying bundle are immutable.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Component, error) {
	resp, err := gen.UpdateComponent(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateComponentInput{
		Name:        input.Name,
		Description: input.Description,
		Attributes:  input.Attributes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update component %s: %w", id, err))
	}
	if err := gql.CheckMutation("update component", resp.UpdateComponent.Successful, resp.UpdateComponent.Messages); err != nil {
		return nil, err
	}
	return toComponent(resp.UpdateComponent.Result)
}

// Remove removes a component from its project's blueprint, along with all of
// its links. Any deployed instances must be decommissioned first.
func (s *Service) Remove(ctx context.Context, id string) (*Component, error) {
	resp, err := gen.RemoveComponent(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("remove component %s: %w", id, err))
	}
	if err := gql.CheckMutation("remove component", resp.RemoveComponent.Successful, resp.RemoveComponent.Messages); err != nil {
		return nil, err
	}
	return toComponent(resp.RemoveComponent.Result)
}

// AddLink creates a design-time link between two components, declaring that
// the source component's output field is wired to the destination
// component's input field.
func (s *Service) AddLink(ctx context.Context, input AddLinkInput) (*Link, error) {
	resp, err := gen.LinkComponents(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.LinkComponentsInput{
		FromComponentId: input.FromComponentID,
		FromField:       input.FromField,
		FromVersion:     input.FromVersion,
		ToComponentId:   input.ToComponentID,
		ToField:         input.ToField,
		ToVersion:       input.ToVersion,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("link components: %w", err))
	}
	if err := gql.CheckMutation("link components", resp.LinkComponents.Successful, resp.LinkComponents.Messages); err != nil {
		return nil, err
	}
	return toLink(resp.LinkComponents.Result)
}

// RemoveLink removes a link by ID. Existing connections in deployed
// environments are unaffected until the next deploy runs.
func (s *Service) RemoveLink(ctx context.Context, linkID string) (*Link, error) {
	resp, err := gen.UnlinkComponents(ctx, s.client.GQLv2, s.client.Config.OrganizationID, linkID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("remove link %s: %w", linkID, err))
	}
	if err := gql.CheckMutation("remove link", resp.UnlinkComponents.Successful, resp.UnlinkComponents.Messages); err != nil {
		return nil, err
	}
	return toLink(resp.UnlinkComponents.Result)
}

func toComponent(v any) (*Component, error) {
	c := Component{}
	if err := decode.Decode(v, &c); err != nil {
		return nil, fmt.Errorf("decode component: %w", err)
	}
	return &c, nil
}

func toLink(v any) (*Link, error) {
	l := Link{}
	if err := decode.Decode(v, &l); err != nil {
		return nil, fmt.Errorf("decode link: %w", err)
	}
	return &l, nil
}
