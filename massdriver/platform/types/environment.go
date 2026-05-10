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

	// Project is the parent project this environment belongs to. Populated by
	// environments.Get/environments.List with id/name/description/attributes
	// and timestamps; nested fields on the project (its environments,
	// components, links) are not populated — call projects.Get to fetch them.
	Project *Project `json:"project,omitempty" mapstructure:"project,omitempty"`

	// Instances are the deployments inside this environment. Reserved for
	// when platform/instances lands; not selected by current queries.
	Instances []Instance `json:"instances,omitempty" mapstructure:"instances,omitempty"`

	// Connections are the runtime wirings between deployed instances.
	// Reserved like Instances above.
	Connections []Connection `json:"connections,omitempty" mapstructure:"connections,omitempty"`
}
