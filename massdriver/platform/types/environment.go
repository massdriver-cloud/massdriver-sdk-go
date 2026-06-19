package types

import "time"

// Environment is an isolated deployment context (production, staging, dev, ...)
// within a [Project].
//
// Embedded fields (Project, Instances, Connections, Defaults) are populated
// only when the underlying GraphQL query selected them.
type Environment struct {
	ID          string         `json:"id" mapstructure:"id"`
	Name        string         `json:"name" mapstructure:"name"`
	Description string         `json:"description,omitempty" mapstructure:"description"`
	Attributes  map[string]any `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	CreatedAt   time.Time      `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	Cost        CostSummary    `json:"cost,omitzero" mapstructure:"cost"`

	Project     *Project             `json:"project,omitempty" mapstructure:"project,omitempty"`
	Instances   []Instance           `json:"instances,omitempty" mapstructure:"instances,omitempty"`
	Connections []Connection         `json:"connections,omitempty" mapstructure:"connections,omitempty"`
	Defaults    []EnvironmentDefault `json:"defaults,omitempty" mapstructure:"-"`
}

// EnvironmentDefault is a resource pre-assigned to an [Environment] so
// instances inherit it automatically when their connection schema
// matches the resource type. Only one default per resource type is
// allowed per environment.
//
// Resource is a slim view of the underlying resource (id / name /
// resourceType) — call platform/resources.Get with the resource ID for
// the full payload. Environment is populated only when the underlying
// GraphQL query selected it.
type EnvironmentDefault struct {
	ID          string                     `json:"id" mapstructure:"id"`
	Resource    EnvironmentDefaultResource `json:"resource" mapstructure:"resource"`
	Environment *Environment               `json:"environment,omitempty" mapstructure:"environment,omitempty"`
	CreatedAt   time.Time                  `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time                  `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}

// EnvironmentDefaultResource is the slim resource view embedded on an
// [EnvironmentDefault]. The full resource is reachable via
// platform/resources.Get using [EnvironmentDefaultResource.ID].
type EnvironmentDefaultResource struct {
	ID           string        `json:"id" mapstructure:"id"`
	Name         string        `json:"name" mapstructure:"name"`
	ResourceType *ResourceType `json:"resourceType,omitempty" mapstructure:"resourceType,omitempty"`
}
