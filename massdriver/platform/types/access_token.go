package types

import "time"

// AccessToken is an access token issued to the authenticated identity — a
// personal access token when the caller is an account, a service-account
// token when the caller is a service account. The API models both as one
// type; which kind you hold is determined by the identity that created it.
//
// The Token field is the raw bearer credential and is populated only by
// the create methods — the API returns it exactly once at creation and
// never again. Tokens from list and revoke operations carry metadata only.
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

	// Token is the raw bearer credential. Populated only at creation —
	// store it immediately; if lost, revoke the token and create a new
	// one. Empty on tokens returned by list and revoke operations.
	Token string `json:"token,omitempty" mapstructure:"token"`
}
