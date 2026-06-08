// Package serviceaccounts provides operations for Massdriver service
// accounts — programmatic API client identities, distinct from human
// users.
//
// Service accounts have access tokens for authenticating API requests;
// when one is created, the server issues a default access token alongside
// it (returned exactly once via [Created.DefaultToken]). Subsequent
// tokens for the same service account are issued via the accesstokens
// package after authenticating as that service account.
//
// Service accounts gain permissions by being added to groups. The
// group-membership operations live in platform/groups
// ([groups.AddServiceAccount] / [groups.RemoveServiceAccount]) so all
// add/remove member operations on a group live in one place.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.ServiceAccounts] field on the top-level SDK client.
package serviceaccounts

import (
	"context"
	"fmt"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Service is the receiver for service-account operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.ServiceAccounts] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// ServiceAccount is a Massdriver service account — alias of
// [types.ServiceAccount].
type ServiceAccount = types.ServiceAccount

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

// ListInput controls a [Service.Iter] call. Zero value lists every service
// account in the organization, sorted by name ascending.
type ListInput struct {
	// Search is a case-insensitive substring search across name and
	// description. When set without an explicit SortBy, results rank by
	// relevance.
	Search string
	// IDs restricts results to one or more service accounts by id.
	IDs []string

	SortBy    SortField
	SortOrder SortOrder

	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page. For Iter it
	// sets the starting page; for ListPage it selects the single page returned.
	After string
}

// CreateInput is the input for [Service.Create].
type CreateInput struct {
	// Name is the human-readable display name.
	Name string
	// Description is optional free-text describing what the service
	// account is used for.
	Description string
	// DefaultAccessTokenExpirationInMinutes sets how long the default
	// access token (issued alongside the service account) remains valid.
	// Required. Capped at ~5,256,000 (10 years).
	DefaultAccessTokenExpirationInMinutes int
}

// UpdateInput is the input for [Service.Update]. As with other update inputs, an
// empty value sends an empty string.
type UpdateInput struct {
	Name        string
	Description string
}

// Created is what [Service.Create] returns. The embedded [ServiceAccount] holds
// the metadata; [Created.DefaultToken] is the raw bearer credential of
// the default access token issued alongside — captured ONCE here, never
// retrievable later.
type Created struct {
	ServiceAccount
	// DefaultToken is the raw bearer token of the default access token
	// issued alongside the service account. Store immediately; if lost,
	// revoke and issue a new one via the accesstokens package.
	DefaultToken string
	// DefaultTokenID is the ID of the default access token (so the caller
	// can revoke it later if needed).
	DefaultTokenID string
}

// Get retrieves a service account by ID.
func (s *Service) Get(ctx context.Context, id string) (*ServiceAccount, error) {
	resp, err := gen.GetServiceAccount(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get service account %s: %w", id, err))
	}
	if resp.ServiceAccount.Id == "" {
		return nil, fmt.Errorf("get service account %s: %w", id, gql.ErrNotFound)
	}
	return toServiceAccount(resp.ServiceAccount)
}

// Iter returns a lazy [iter.Seq2] over service accounts matching input,
// fetching pages on demand. It is the recommended way to list: ranging the
// sequence streams results without buffering the whole match set, and breaking
// out of the loop stops requesting further pages. The yielded error is non-nil
// exactly once, on a failed page fetch, after which iteration stops.
//
// To buffer every match into a slice, wrap with [types.Collect].
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[ServiceAccount, error] {
	return paging.Iter(ctx, input.After, s.page(input))
}

// ListPage returns a single page of service accounts matching input.
// input.PageSize bounds the page and input.After (an opaque cursor from a prior
// page's Next) selects which page. Use it for stateless pagination — e.g. a UI
// or CLI that hands the returned [types.Page].Next back to its own client to
// fetch the next page on demand.
func (s *Service) ListPage(ctx context.Context, input ListInput) (types.Page[ServiceAccount], error) {
	return s.page(input)(ctx, input.After)
}

// page builds the single-page fetcher shared by Iter and ListPage.
func (s *Service) page(input ListInput) paging.FetchFunc[ServiceAccount] {
	filter := buildListFilter(input)
	sort := buildListSort(input)
	limit := input.PageSize
	return func(ctx context.Context, after string) (types.Page[ServiceAccount], error) {
		resp, err := gen.ListServiceAccounts(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, scalars.NewCursor(limit, after))
		if err != nil {
			return types.Page[ServiceAccount]{}, gql.ClassifyError(fmt.Errorf("list service accounts: %w", err))
		}
		items := make([]ServiceAccount, 0, len(resp.ServiceAccounts.Items))
		for _, item := range resp.ServiceAccounts.Items {
			sa, derr := toServiceAccount(item)
			if derr != nil {
				return types.Page[ServiceAccount]{}, derr
			}
			items = append(items, *sa)
		}
		return types.Page[ServiceAccount]{
			Items:    items,
			Next:     resp.ServiceAccounts.Cursor.Next,
			Previous: resp.ServiceAccounts.Cursor.Previous,
		}, nil
	}
}

// Create creates a new service account and issues its default access
// token. The raw bearer value is in [Created.DefaultToken] and cannot be
// retrieved later.
func (s *Service) Create(ctx context.Context, input CreateInput) (*Created, error) {
	resp, err := gen.CreateServiceAccount(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.CreateServiceAccountInput{
		Name:                                  input.Name,
		Description:                           input.Description,
		DefaultAccessTokenExpirationInMinutes: input.DefaultAccessTokenExpirationInMinutes,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create service account: %w", err))
	}
	if err := gql.CheckMutation("create service account", resp.CreateServiceAccount.Successful, resp.CreateServiceAccount.Messages); err != nil {
		return nil, err
	}
	r := resp.CreateServiceAccount.Result
	return &Created{
		ServiceAccount: ServiceAccount{
			ID:          r.Id,
			Name:        r.Name,
			Description: r.Description,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		},
		DefaultToken:   r.DefaultAccessToken.Token,
		DefaultTokenID: r.DefaultAccessToken.Id,
	}, nil
}

// Update updates a service account's name and/or description.
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (*ServiceAccount, error) {
	resp, err := gen.UpdateServiceAccount(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id, gen.UpdateServiceAccountInput{
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update service account %s: %w", id, err))
	}
	if err := gql.CheckMutation("update service account", resp.UpdateServiceAccount.Successful, resp.UpdateServiceAccount.Messages); err != nil {
		return nil, err
	}
	return toServiceAccount(resp.UpdateServiceAccount.Result)
}

// Delete deletes a service account permanently. Immediately revokes all
// API access including any active access tokens, and removes all group
// memberships.
func (s *Service) Delete(ctx context.Context, id string) (*ServiceAccount, error) {
	resp, err := gen.DeleteServiceAccount(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("delete service account %s: %w", id, err))
	}
	if err := gql.CheckMutation("delete service account", resp.DeleteServiceAccount.Successful, resp.DeleteServiceAccount.Messages); err != nil {
		return nil, err
	}
	return toServiceAccount(resp.DeleteServiceAccount.Result)
}

func toServiceAccount(v any) (*ServiceAccount, error) {
	sa := ServiceAccount{}
	if err := decode.Decode(v, &sa); err != nil {
		return nil, fmt.Errorf("decode service account: %w", err)
	}
	return &sa, nil
}

func buildListFilter(input ListInput) *gen.ServiceAccountsFilter {
	filter := &gen.ServiceAccountsFilter{}
	set := false
	if input.Search != "" {
		filter.Search = input.Search
		set = true
	}
	if len(input.IDs) > 0 {
		filter.Id = &gen.IdFilter{In: input.IDs}
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildListSort(input ListInput) *gen.ServiceAccountsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.ServiceAccountsSortFieldName
	if input.SortBy == SortByCreatedAt {
		field = gen.ServiceAccountsSortFieldCreatedAt
	}
	order := gen.SortOrderAsc
	if input.SortOrder == SortDesc {
		order = gen.SortOrderDesc
	}
	return &gen.ServiceAccountsSort{Field: field, Order: order}
}
