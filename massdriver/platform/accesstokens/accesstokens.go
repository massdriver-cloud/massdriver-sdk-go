// Package accesstokens provides operations for personal access tokens
// (PATs) issued to the authenticated identity.
//
// Accounts create personal tokens for themselves; service accounts create
// tokens for their own identity. There is no admin view of another user's
// personal tokens — list/create/revoke always operate on the caller's
// own tokens.
//
// The full bearer token value is returned only once at creation time
// ([Created.Token]). Store it immediately — if it's lost, revoke the
// token and create a new one.
//
// # Verbs
//
// [Service.Revoke] (rather than Delete) reflects that a revoked token's
// metadata is retained — the row remains queryable in [Service.List]
// with Status=Revoked so the audit trail is preserved.
//
// Construct a [*Service] with [New] passing the low-level client, or use
// the pre-wired [massdriver.Client.AccessTokens] field on the top-level
// SDK client.
package accesstokens

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

// AccessToken is access-token metadata — alias of [types.AccessToken].
type AccessToken = types.AccessToken

// Service is the receiver for access-token operations. Construct with
// [New]; for the typical case you'll use the
// [massdriver.Client.AccessTokens] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Status filters [Service.List] results by token state.
type Status string

const (
	// StatusAny lists every token regardless of state.
	StatusAny Status = ""
	// StatusActive lists only tokens that are not revoked and not expired.
	StatusActive Status = "active"
	// StatusRevoked lists only revoked tokens.
	StatusRevoked Status = "revoked"
)

// SortField is the field a [Service.List] result can be ordered by.
type SortField string

const (
	SortByCreatedAt SortField = "CREATED_AT"
	SortByExpiresAt SortField = "EXPIRES_AT"
)

// SortOrder is the direction of a sort.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ListInput controls a [Service.List] call. Zero value lists every token visible
// to the caller (active, revoked, and expired).
type ListInput struct {
	Status    Status
	SortBy    SortField
	SortOrder SortOrder
	PageSize  int
}

// CreateInput is the input for [Service.Create].
type CreateInput struct {
	// Name is a human-readable label for identifying the token (e.g.
	// "CI deploy key").
	Name string
	// Scopes is the list of permission scopes for the token. At least one
	// is required; today only ["*"] (full access) is supported.
	Scopes []string
	// ExpiresInMinutes sets how long the token is valid. Zero uses the
	// server default (60 minutes / 1 hour). Maximum ~5,256,000 (10 years).
	ExpiresInMinutes int
}

// Created is what [Service.Create] returns. The embedded [AccessToken] holds the
// metadata; [Created.Token] is the raw bearer credential — captured ONCE
// at creation time and unrecoverable afterwards.
type Created struct {
	AccessToken
	// Token is the raw bearer token. Store immediately; the API never
	// returns it again. If lost, revoke this token and create a new one.
	Token string
}

// List returns the caller's access tokens, filtered and sorted, following
// pagination automatically.
func (s *Service) List(ctx context.Context, input ListInput) ([]AccessToken, error) {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	var (
		out    []AccessToken
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListAccessTokens(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list access tokens: %w", err))
		}
		for _, item := range resp.AccessTokens.Items {
			t, derr := toAccessToken(item)
			if derr != nil {
				return nil, derr
			}
			out = append(out, *t)
		}
		next := resp.AccessTokens.Cursor.Next
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

// Create issues a new access token for the authenticated identity. The
// raw bearer value is in [Created.Token] and cannot be retrieved later.
func (s *Service) Create(ctx context.Context, input CreateInput) (*Created, error) {
	in := gen.CreateAccessTokenInput{
		Name:   input.Name,
		Scopes: input.Scopes,
	}
	if input.ExpiresInMinutes > 0 {
		v := input.ExpiresInMinutes
		in.ExpiresInMinutes = &v
	}

	resp, err := gen.CreateAccessToken(ctx, s.client.GQLv2, s.client.Config.OrganizationID, in)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create access token: %w", err))
	}
	if err := gql.CheckMutation("create access token", resp.CreateAccessToken.Successful, resp.CreateAccessToken.Messages); err != nil {
		return nil, err
	}
	r := resp.CreateAccessToken.Result
	created := &Created{
		AccessToken: AccessToken{
			ID:        r.Id,
			Name:      r.Name,
			Prefix:    r.Prefix,
			Scopes:    r.Scopes,
			ExpiresAt: r.ExpiresAt,
			CreatedAt: r.CreatedAt,
		},
		Token: r.Token,
	}
	return created, nil
}

// Revoke revokes an access token by ID. The token immediately stops
// working for all API requests. Revoking an already-revoked or expired
// token is a no-op that returns the existing record.
func (s *Service) Revoke(ctx context.Context, id string) (*AccessToken, error) {
	resp, err := gen.RevokeAccessToken(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("revoke access token %s: %w", id, err))
	}
	if err := gql.CheckMutation("revoke access token", resp.RevokeAccessToken.Successful, resp.RevokeAccessToken.Messages); err != nil {
		return nil, err
	}
	return toAccessToken(resp.RevokeAccessToken.Result)
}

func toAccessToken(v any) (*AccessToken, error) {
	t := AccessToken{}
	if err := decode.Decode(v, &t); err != nil {
		return nil, fmt.Errorf("decode access token: %w", err)
	}
	return &t, nil
}

func buildListFilter(input ListInput) *gen.AccessTokensFilter {
	switch input.Status {
	case StatusActive:
		v := false
		return &gen.AccessTokensFilter{Revoked: &v}
	case StatusRevoked:
		v := true
		return &gen.AccessTokensFilter{Revoked: &v}
	case StatusAny:
		return nil
	}
	return nil
}

func buildListSort(input ListInput) *gen.AccessTokensSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.AccessTokensSortFieldCreatedAt
	if input.SortBy == SortByExpiresAt {
		field = gen.AccessTokensSortFieldExpiresAt
	}
	order := gen.SortOrderDesc
	if input.SortOrder == SortAsc {
		order = gen.SortOrderAsc
	}
	return &gen.AccessTokensSort{Field: field, Order: order}
}
