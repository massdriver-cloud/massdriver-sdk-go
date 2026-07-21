package types

import "time"

// RemoteReference is a per-instance override of a single connection slot,
// pointing the slot at a resource from another project (or an imported
// resource) instead of the blueprint Link's wiring. Field names the key in
// the instance's bundle connectionsSchema that the reference satisfies.
//
// Resource carries the slim selection the underlying mutation returns
// (id, name, resourceType); call platform/resources.Get with the resource ID
// for the full payload. Instance is populated only when the underlying
// GraphQL query selected it.
type RemoteReference struct {
	ID        string    `json:"id" mapstructure:"id"`
	Field     string    `json:"field" mapstructure:"field"`
	Resource  Resource  `json:"resource" mapstructure:"resource"`
	Instance  *Instance `json:"instance,omitempty" mapstructure:"instance,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
