package types

import "time"

// Environment is an isolated deployment context (production, staging, dev, ...)
// within a [Project].
//
// Embedded fields (Project, Instances, Connections) are populated only when
// the underlying GraphQL query selected them.
type Environment struct {
	ID          string         `json:"id" mapstructure:"id"`
	Name        string         `json:"name" mapstructure:"name"`
	Description string         `json:"description,omitempty" mapstructure:"description"`
	Attributes  map[string]any `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	CreatedAt   time.Time      `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	Cost        CostSummary    `json:"cost,omitzero" mapstructure:"cost"`

	Project     *Project     `json:"project,omitempty" mapstructure:"project,omitempty"`
	Instances   []Instance   `json:"instances,omitempty" mapstructure:"instances,omitempty"`
	Connections []Connection `json:"connections,omitempty" mapstructure:"connections,omitempty"`
}
