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

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Organization is a Massdriver organization — alias of [types.Organization].
type Organization = types.Organization

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

// Get retrieves the configured organization's metadata.
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
