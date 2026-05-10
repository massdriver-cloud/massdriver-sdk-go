package types

import "time"

// AccessToken is metadata for a personal access token (PAT) issued to an
// account or service account. The full bearer token value is returned only
// once at creation time — see the platform/accesstokens package for the
// shape that includes the raw value.
//
// Token states (derive from the timestamps below):
//   - Active: RevokedAt is zero AND ExpiresAt is in the future.
//   - Expired: ExpiresAt is in the past.
//   - Revoked: RevokedAt is non-zero.
type AccessToken struct {
	ID         string    `json:"id" mapstructure:"id"`
	Name       string    `json:"name" mapstructure:"name"`
	Prefix     string    `json:"prefix" mapstructure:"prefix"`
	Scopes     []string  `json:"scopes" mapstructure:"scopes"`
	ExpiresAt  time.Time `json:"expiresAt,omitzero" mapstructure:"expiresAt"`
	RevokedAt  time.Time `json:"revokedAt,omitzero" mapstructure:"revokedAt"`
	LastUsedAt time.Time `json:"lastUsedAt,omitzero" mapstructure:"lastUsedAt"`
	CreatedAt  time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
}
