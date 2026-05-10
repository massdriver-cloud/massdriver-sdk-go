package types

import "time"

// Grant is a sharing rule the publisher of an OCI repo or a resource
// has authored. It says "this thing is shared as <action>, available to
// recipients matching <conditions>."
//
// RecipientConditions follows the same shape conventions as
// [PolicyConditions]:
//   - nil map: wildcard — every recipient in the org qualifies
//   - non-nil map: per-attribute conditions on the recipient project
//     (for OCI-repo grants) or environment (for resource grants)
//
// Action values depend on the source kind:
//   - OCI repo grants: "repo:pull" today (visibility implied)
//   - Resource grants: "resource:export" today (visibility implied)
//
// Source identifies the OCI repo or resource being shared. The wrapper
// surface in platform/resources / platform/ocirepos doesn't currently
// select the source on the Grant return shape (the caller already knows
// what they granted on); this field is reserved for cases where it
// matters and stays nil otherwise.
type Grant struct {
	ID                  string           `json:"id" mapstructure:"id"`
	Action              string           `json:"action" mapstructure:"action"`
	RecipientConditions PolicyConditions `json:"recipientConditions,omitempty" mapstructure:"recipientConditions,omitempty"`
	CreatedAt           time.Time        `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
