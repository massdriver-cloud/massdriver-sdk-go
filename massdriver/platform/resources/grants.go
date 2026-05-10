package resources

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Grant is a sharing rule on a resource — alias of [types.Grant]. The
// caller must have `resource:grant` on the source resource to create
// or delete grants.
type Grant = types.Grant

// CreateGrantInput is the input for [Service.CreateGrant].
type CreateGrantInput struct {
	// Action is the action being granted on the resource. Currently
	// the only grantable action is "resource:export" — visibility is
	// inferred from any granted action.
	Action string

	// RecipientConditions restricts the grant to recipient
	// environments matching attribute conditions. Nil (the zero
	// value) is the wildcard — every environment in the org
	// qualifies. A non-nil map describes the required attribute
	// values; per-key, an empty/nil slice is the per-key wildcard
	// (any value of that attribute), a non-empty slice is a closed
	// set.
	//
	// Same convention as [policies.PolicyConditions]; see [types.PolicyConditions]
	// for the wire encoding.
	RecipientConditions types.PolicyConditions
}

// CreateGrant creates a sharing grant on the named resource. Grants
// are immutable — to change action or conditions, delete and
// re-create.
func (s *Service) CreateGrant(ctx context.Context, resourceID string, input CreateGrantInput) (*Grant, error) {
	resp, err := gen.CreateResourceGrant(ctx, s.client.GQLv2, s.client.Config.OrganizationID, resourceID, gen.CreateResourceGrantInput{
		Action:              input.Action,
		RecipientConditions: input.RecipientConditions,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create grant on resource %s: %w", resourceID, err))
	}
	if err := gql.CheckMutation("create resource grant", resp.CreateResourceGrant.Successful, resp.CreateResourceGrant.Messages); err != nil {
		return nil, err
	}
	return toGrant(resp.CreateResourceGrant.Result)
}

// DeleteGrant deletes a grant by ID. The caller must have
// `resource:grant` on the grant's source resource (for resource
// grants) or `repo:grant` on its source repo (for OCI repo grants);
// this same DeleteGrant covers both kinds since the server treats
// grants uniformly by id.
func (s *Service) DeleteGrant(ctx context.Context, grantID string) error {
	resp, err := gen.DeleteGrant(ctx, s.client.GQLv2, s.client.Config.OrganizationID, grantID)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("delete grant %s: %w", grantID, err))
	}
	return gql.CheckMutation("delete grant", resp.DeleteGrant.Successful, resp.DeleteGrant.Messages)
}

// toGrant decodes a genqlient grant result. RecipientConditions
// translation happens through [types.PolicyConditions]'s json
// (un)marshaler, so mapstructure copies the field through natively.
func toGrant(v any) (*Grant, error) {
	g := Grant{}
	if err := decode.Decode(v, &g); err != nil {
		return nil, fmt.Errorf("decode grant: %w", err)
	}
	return &g, nil
}
