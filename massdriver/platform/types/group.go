package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Group is a collection of users and service accounts that share the same
// access level within an organization. Groups are the primary mechanism
// for managing access control in Massdriver.
//
// Two built-in groups exist on every organization (Admins with role
// ORGANIZATION_ADMIN, Viewers with role ORGANIZATION_VIEWER). Custom
// groups have role CUSTOM and grant project-level access via attached
// policies.
type Group struct {
	ID          string    `json:"id" mapstructure:"id"`
	Name        string    `json:"name" mapstructure:"name"`
	Description string    `json:"description,omitempty" mapstructure:"description"`
	Role        string    `json:"role" mapstructure:"role"`
	CreatedAt   time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}

// Account is a human user record — used in [Group] and [Organization]
// membership lists. Different from [Viewer], which is the
// currently-authenticated entity (account or service account).
type Account struct {
	ID        string `json:"id" mapstructure:"id"`
	Email     string `json:"email" mapstructure:"email"`
	FirstName string `json:"firstName,omitempty" mapstructure:"firstName"`
	LastName  string `json:"lastName,omitempty" mapstructure:"lastName"`
}

// GroupInvitation is a pending email invitation to join a [Group]. The
// invited user must accept before they become a member.
type GroupInvitation struct {
	ID        string    `json:"id" mapstructure:"id"`
	Email     string    `json:"email" mapstructure:"email"`
	CreatedAt time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
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
// On the wire the value is always a JSON-encoded string — either the
// literal `"*"` for the whole-policy wildcard, or a JSON-encoded
// object like `"{\"team\":[\"eng\"]}"`. [PolicyConditions] implements
// [json.Marshaler] and [json.Unmarshaler] so callers see a regular
// Go map.
type PolicyConditions map[string][]string

// MarshalJSON encodes c into the wire form: a JSON-encoded string.
// Nil map → `"*"`; populated map → JSON-encoded object literal whose
// per-key values are `"*"` (nil/empty slices) or arrays of strings.
func (c PolicyConditions) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte(`"*"`), nil
	}
	raw := make(map[string]any, len(c))
	for k, v := range c {
		if len(v) == 0 {
			raw[k] = "*"
		} else {
			raw[k] = v
		}
	}
	inner, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(inner))
}

// UnmarshalJSON decodes the wire form. The platform is asymmetric:
// inputs must be a JSON-encoded string, but responses come back as
// raw JSON objects. We accept both shapes here so callers don't see
// the inconsistency.
func (c *PolicyConditions) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("PolicyConditions: empty input")
	}

	// Peel one layer of string-encoding if present (input-side wire
	// shape). Object-shape responses skip this.
	body := trimmed
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return fmt.Errorf("PolicyConditions: %w", err)
		}
		if s == "*" {
			*c = nil
			return nil
		}
		body = []byte(s)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
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
