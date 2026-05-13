// Package policies provides ABAC policy primitives — the policies
// themselves (CRUD against a group), the runtime catalog of actions and
// entities, the policy evaluator, and the policy explainer.
//
// Policies are attached to groups (the principal); each policy grants
// or denies one or more actions on entities whose attributes satisfy
// optional conditions. Group management (members, roles, the group
// itself) lives in platform/groups; this package owns everything else
// in the ABAC model.
//
// The action catalog is exposed at runtime ([Service.ListActions]) rather
// than as Go constants. The server's catalog is "small and static" but
// modeled as queryable data so it can grow without breaking clients —
// an SDK that hardcoded constants would silently lag the server.
//
// Files in this package:
//
//   - policies.go    — [Policy] type + [Service.Create], [Service.Update],
//     [Service.Delete] plus the conditions plumbing ([Wildcard],
//     [WildcardConditions], [Effect] constants, [PolicyConditions] alias).
//   - catalog.go     — [Action] / [Entity] types + [Service.ListActions] /
//     [Service.ListEntities].
//   - evaluator.go   — [Decision] + [Check] + [Service.Evaluate] /
//     [Service.EvaluateBatch], plus [Service.Explain] for human-readable
//     rendering of a proposed policy.
//   - attributes.go  — [Service.CustomAttributeSchema] /
//     [Service.CustomAttributeValues] for policy-authoring UIs.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Policies] field on the top-level SDK client.
package policies

import (
	"context"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Service is the receiver for policy operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Policies] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Policy is one ABAC rule attached to a group — alias of [types.Policy].
type Policy = types.Policy

// PolicyConditions is the condition set for a policy — alias of
// [types.PolicyConditions].
//
//   - Nil map: whole-policy wildcard (matches every entity).
//   - Non-nil map: per-attribute conditions.
//
// Per-key values follow the same convention via the [Wildcard] var:
// nil/empty slice means "any value of this attribute," non-empty means
// "value must be in this set."
type PolicyConditions = types.PolicyConditions

// Wildcard is the per-key value used inside a [PolicyConditions] map to
// match any value of an attribute key. The entity must HAVE the
// attribute set; any value is accepted.
//
// Equivalent to a nil or empty []string — the variable exists to make
// caller intent explicit:
//
//	policies.PolicyConditions{
//	    "md-project": policies.Wildcard,        // any md-project value
//	    "md-team":    {"platform", "data"},     // closed set
//	}
//
// Distinct from [WildcardConditions], which is for the *whole-policy*
// wildcard sentinel used in [UpdatePolicyInput].
//
// Treat as read-only — reassigning this variable would break every
// other caller in the program.
var Wildcard []string

// Effect is whether a [Policy] grants or denies its actions.
type Effect string

const (
	EffectAllow Effect = "ALLOW"
	EffectDeny  Effect = "DENY"
)

// CreatePolicyInput is the input for [Service.Create]. The policy is
// attached to the named group.
type CreatePolicyInput struct {
	// Effect determines whether the policy grants ([EffectAllow]) or
	// blocks ([EffectDeny]) the actions.
	Effect Effect
	// Actions is the list of action ids in `entity:verb` form (e.g.
	// "project:view", "instance:deploy"). At least one is required.
	// Use [Service.ListActions] to enumerate the catalog.
	Actions []string
	// Conditions restricts the policy. Nil (the zero value) is the
	// wildcard — the policy matches every entity of each action's
	// type. A non-nil map describes the required attribute values; the
	// policy only matches entities whose attributes satisfy every key.
	Conditions PolicyConditions
}

// UpdatePolicyInput is the input for [Service.Update]. Empty fields are
// left unchanged.
type UpdatePolicyInput struct {
	// Effect, when non-empty, replaces the policy's effect.
	Effect Effect
	// Actions, when non-nil, replaces the policy's full action list.
	Actions []string
	// Conditions, when non-nil, replaces the policy's conditions.
	//
	// To leave conditions unchanged, leave this nil. To set the policy
	// back to wildcard, use [WildcardConditions]. To set attribute
	// conditions, take the address of a populated [PolicyConditions]
	// map: `&PolicyConditions{"key": {"val"}}`.
	Conditions *PolicyConditions
}

// WildcardConditions returns a non-nil *[PolicyConditions] whose
// underlying map is nil — the sentinel that [Service.Update] uses to
// distinguish "set conditions to wildcard" from "leave conditions
// unchanged" (which uses a nil pointer).
func WildcardConditions() *PolicyConditions {
	var c PolicyConditions
	return &c
}

// Get retrieves a single policy by its ID.
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no
// policy with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, policyID string) (*Policy, error) {
	resp, err := gen.GetGroupPolicy(ctx, s.client.GQLv2, s.client.Config.OrganizationID, policyID)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get policy %s: %w", policyID, err))
	}
	if resp.GroupPolicy.Id == "" {
		return nil, fmt.Errorf("get policy %s: %w", policyID, gql.ErrNotFound)
	}
	return toPolicy(resp.GroupPolicy)
}

// Create attaches an ABAC policy to the named group.
//
// Pass nil for [CreatePolicyInput.Conditions] to create a wildcard
// policy that matches every entity. Pass a populated [PolicyConditions]
// map for attribute conditions.
func (s *Service) Create(ctx context.Context, groupID string, input CreatePolicyInput) (*Policy, error) {
	resp, err := gen.CreateGroupPolicy(ctx, s.client.GQLv2, s.client.Config.OrganizationID, groupID, gen.CreateGroupPolicyInput{
		Effect:     gen.PolicyEffect(input.Effect),
		Actions:    input.Actions,
		Conditions: input.Conditions,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create policy on group %s: %w", groupID, err))
	}
	if err := gql.CheckMutation("create policy", resp.CreateGroupPolicy.Successful, resp.CreateGroupPolicy.Messages); err != nil {
		return nil, err
	}
	return toPolicy(resp.CreateGroupPolicy.Result)
}

// Update updates an existing policy in place. Empty input fields leave
// the existing values unchanged. The policy's group cannot be changed
// — to retarget a policy, delete it and create a new one.
func (s *Service) Update(ctx context.Context, policyID string, input UpdatePolicyInput) (*Policy, error) {
	in := gen.UpdatePolicyInput{
		Actions:    input.Actions,
		Conditions: input.Conditions,
	}
	if input.Effect != "" {
		in.Effect = gen.PolicyEffect(input.Effect)
	}
	resp, err := gen.UpdatePolicy(ctx, s.client.GQLv2, s.client.Config.OrganizationID, policyID, in)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("update policy %s: %w", policyID, err))
	}
	if err := gql.CheckMutation("update policy", resp.UpdatePolicy.Successful, resp.UpdatePolicy.Messages); err != nil {
		return nil, err
	}
	return toPolicy(resp.UpdatePolicy.Result)
}

// Delete deletes a policy by ID.
func (s *Service) Delete(ctx context.Context, policyID string) error {
	resp, err := gen.DeletePolicy(ctx, s.client.GQLv2, s.client.Config.OrganizationID, policyID)
	if err != nil {
		return gql.ClassifyError(fmt.Errorf("delete policy %s: %w", policyID, err))
	}
	return gql.CheckMutation("delete policy", resp.DeletePolicy.Successful, resp.DeletePolicy.Messages)
}

// toPolicy decodes a genqlient policy result. The wire conditions
// translation happens through [PolicyConditions]'s json (un)marshaler,
// so mapstructure copies the field through natively.
func toPolicy(v any) (*Policy, error) {
	p := Policy{}
	if err := decode.Decode(v, &p); err != nil {
		return nil, fmt.Errorf("decode policy: %w", err)
	}
	return &p, nil
}
