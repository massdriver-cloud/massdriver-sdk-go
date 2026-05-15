package groups

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
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
