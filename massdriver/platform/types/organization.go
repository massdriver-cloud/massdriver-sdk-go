package types

import "time"

// Organization is a Massdriver organization — the top-level container for
// every project, environment, and bundle catalog.
//
// SubscriptionStatus, TrialEndsAt, and PlanExpiresOn are populated when
// the underlying GraphQL query selects them (organizations.Get does;
// embed selections elsewhere may not).
type Organization struct {
	ID                 string    `json:"id" mapstructure:"id"`
	Name               string    `json:"name" mapstructure:"name"`
	SubscriptionStatus string    `json:"subscriptionStatus,omitempty" mapstructure:"subscriptionStatus"`
	TrialEndsAt        time.Time `json:"trialEndsAt,omitzero" mapstructure:"trialEndsAt"`
	CreatedAt          time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
