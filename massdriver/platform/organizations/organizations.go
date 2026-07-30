// Package organizations provides operations for the Massdriver
// organization record itself, plus its custom-attribute schema and
// member-removal operations.
//
// An [Organization] is the top-level container for everything else
// (projects, environments, the bundle catalog, groups, service accounts).
// Most callers don't need this package — the configured organization id
// is implicit in every other domain operation. Use it when you need to
// inspect organization-level metadata (subscription status, trial
// expiry), declare custom attributes, or remove a member.
//
// Custom attribute CRUD lives in custom_attributes.go in this package.
// Logo upload requires multipart file transport and is not yet exposed.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Organizations] field on the top-level SDK client.
package organizations

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

// Organization is a Massdriver organization — alias of [types.Organization].
type Organization = types.Organization

// Account is a human user record — alias of [types.Account]. Returned by
// [Service.IterMembers] / [Service.ListMembersPage].
type Account = types.Account

// Service is the receiver for organization operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Organizations] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// SubscriptionStatus values surfaced by the server. Use these to gate UI
// affordances or surface billing warnings.
type SubscriptionStatus string

const (
	SubscriptionTrial    SubscriptionStatus = "TRIAL"
	SubscriptionActive   SubscriptionStatus = "ACTIVE"
	SubscriptionPastDue  SubscriptionStatus = "PAST_DUE"
	SubscriptionExpired  SubscriptionStatus = "EXPIRED"
	SubscriptionCanceled SubscriptionStatus = "CANCELED"
)

// CreateInput is the input for [Service.Create]. The caller becomes owner and
// first admin automatically.
type CreateInput struct {
	// ID is a short, memorable identifier (max 20 chars, lowercase
	// alphanumeric). Immutable after creation.
	ID string
	// Name is the human-readable display name.
	Name string
}

// UpdateInput is the input for [Service.Update]. Only the display name is
// mutable; the organization id is fixed at creation.
type UpdateInput struct {
	Name string
}

// Get retrieves the configured organization's metadata. The org's declared
// custom attributes and member roster are paginated sub-lists exposed
// separately — [Service.IterCustomAttributes] and [Service.IterMembers]
// (each with a ListPage variant) — so Get stays a single cheap lookup.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// organization with the configured ID exists.
func (s *Service) Get(ctx context.Context) (*Organization, error) {
	resp, err := gen.GetOrganization(ctx, s.client.GQLv2, s.client.Config.OrganizationID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get organization: %w", err))
	}
	if resp.Organization.Id == "" {
		return nil, fmt.Errorf("get organization: %w", gql.ErrNotFound)
	}
	return toOrganization(resp.Organization)
}

// ListMembersInput controls a [Service.IterMembers] /
// [Service.ListMembersPage] call. The zero value lists every human account
// in the organization, sorted by email ascending.
type ListMembersInput struct {
	// PageSize bounds how many accounts each underlying request fetches
	// (1..100). Zero lets the server pick its default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page.
	After string
}

// IterMembers returns a lazy [iter.Seq2] over every human account in the
// organization, whether or not they belong to a group. Service accounts are
// listed separately via the serviceaccounts package.
//
// Requires the `organization:manageProfile` action (organization admins);
// other callers receive a forbidden error. To buffer every account into a
// slice, wrap with [types.Collect].
func (s *Service) IterMembers(ctx context.Context, input ListMembersInput) iter.Seq2[Account, error] {
	return paging.Iter(ctx, input.After, s.membersPage(input))
}

// ListMembersPage returns a single page of the organization's human
// accounts. input.PageSize bounds the page and input.After (an opaque cursor
// from a prior page's Next) selects which page.
//
// Requires the `organization:manageProfile` action (organization admins);
// other callers receive a forbidden error.
func (s *Service) ListMembersPage(ctx context.Context, input ListMembersInput) (types.Page[Account], error) {
	return s.membersPage(input)(ctx, input.After)
}

// membersPage builds the single-page fetcher shared by IterMembers and
// ListMembersPage.
func (s *Service) membersPage(input ListMembersInput) paging.FetchFunc[Account] {
	return paging.DecodeFetch[Account](input.PageSize, "member", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		resp, err := gen.ListOrganizationMembers(ctx, s.client.GQLv2, s.client.Config.OrganizationID, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list organization members: %w", err))
		}
		return paging.RawPage{
			Items:    resp.Organization.Members.Items,
			Next:     resp.Organization.Members.Cursor.Next,
			Previous: resp.Organization.Members.Cursor.Previous,
		}, nil
	})
}

// Create creates a new organization. The caller becomes owner/first
// admin automatically.
//
// Note: this mutation does not take an organizationId — the configured
// org on the client is irrelevant.
func (s *Service) Create(ctx context.Context, input CreateInput) (*Organization, error) {
	resp, err := gen.CreateOrganization(ctx, s.client.GQLv2, gen.CreateOrganizationInput{
		Id:   input.ID,
		Name: input.Name,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create organization: %w", err))
	}
	if err := gql.CheckMutation("create organization", resp.CreateOrganization.Successful, resp.CreateOrganization.Messages); err != nil {
		return nil, err
	}
	return toOrganization(resp.CreateOrganization.Result)
}

// Update updates the configured organization's display name.
func (s *Service) Update(ctx context.Context, input UpdateInput) (*Organization, error) {
	resp, err := gen.UpdateOrganization(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.UpdateOrganizationInput{
		Name: input.Name,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update organization: %w", err))
	}
	if err := gql.CheckMutation("update organization", resp.UpdateOrganization.Successful, resp.UpdateOrganization.Messages); err != nil {
		return nil, err
	}
	return toOrganization(resp.UpdateOrganization.Result)
}

// RemoveMember removes a user from the organization by email. This
// revokes all their group memberships and cancels any pending invitations
// for that email. The user immediately loses access to all organization
// resources.
func (s *Service) RemoveMember(ctx context.Context, email string) error {
	resp, err := gen.DeleteOrganizationMember(ctx, s.client.GQLv2, s.client.Config.OrganizationID, email)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("remove organization member %s: %w", email, err))
	}
	return gql.CheckMutation("remove organization member", resp.DeleteOrganizationMember.Successful, resp.DeleteOrganizationMember.Messages)
}

func toOrganization(v any) (*Organization, error) {
	o := Organization{}
	if err := decode.Decode(v, &o); err != nil {
		return nil, fmt.Errorf("decode organization: %w", err)
	}
	return &o, nil
}
