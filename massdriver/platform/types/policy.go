package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Policy is one ABAC rule attached to a [Group]. Policies grant or deny
// actions on entities whose attributes match the conditions.
type Policy struct {
	ID      string   `json:"id" mapstructure:"id"`
	Effect  string   `json:"effect" mapstructure:"effect"`
	Actions []string `json:"actions" mapstructure:"actions"`

	// Conditions is the policy's condition set. Nil represents the
	// wildcard ("matches everything"); a non-nil map is the attribute
	// restrictions. See [PolicyConditions] for the encoding.
	Conditions PolicyConditions `json:"conditions,omitempty" mapstructure:"conditions,omitempty"`
	CreatedAt  time.Time        `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt  time.Time        `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	// Group is the principal this policy applies to. Populated when fetched
	// directly; nil when embedded under a Group's policies list (where the
	// owning group is already known).
	Group *Group `json:"group,omitempty" mapstructure:"group,omitempty"`
}

// PolicyAction is one entry in the ABAC action catalog. Actions follow
// the `{entity}:{verb}` format (e.g. "project:view", "instance:deploy")
// and are the building blocks of [Policy]s.
//
// The catalog is exposed at runtime via platform/policies.ListActions
// rather than as a static enum, so the server can grow the action set
// without breaking clients.
type PolicyAction struct {
	// ID is the canonical action identifier, e.g. "project:view".
	ID string `json:"id" mapstructure:"id"`
	// Verb is the action portion of the id, e.g. "view".
	Verb string `json:"verb" mapstructure:"verb"`
	// Entity identifies the entity this action applies to (id matches
	// the prefix of [PolicyAction.ID]).
	Entity *PolicyEntity `json:"entity,omitempty" mapstructure:"entity,omitempty"`
	// Description is a human-readable explanation of what the action
	// permits — written for end-user docs and policy-authoring UIs.
	Description string `json:"description" mapstructure:"description"`
}

// PolicyEntity is one of the entity kinds an action can apply to (e.g.
// "project", "environment"). Surfaced via platform/policies.ListEntities
// when grouping actions by what they apply to in a UI.
type PolicyEntity struct {
	ID          string `json:"id" mapstructure:"id"`
	Description string `json:"description" mapstructure:"description"`
}

// PolicyDecision is the result of [platform/policies.Evaluate] or one
// element of a [platform/policies.EvaluateBatch] result. Action and
// EntityID echo the request inputs so batch callers can correlate
// decisions without tracking positions.
type PolicyDecision struct {
	Allowed  bool   `json:"allowed" mapstructure:"allowed"`
	Action   string `json:"action" mapstructure:"action"`
	EntityID string `json:"entityId" mapstructure:"entityId"`
}

// PolicyConditions describes the entity-attribute restrictions on a
// [Policy].
//
// The map itself encodes whole-policy semantics:
//   - nil map (the zero value) — wildcard policy. Matches every entity
//     of each action's type, no restrictions.
//   - non-nil map — restricts the policy to entities whose attributes
//     satisfy every key in the map.
//
// Each per-key value encodes per-attribute semantics:
//   - nil or empty []string — the entity must HAVE the attribute set,
//     but any value is accepted. Use [policies.Wildcard] to make this
//     intent explicit at the call site.
//   - non-empty []string — closed set; the entity's attribute value
//     must match one of these.
//
// Example combining both:
//
//	PolicyConditions{
//	    "md-project":     policies.Wildcard,       // any md-project, but it must be set
//	    "md-environment": {"dev", "staging"},      // closed set
//	}
//
// matches entities that have any md-project AND whose md-environment
// is dev or staging.
//
// [PolicyConditions.MarshalJSON] produces the plain user-facing form
// (`null` for the wildcard, `{"key": "*"}` or `{"key": [...]}` per
// entry) so json.Marshal round-trips through jsonencode/decode without
// surprises. The wire form (a JSON-encoded string the GraphQL scalar
// requires) is handled by the genqlient binding in gql/scalars; SDK
// callers never need to see it.
type PolicyConditions map[string][]string

// MarshalJSON encodes c into the plain user-facing form:
//   - nil map → `null`
//   - populated map → JSON object whose per-key values are `"*"` for
//     nil/empty slices or `["a","b",...]` for closed sets
//
// This is NOT the GraphQL wire form — that lives in gql/scalars.
func (c PolicyConditions) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte("null"), nil
	}
	raw := make(map[string]any, len(c))
	for k, v := range c {
		if len(v) == 0 {
			raw[k] = "*"
		} else {
			raw[k] = v
		}
	}
	return json.Marshal(raw)
}

// UnmarshalJSON decodes the plain user-facing form (the inverse of
// MarshalJSON). The GraphQL wire form is peeled by the genqlient
// binding in gql/scalars before this method runs, so by the time we
// see the bytes they are either `null` or a JSON object.
func (c *PolicyConditions) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*c = nil
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return fmt.Errorf("PolicyConditions: %w", err)
	}
	out := make(PolicyConditions, len(raw))
	for k, v := range raw {
		if bytes.Equal(bytes.TrimSpace(v), []byte(`"*"`)) {
			out[k] = nil
			continue
		}
		var vals []string
		if err := json.Unmarshal(v, &vals); err != nil {
			return fmt.Errorf("PolicyConditions[%s]: %w", k, err)
		}
		out[k] = vals
	}
	*c = out
	return nil
}
