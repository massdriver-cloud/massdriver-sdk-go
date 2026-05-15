package types

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
