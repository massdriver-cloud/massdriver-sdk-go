package groups

import (
	"context"
	"fmt"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// User is a human user account record — alias of [types.Account]. Named
// "User" in this package to mirror the [Service.AddUser] / [Service.RemoveUser]
// terminology callers see.
type User = types.Account

// Invitation is a pending group invitation — alias of [types.GroupInvitation].
type Invitation = types.GroupInvitation

// AddUserResult holds the outcome of [Service.AddUser]. Exactly one of User or
// Invitation is non-nil:
//
//   - User is set when the email already belonged to an organization
//     member. They were added to the group directly.
//   - Invitation is set when the email was new to the organization. An
//     invitation email was sent; the recipient becomes a member when
//     they accept.
type AddUserResult struct {
	User       *User
	Invitation *Invitation
}

// AddUser invites a user to a group by email. If the email already
// belongs to an organization member, they're added to the group
// immediately and [AddUserResult.User] is populated. Otherwise an
// invitation is sent and [AddUserResult.Invitation] is populated.
func (s *Service) AddUser(ctx context.Context, groupID, email string) (*AddUserResult, error) {
	resp, err := gen.AddAccountToGroup(ctx, s.client.GQLv2, s.client.Config.OrganizationID, groupID, gen.AddAccountToGroupInput{
		Email: email,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("add user %s to group %s: %w", email, groupID, err))
	}
	if err := gql.CheckMutation("add user to group", resp.AddAccountToGroup.Successful, resp.AddAccountToGroup.Messages); err != nil {
		return nil, err
	}

	switch r := resp.AddAccountToGroup.Result.(type) {
	case *gen.AddAccountToGroupAddAccountToGroupAddedAccountToGroupPayloadResultAccount:
		return &AddUserResult{
			User: &User{
				ID:        r.Id,
				Email:     r.Email,
				FirstName: r.FirstName,
				LastName:  r.LastName,
			},
		}, nil
	case *gen.AddAccountToGroupAddAccountToGroupAddedAccountToGroupPayloadResultGroupInvitation:
		return &AddUserResult{
			Invitation: &Invitation{
				ID:        r.Id,
				Email:     r.Email,
				CreatedAt: r.CreatedAt,
			},
		}, nil
	default:
		return nil, fmt.Errorf("add user to group: unexpected result type %T", r)
	}
}

// RemoveUser removes a user from a group by email. The user immediately
// loses any access granted by this group; if it was their only group,
// they lose all access to the organization.
func (s *Service) RemoveUser(ctx context.Context, groupID, email string) error {
	resp, err := gen.DeleteGroupMember(ctx, s.client.GQLv2, s.client.Config.OrganizationID, groupID, email)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("remove user %s from group %s: %w", email, groupID, err))
	}
	return gql.CheckMutation("remove group user", resp.DeleteGroupMember.Successful, resp.DeleteGroupMember.Messages)
}

// ServiceAccount is a programmatic API identity — alias of
// [types.ServiceAccount]. Returned by [Service.IterServiceAccounts] /
// [Service.ListServiceAccountsPage]; service-account CRUD lives in the
// serviceaccounts package.
type ServiceAccount = types.ServiceAccount

// ListMembersInput controls a [Service.IterMembers] /
// [Service.ListMembersPage] call. The zero value lists every human member
// of the group, sorted by email ascending.
type ListMembersInput struct {
	// PageSize bounds how many members each underlying request fetches
	// (1..100). Zero lets the server pick its default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page.
	After string
}

// IterMembers returns a lazy [iter.Seq2] over the group's human members,
// fetching pages on demand. Service accounts are listed separately via
// [Service.IterServiceAccounts] — pair both when rendering the full
// membership. To buffer every member into a slice, wrap with
// [types.Collect].
func (s *Service) IterMembers(ctx context.Context, groupID string, input ListMembersInput) iter.Seq2[User, error] {
	return paging.Iter(ctx, input.After, s.membersPage(groupID, input))
}

// ListMembersPage returns a single page of the group's human members.
// input.PageSize bounds the page and input.After (an opaque cursor from a
// prior page's Next) selects which page.
func (s *Service) ListMembersPage(ctx context.Context, groupID string, input ListMembersInput) (types.Page[User], error) {
	return s.membersPage(groupID, input)(ctx, input.After)
}

// membersPage builds the single-page fetcher shared by IterMembers and
// ListMembersPage.
func (s *Service) membersPage(groupID string, input ListMembersInput) paging.FetchFunc[User] {
	return paging.DecodeFetch[User](input.PageSize, "member", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		resp, err := gen.ListGroupMembers(ctx, s.client.GQLv2, s.client.Config.OrganizationID, groupID, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list members for group %s: %w", groupID, err))
		}
		return paging.RawPage{
			Items:    resp.Group.Members.Items,
			Next:     resp.Group.Members.Cursor.Next,
			Previous: resp.Group.Members.Cursor.Previous,
		}, nil
	})
}

// ListServiceAccountsInput controls a [Service.IterServiceAccounts] /
// [Service.ListServiceAccountsPage] call. The zero value lists every
// service account in the group, sorted by name ascending.
type ListServiceAccountsInput struct {
	// PageSize bounds how many service accounts each underlying request
	// fetches (1..100). Zero lets the server pick its default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page.
	After string
}

// IterServiceAccounts returns a lazy [iter.Seq2] over the group's service
// accounts, fetching pages on demand. Human members are listed separately
// via [Service.IterMembers] — pair both when rendering the full membership.
// To buffer every service account into a slice, wrap with [types.Collect].
func (s *Service) IterServiceAccounts(ctx context.Context, groupID string, input ListServiceAccountsInput) iter.Seq2[ServiceAccount, error] {
	return paging.Iter(ctx, input.After, s.serviceAccountsPage(groupID, input))
}

// ListServiceAccountsPage returns a single page of the group's service
// accounts. input.PageSize bounds the page and input.After (an opaque
// cursor from a prior page's Next) selects which page.
func (s *Service) ListServiceAccountsPage(ctx context.Context, groupID string, input ListServiceAccountsInput) (types.Page[ServiceAccount], error) {
	return s.serviceAccountsPage(groupID, input)(ctx, input.After)
}

// serviceAccountsPage builds the single-page fetcher shared by
// IterServiceAccounts and ListServiceAccountsPage.
func (s *Service) serviceAccountsPage(groupID string, input ListServiceAccountsInput) paging.FetchFunc[ServiceAccount] {
	return paging.DecodeFetch[ServiceAccount](input.PageSize, "service account", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		resp, err := gen.ListGroupServiceAccounts(ctx, s.client.GQLv2, s.client.Config.OrganizationID, groupID, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list service accounts for group %s: %w", groupID, err))
		}
		return paging.RawPage{
			Items:    resp.Group.ServiceAccounts.Items,
			Next:     resp.Group.ServiceAccounts.Cursor.Next,
			Previous: resp.Group.ServiceAccounts.Cursor.Previous,
		}, nil
	})
}

// ListInvitationsInput controls a [Service.IterInvitations] /
// [Service.ListInvitationsPage] call. The zero value lists every pending
// invitation on the group.
type ListInvitationsInput struct {
	// PageSize bounds how many invitations each underlying request fetches
	// (1..100). Zero lets the server pick its default.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next, selecting
	// which page to start from. Empty starts at the first page.
	After string
}

// IterInvitations returns a lazy [iter.Seq2] over the group's pending email
// invitations, fetching pages on demand. Once an invitation is accepted it
// becomes a group membership and no longer appears here.
//
// Admin-only: visible to organization admins; other callers receive a
// forbidden error. To buffer every invitation into a slice, wrap with
// [types.Collect].
func (s *Service) IterInvitations(ctx context.Context, groupID string, input ListInvitationsInput) iter.Seq2[Invitation, error] {
	return paging.Iter(ctx, input.After, s.invitationsPage(groupID, input))
}

// ListInvitationsPage returns a single page of the group's pending email
// invitations. input.PageSize bounds the page and input.After (an opaque
// cursor from a prior page's Next) selects which page.
//
// Admin-only: visible to organization admins; other callers receive a
// forbidden error.
func (s *Service) ListInvitationsPage(ctx context.Context, groupID string, input ListInvitationsInput) (types.Page[Invitation], error) {
	return s.invitationsPage(groupID, input)(ctx, input.After)
}

// invitationsPage builds the single-page fetcher shared by IterInvitations
// and ListInvitationsPage.
func (s *Service) invitationsPage(groupID string, input ListInvitationsInput) paging.FetchFunc[Invitation] {
	return paging.DecodeFetch[Invitation](input.PageSize, "invitation", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		resp, err := gen.ListGroupInvitations(ctx, s.client.GQLv2, s.client.Config.OrganizationID, groupID, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list invitations for group %s: %w", groupID, err))
		}
		return paging.RawPage{
			Items:    resp.Group.Invitations.Items,
			Next:     resp.Group.Invitations.Cursor.Next,
			Previous: resp.Group.Invitations.Cursor.Previous,
		}, nil
	})
}

// RevokeInvitation revokes a pending group invitation by email. Has no
// effect if the invitation was already accepted.
func (s *Service) RevokeInvitation(ctx context.Context, groupID, email string) error {
	resp, err := gen.DeleteGroupInvitation(ctx, s.client.GQLv2, s.client.Config.OrganizationID, groupID, email)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("revoke group %s invitation for %s: %w", groupID, email, err))
	}
	return gql.CheckMutation("revoke group invitation", resp.DeleteGroupInvitation.Successful, resp.DeleteGroupInvitation.Messages)
}

// AddServiceAccount adds a service account to the group, granting it
// the group's access level. A service account can belong to multiple
// groups; its effective permissions are the union.
func (s *Service) AddServiceAccount(ctx context.Context, groupID, serviceAccountID string) error {
	resp, err := gen.AddServiceAccountToGroup(ctx, s.client.GQLv2, s.client.Config.OrganizationID, serviceAccountID, groupID)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("add service account %s to group %s: %w", serviceAccountID, groupID, err))
	}
	return gql.CheckMutation("add service account to group", resp.AddServiceAccountToGroup.Successful, resp.AddServiceAccountToGroup.Messages)
}

// RemoveServiceAccount removes a service account from the group. If
// this was its only group, the service account retains its identity
// but loses access to all resources.
func (s *Service) RemoveServiceAccount(ctx context.Context, groupID, serviceAccountID string) error {
	resp, err := gen.RemoveServiceAccountFromGroup(ctx, s.client.GQLv2, s.client.Config.OrganizationID, serviceAccountID, groupID)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("remove service account %s from group %s: %w", serviceAccountID, groupID, err))
	}
	return gql.CheckMutation("remove service account from group", resp.RemoveServiceAccountFromGroup.Successful, resp.RemoveServiceAccountFromGroup.Messages)
}
