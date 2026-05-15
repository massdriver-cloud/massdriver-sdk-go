package types

import "time"

// ServiceAccount is a Massdriver service account — a programmatic API
// client identity, distinct from a human user. Service accounts have
// access tokens (created via the accesstokens package) and can be added
// to groups for permission control.
type ServiceAccount struct {
	ID          string    `json:"id" mapstructure:"id"`
	Name        string    `json:"name" mapstructure:"name"`
	Description string    `json:"description,omitempty" mapstructure:"description"`
	CreatedAt   time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
