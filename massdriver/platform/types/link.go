package types

import "time"

// Link is a design-time wire between two components in a project's blueprint:
// the source component's output field is connected to the destination
// component's input field. At deploy time, each link is realized as a
// [Connection] in the environment.
//
// FromComponent and ToComponent are populated when the underlying GraphQL
// query selected them — typically with a slim shape (id/name) when embedded
// on a [Project]; Components fetched separately via platform/components carry
// their full shape.
type Link struct {
	ID            string     `json:"id" mapstructure:"id"`
	FromField     string     `json:"fromField" mapstructure:"fromField"`
	ToField       string     `json:"toField" mapstructure:"toField"`
	CreatedAt     time.Time  `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	FromComponent *Component `json:"fromComponent,omitempty" mapstructure:"fromComponent,omitempty"`
	ToComponent   *Component `json:"toComponent,omitempty" mapstructure:"toComponent,omitempty"`
}
