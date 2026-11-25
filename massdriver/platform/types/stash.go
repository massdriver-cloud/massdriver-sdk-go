package types

import "time"

// StashEntry is a key and value stored in one stash scope: a project,
// environment, component, instance, or deployment.
//
// Value is nil when the entry is sensitive and the caller cannot write its
// scope. When Format is JSON, Value is a JSON object and Data holds it
// parsed. ExpiresAt is zero for an entry that does not expire.
type StashEntry struct {
	Key       string         `json:"key" mapstructure:"key"`
	Value     *string        `json:"value" mapstructure:"value"`
	Data      map[string]any `json:"data,omitempty" mapstructure:"data"`
	Format    string         `json:"format" mapstructure:"format"`
	Immutable bool           `json:"immutable" mapstructure:"immutable"`
	Sensitive bool           `json:"sensitive" mapstructure:"sensitive"`
	ExpiresAt time.Time      `json:"expiresAt,omitzero" mapstructure:"expiresAt"`
	UpdatedAt time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
