package types

import "time"

// CustomAttribute is a user-declared attribute key, the resource scope
// where it applies, and whether it is required at create time.
//
// Custom attributes enforce consistent metadata across an organization.
// System attributes (`md-*`) are auto-injected by Massdriver and are not
// declared via this type — only user-defined keys.
type CustomAttribute struct {
	ID        string    `json:"id" mapstructure:"id"`
	Key       string    `json:"key" mapstructure:"key"`
	Scope     string    `json:"scope" mapstructure:"scope"`
	Required  bool      `json:"required" mapstructure:"required"`
	Values    []string  `json:"values" mapstructure:"values"`
	CreatedAt time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
