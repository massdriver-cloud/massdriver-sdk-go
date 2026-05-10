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
// On the wire the type is polymorphic: the JSON string `"*"` for the
// whole-policy wildcard, otherwise an object whose values are either
// `"*"` (per-key wildcard) or arrays of strings. [PolicyConditions]
// implements [json.Marshaler] and [json.Unmarshaler] so this
// translation is invisible — both genqlient and mapstructure see a
// regular Go map.
type PolicyConditions map[string][]string

// wildcardWire is the on-wire JSON encoding of the wildcard sentinel:
// the JSON-encoded string `"*"`. Used both for the whole-policy
// wildcard and for per-key wildcards.
var wildcardWire = []byte(`"*"`)

// MarshalJSON encodes c into the polymorphic wire form. A nil map
// becomes the wildcard sentinel `"*"`; a populated map becomes an
// object literal whose values are either `"*"` (for nil/empty
// per-key slices) or arrays of strings.
func (c PolicyConditions) MarshalJSON() ([]byte, error) {
	if c == nil {
		return append([]byte(nil), wildcardWire...), nil
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

// UnmarshalJSON decodes the polymorphic wire form. The wildcard
// sentinel `"*"` becomes a nil map; per-key `"*"` values become nil
// slices; arrays decode into the slice positions.
func (c *PolicyConditions) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), wildcardWire) {
		*c = nil
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("PolicyConditions: %w", err)
	}
	out := make(PolicyConditions, len(raw))
	for k, v := range raw {
		if bytes.Equal(bytes.TrimSpace(v), wildcardWire) {
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
