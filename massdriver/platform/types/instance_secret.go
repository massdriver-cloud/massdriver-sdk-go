package types

import "time"

// InstanceSecret is metadata for an encrypted secret attached to an
// [Instance]. The value is never returned by the API — only the name, the
// SHA-256 fingerprint, and timestamps are exposed.
type InstanceSecret struct {
	Name      string    `json:"name" mapstructure:"name"`
	SHA256    string    `json:"sha256,omitempty" mapstructure:"sha256"`
	CreatedAt time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
