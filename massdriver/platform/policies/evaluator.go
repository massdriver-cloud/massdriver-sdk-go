package policies

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Decision is the result of [Service.Evaluate] / one element of
// [Service.EvaluateBatch] — alias of [types.PolicyDecision]. Action and
// EntityID echo the request inputs so batch callers can correlate
// decisions without tracking positions.
type Decision = types.PolicyDecision

// Check is one (action, entityID) pair for [Service.EvaluateBatch].
type Check struct {
	// Action is the action id in `entity:verb` form (e.g. "project:view").
	Action string
	// EntityID is the identifier of the entity (e.g. a project id).
	EntityID string
}

// ExplainInput describes the policy spec to render as plain English.
// Same shape as [CreatePolicyInput] — the explainer accepts a
// proposed policy without requiring it to be saved first.
type ExplainInput struct {
	Effect     Effect
	Actions    []string
	Conditions PolicyConditions
}

// Evaluate asks the server whether the authenticated subject is
// permitted to perform `action` on `entityID`. Returns Allowed=false
// (not an error) for entities that don't exist or belong to a
// different organization, so callers can't probe for existence.
//
// Returns an error when `action` is not in the catalog or is not
// valid against the supplied entity id.
func (s *Service) Evaluate(ctx context.Context, action, entityID string) (*Decision, error) {
	resp, err := gen.EvaluatePolicy(ctx, s.client.GQLv2, s.client.Config.OrganizationID, action, entityID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("evaluate policy %s on %s: %w", action, entityID, err))
	}
	d := Decision{}
	if derr := decode.Decode(resp.EvaluatePolicy, &d); derr != nil {
		return nil, fmt.Errorf("decode policy decision: %w", derr)
	}
	return &d, nil
}

// EvaluateBatch evaluates multiple permission checks in a single
// request. The server caps the batch at 10 entries; passing more
// returns a server-side error.
//
// Decisions are returned in the same order as `checks`; each carries
// its inputs back so callers can correlate without relying on
// positional indices.
func (s *Service) EvaluateBatch(ctx context.Context, checks []Check) ([]Decision, error) {
	in := make([]gen.PolicyDecisionInput, 0, len(checks))
	for _, c := range checks {
		in = append(in, gen.PolicyDecisionInput{Action: c.Action, EntityId: c.EntityID})
	}
	resp, err := gen.EvaluatePolicies(ctx, s.client.GQLv2, s.client.Config.OrganizationID, in)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("evaluate policies: %w", err))
	}
	out := make([]Decision, 0, len(resp.EvaluatePolicies))
	for _, item := range resp.EvaluatePolicies {
		d := Decision{}
		if derr := decode.Decode(item, &d); derr != nil {
			return nil, fmt.Errorf("decode policy decision: %w", derr)
		}
		out = append(out, d)
	}
	return out, nil
}

// Explain renders a proposed policy as plain-English sentences
// describing what it permits or blocks. Useful for showing a preview
// in a policy-authoring UI before saving.
//
// Conditions referencing undeclared custom-attribute keys are
// silently dropped by the explainer — typos surface as a "wider than
// expected" sentence rather than a hard error.
func (s *Service) Explain(ctx context.Context, input ExplainInput) ([]string, error) {
	resp, err := gen.ExplainPolicy(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gen.CreateGroupPolicyInput{
		Effect:     gen.PolicyEffect(input.Effect),
		Actions:    input.Actions,
		Conditions: input.Conditions,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("explain policy: %w", err))
	}
	return resp.ExplainPolicy, nil
}
