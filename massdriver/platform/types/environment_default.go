package types

import "time"

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
