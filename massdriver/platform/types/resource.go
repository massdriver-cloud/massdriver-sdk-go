package types

import "time"

// Resource is a Massdriver resource — either a provisioned output of a
// deployed [Instance] (a connection string, endpoint, credential, etc.) or
// an imported resource manually registered in the catalog.
//
// Origin is "PROVISIONED" or "IMPORTED" — see [platform/resources].Origin
// for the typed constants. For provisioned resources, [Resource.Field] is
// the bundle output handle (e.g. "authentication") and [Resource.Instance]
// references the producing instance. Both are nil/empty for imported
// resources.
//
// Payload contains the structured data; fields marked sensitive in the
// resource type's schema are masked as the literal string "[SENSITIVE]"
// — call platform/resources.Export to retrieve unmasked values.
//
// Embedded refs (ResourceType, Instance) are populated only when the
// underlying GraphQL query selected them; List/Get pull both, but slim
// embed sites elsewhere (e.g. InstanceResource.Resource) populate only
// id/name/origin/createdAt/updatedAt.
type Resource struct {
	ID           string         `json:"id" mapstructure:"id"`
	Name         string         `json:"name" mapstructure:"name"`
	Origin       string         `json:"origin,omitempty" mapstructure:"origin"`
	Field        string         `json:"field,omitempty" mapstructure:"field"`
	Formats      []string       `json:"formats,omitempty" mapstructure:"formats,omitempty"`
	Payload      map[string]any `json:"payload,omitempty" mapstructure:"payload,omitempty"`
	Attributes   map[string]any `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	ResourceType *ResourceType  `json:"resourceType,omitempty" mapstructure:"resourceType,omitempty"`
	Instance     *Instance      `json:"instance,omitempty" mapstructure:"instance,omitempty"`
	CreatedAt    time.Time      `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}

// ResourceType is the artifact-definition contract a [Resource] conforms to
// — the schema describing what fields the resource carries.
//
// Skeleton — fuller shape (icon, schema, connection orientation, etc.) will
// land alongside the platform/resourcetypes package, when that package is
// designed.
type ResourceType struct {
	ID   string `json:"id" mapstructure:"id"`
	Name string `json:"name" mapstructure:"name"`
	Icon string `json:"icon,omitempty" mapstructure:"icon,omitempty"`
}

