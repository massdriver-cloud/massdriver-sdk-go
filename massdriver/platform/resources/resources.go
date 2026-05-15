// Package resources provides operations for Massdriver resources —
// the outputs published by deployed instances and the imported
// resources manually registered in the catalog.
//
// Resource origins:
//
//   - [OriginProvisioned] — created by an instance's deployment.
//     Lifecycle is owned by the instance; the resource record cannot
//     be deleted directly. Updates are limited to the name (the
//     payload is regenerated on each deployment).
//   - [OriginImported] — manually registered. Both name and payload
//     can be updated, and the resource can be deleted directly.
//
// Sensitive values: [Resource.Payload] returns sensitive fields masked
// as "[SENSITIVE]". Use [Service.Export] to retrieve unmasked values along
// with a server-rendered template (json/yaml/etc.); export is recorded in
// the audit log.
//
// Files in this package:
//
//   - resources.go — [Resource] type + Get/List/Create/Update/Delete
//   - export.go    — [Service.Export] returning [Exported] with unmasked payload
//   - grants.go    — [Service.CreateGrant] / [Service.DeleteGrant] for resource sharing
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Resources] field on the top-level SDK client.
package resources

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

// Service is the receiver for resource operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Resources] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Resource is a Massdriver resource — alias of [types.Resource].
type Resource = types.Resource

// Origin is how a resource was created.
type Origin string

const (
	// OriginImported is a manually-registered resource (visible/managed
	// via the API).
	OriginImported Origin = "IMPORTED"
	// OriginProvisioned is a deployment output, owned by the producing
	// instance's lifecycle.
	OriginProvisioned Origin = "PROVISIONED"
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

// ListInput controls a [Service.List] call. Zero value lists every resource
// the caller can see, sorted by name ascending.
type ListInput struct {
	// Origin limits results by origin (imported vs provisioned). Empty
	// matches both.
	Origin Origin

	// ResourceType limits to resources of the given type id (e.g.
	// "aws-iam-role").
	ResourceType string

	// EnvironmentID limits to provisioned resources in the named
	// environment. Imported resources have no environment and are
	// excluded when this filter is set.
	EnvironmentID string

	// Search is a full-text search across the resource name. When set
	// without an explicit SortBy, results rank by relevance.
	Search string

	SortBy    SortField
	SortOrder SortOrder

	PageSize int
}

// CreateInput is the input for [Service.Create] — importing a new resource
// of the named resource type.
type CreateInput struct {
	// Name is a human-readable display name.
	Name string
	// Payload is the resource data conforming to the resource type's
	// schema. Optional — some resource types accept resources with no
	// payload at create time.
	Payload map[string]any
}

// UpdateInput is the input for [Service.Update]. Provisioned resources can
// only have their name changed; imported resources can also update
// the payload. Empty fields are left unchanged.
type UpdateInput struct {
	Name    string
	Payload map[string]any
}

// Get retrieves a resource by ID with its full shape (payload masked,
// resource type, instance ref).
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no resource
// with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*Resource, error) {
	resp, err := gen.GetResource(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get resource %s: %w", id, err))
	}
	if resp.Resource.Id == "" {
		return nil, fmt.Errorf("get resource %s: %w", id, gql.ErrNotFound)
	}
	return toResource(resp.Resource)
}

// Iter returns a [iter.Seq2] over resources matching the supplied
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
//	for r, err := range svc.Iter(ctx, resources.ListInput{Origin: resources.OriginImported}) {
//	    if err != nil { return err }
//	    process(r)
//	}
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[Resource, error] {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	return func(yield func(Resource, error) bool) {
		var cursor *scalars.Cursor
		if input.PageSize > 0 {
			cursor = &scalars.Cursor{Limit: input.PageSize}
		}
		for {
			resp, err := gen.ListResources(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
			if err != nil {
				yield(Resource{}, gql.ClassifyError(fmt.Errorf("list resources: %w", err)))
				return
			}
			for _, item := range resp.Resources.Items {
				r, derr := toResource(item)
				if derr != nil {
					yield(Resource{}, derr)
					return
				}
				if !yield(*r, nil) {
					return
				}
			}
			next := resp.Resources.Cursor.Next
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

// List returns resources matching the supplied filters, following
// pagination cursors automatically and buffering every match into a
// single slice. The result shape excludes the payload (call
// [Service.Get] for the full record) to keep page payloads small.
//
// For large result sets — anything where the full match could be tens
// of thousands of rows — prefer [Service.Iter], which yields one
// resource at a time. Cancel ctx to stop early.
func (s *Service) List(ctx context.Context, input ListInput) ([]Resource, error) {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	var (
		out    []Resource
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListResources(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list resources: %w", err))
		}
		for _, item := range resp.Resources.Items {
			r, derr := toResource(item)
			if derr != nil {
				return nil, derr
			}
			out = append(out, *r)
		}
		next := resp.Resources.Cursor.Next
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

// Create imports a new resource of the named resource type. The
// returned [Resource] has [OriginImported].
func (s *Service) Create(ctx context.Context, resourceTypeID string, input CreateInput) (*Resource, error) {
	resp, err := gen.CreateResource(ctx, s.client.GQLv2, s.client.Config.OrganizationID, resourceTypeID, gen.CreateResourceInput{
		Name:    input.Name,
		Payload: input.Payload,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create resource of type %s: %w", resourceTypeID, err))
	}
	if err := gql.CheckMutation("create resource", resp.CreateResource.Successful, resp.CreateResource.Messages); err != nil {
		return nil, err
	}
	return toResource(resp.CreateResource.Result)
}

// Update updates a resource's name and (for imported resources)
// payload. Provisioned resources only accept name changes; the server
// rejects payload updates with a validation error.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Resource, error) {
	resp, err := gen.UpdateResource(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateResourceInput{
		Name:    input.Name,
		Payload: input.Payload,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update resource %s: %w", id, err))
	}
	if err := gql.CheckMutation("update resource", resp.UpdateResource.Successful, resp.UpdateResource.Messages); err != nil {
		return nil, err
	}
	return toResource(resp.UpdateResource.Result)
}

// Delete deletes an imported resource. Refused for provisioned
// resources (those are tied to their producing instance's lifecycle)
// and for resources currently consumed by active connections —
// disconnect consumers first.
func (s *Service) Delete(ctx context.Context, id string) (*Resource, error) {
	resp, err := gen.DeleteResource(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete resource %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete resource", resp.DeleteResource.Successful, resp.DeleteResource.Messages); err != nil {
		return nil, err
	}
	return toResource(resp.DeleteResource.Result)
}

func toResource(v any) (*Resource, error) {
	r := Resource{}
	if err := decode.Decode(v, &r); err != nil {
		return nil, fmt.Errorf("decode resource: %w", err)
	}
	return &r, nil
}

func buildListFilter(input ListInput) *gen.ResourcesFilter {
	filter := &gen.ResourcesFilter{}
	set := false
	if input.Origin != "" {
		filter.Origin = &gen.ResourceOriginFilter{Eq: gen.ResourceOrigin(input.Origin)}
		set = true
	}
	if input.ResourceType != "" {
		filter.ResourceType = &gen.StringFilter{Eq: input.ResourceType}
		set = true
	}
	if input.EnvironmentID != "" {
		filter.EnvironmentId = &gen.IdFilter{Eq: input.EnvironmentID}
		set = true
	}
	if input.Search != "" {
		filter.Search = input.Search
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildListSort(input ListInput) *gen.ResourcesSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.ResourcesSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.ResourcesSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.ResourcesSort{Field: field, Order: order}
}
