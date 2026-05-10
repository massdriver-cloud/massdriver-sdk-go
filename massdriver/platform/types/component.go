package types

import "time"

// Component is a bundle slot in a project's blueprint — the design-time
// declaration of what infrastructure the project consists of. A component
// names *what* to deploy; the running infrastructure lives in [Instance]s,
// one per environment the component is deployed to.
//
// OciRepo and Project are populated only when the underlying GraphQL query
// selected them. Components fetched directly via components.Get include
// both; Components embedded on a [Project] include OciRepo (slim — id/name
// /reference) and leave Project nil to avoid recursing into the parent we
// already have.
type Component struct {
	ID          string             `json:"id" mapstructure:"id"`
	Name        string             `json:"name" mapstructure:"name"`
	Description string             `json:"description,omitempty" mapstructure:"description"`
	Attributes  map[string]any     `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	Position    *ComponentPosition `json:"position,omitempty" mapstructure:"position,omitempty"`
	CreatedAt   time.Time          `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	OciRepo     *OciRepo           `json:"ociRepo,omitempty" mapstructure:"ociRepo,omitempty"`
	Project     *Project           `json:"project,omitempty" mapstructure:"project,omitempty"`

	// Instances are the runtime deployments of this component, one per
	// environment. Reserved for when platform/instances lands.
	Instances []Instance `json:"instances,omitempty" mapstructure:"instances,omitempty"`
}

// ComponentPosition is a component's coordinates on the visual canvas, in
// pixels.
type ComponentPosition struct {
	X int `json:"x" mapstructure:"x"`
	Y int `json:"y" mapstructure:"y"`
}
