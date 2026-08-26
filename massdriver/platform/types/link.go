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
	ID        string `json:"id" mapstructure:"id"`
	FromField string `json:"fromField" mapstructure:"fromField"`
	ToField   string `json:"toField" mapstructure:"toField"`

	// FromVersionConstraint / ToVersionConstraint are the version ranges of
	// the source and destination components this link routes between, as
	// tilde constraints (`~1` covers 1.x, `~0.4` covers 0.4.x). Empty when
	// the range has not been determined.
	FromVersionConstraint string `json:"fromVersionConstraint,omitempty" mapstructure:"fromVersionConstraint"`
	ToVersionConstraint   string `json:"toVersionConstraint,omitempty" mapstructure:"toVersionConstraint"`

	CreatedAt     time.Time  `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	FromComponent *Component `json:"fromComponent,omitempty" mapstructure:"fromComponent,omitempty"`
	ToComponent   *Component `json:"toComponent,omitempty" mapstructure:"toComponent,omitempty"`
}
