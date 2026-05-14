// Package groups provides operations for Massdriver groups, the primary
// access-control primitive in the platform.
//
// A [Group] is a collection of users and service accounts that share an
// access level. Groups gain permissions through ABAC [Policy]s attached
// to them — see policies.go in this package for policy CRUD.
//
// Members and invitations are managed via the helpers in members.go.
// Service-account membership is managed via the serviceaccounts package
// (serviceaccounts.AddToGroup / RemoveFromGroup).
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Groups] field on the top-level SDK client.
package groups

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Group is a Massdriver group — alias of [types.Group].
type Group = types.Group

// Service is the receiver for group operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Groups] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Role identifies a [Group]'s access level. Built-in groups have
// [RoleOrganizationAdmin] or [RoleOrganizationViewer]; custom groups have
// [RoleCustom] and grant access via attached policies.
type Role string

const (
	RoleOrganizationAdmin  Role = "ORGANIZATION_ADMIN"
	RoleOrganizationViewer Role = "ORGANIZATION_VIEWER"
	RoleCustom             Role = "CUSTOM"
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

// ListInput controls a [Service.List] call. Zero value lists every group, sorted
// by name ascending.
type ListInput struct {
	SortBy    SortField
	SortOrder SortOrder
	PageSize  int
}

// CreateInput is the input for [Service.Create]. New groups always have role
// [RoleCustom]; that is not configurable via this input.
type CreateInput struct {
	Name        string
	Description string
}

// UpdateInput is the input for [Service.Update]. Only Name and Description are
// mutable; the group's role is immutable.
type UpdateInput struct {
	Name        string
	Description string
}

// Get retrieves a group by ID.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no group
// with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*Group, error) {
	resp, err := gen.GetGroup(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get group %s: %w", id, err))
	}
	if resp.Group.Id == "" {
		return nil, fmt.Errorf("get group %s: %w", id, gql.ErrNotFound)
	}
	return toGroup(resp.Group)
}

// List returns groups in the organization, applying sort options and
// following pagination automatically.
func (s *Service) List(ctx context.Context, input ListInput) ([]Group, error) {
	sort := buildListSort(input)

	var (
		out    []Group
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListGroups(ctx, s.client.GQLv2, s.client.Config.OrganizationID, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list groups: %w", err))
		}
		for _, item := range resp.Groups.Items {
			g, derr := toGroup(item)
			if derr != nil {
				return nil, derr
			}
			out = append(out, *g)
		}
		next := resp.Groups.Cursor.Next
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

// Create creates a new custom group. Returns a [*gql.MutationFailedError]
// (wrapped) if the server reports `successful: false`.
func (s *Service) Create(ctx context.Context, input CreateInput) (*Group, error) {
	resp, err := gen.CreateGroup(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.CreateGroupInput{
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create group: %w", err))
	}
	if err := gql.CheckMutation("create group", resp.CreateGroup.Successful, resp.CreateGroup.Messages); err != nil {
		return nil, err
	}
	return toGroup(resp.CreateGroup.Result)
}

// Update updates a group's name and/or description.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*Group, error) {
	resp, err := gen.UpdateGroup(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateGroupInput{
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update group %s: %w", id, err))
	}
	if err := gql.CheckMutation("update group", resp.UpdateGroup.Successful, resp.UpdateGroup.Messages); err != nil {
		return nil, err
	}
	return toGroup(resp.UpdateGroup.Result)
}

// Delete deletes a custom group. Built-in groups (Admins, Viewers) cannot
// be deleted — the API rejects those requests.
func (s *Service) Delete(ctx context.Context, id string) (*Group, error) {
	resp, err := gen.DeleteGroup(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete group %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete group", resp.DeleteGroup.Successful, resp.DeleteGroup.Messages); err != nil {
		return nil, err
	}
	return toGroup(resp.DeleteGroup.Result)
}

func toGroup(v any) (*Group, error) {
	g := Group{}
	if err := decode.Decode(v, &g); err != nil {
		return nil, fmt.Errorf("decode group: %w", err)
	}
	return &g, nil
}

func buildListSort(input ListInput) *gen.GroupsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.GroupsSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.GroupsSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.GroupsSort{Field: field, Order: order}
}
