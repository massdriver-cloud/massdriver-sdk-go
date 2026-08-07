package types

import "time"

// ConnectionOrientation determines how instances receive a dependency of a
// resource type: wired explicitly on the canvas, or satisfied automatically
// by an environment-level default.
type ConnectionOrientation string

const (
	// ConnectionOrientationLink means the dependency is wired explicitly by
	// drawing a connection between two instances on the canvas.
	ConnectionOrientationLink ConnectionOrientation = "LINK"
	// ConnectionOrientationEnvironmentDefault means the dependency is
	// satisfied automatically by an environment-level default, shared across
	// all instances in the environment without explicit wiring.
	ConnectionOrientationEnvironmentDefault ConnectionOrientation = "ENVIRONMENT_DEFAULT"
)

// ImportInstruction is one set of import instructions for a resource type,
// typically rendered as a tab (e.g. one for the CLI and one for the cloud
// console). Label is the tab heading; Content is the markdown body.
type ImportInstruction struct {
	Label   string `json:"label" mapstructure:"label"`
	Content string `json:"content" mapstructure:"content"`
}

// ResourceType is the contract a [Resource] conforms to — the schema layer
// of Massdriver's connection system. Every dependency a bundle declares and
// every resource a bundle produces references a resource type.
//
// The full shape (schema, UI schema, instructions, orientation, attributes)
// is populated by resourcetypes.Get; slim refs embedded in other types
// (e.g. Resource.ResourceType) carry only ID/Name/Icon.
type ResourceType struct {
	ID   string `json:"id" mapstructure:"id"`
	Name string `json:"name" mapstructure:"name"`
	Icon string `json:"icon,omitempty" mapstructure:"icon,omitempty"`

	// ConnectionOrientation is how instances receive a dependency of this
	// resource type.
	ConnectionOrientation ConnectionOrientation `json:"connectionOrientation,omitempty" mapstructure:"connectionOrientation"`

	// Schema is the full JSON Schema describing the shape of data this
	// resource type exposes to dependents, returned verbatim including
	// Massdriver's `$md` extensions.
	Schema map[string]any `json:"schema,omitempty" mapstructure:"schema"`

	// UISchema holds rendering hints for the import form, following
	// react-jsonschema-form's uiSchema conventions. Empty when the resource
	// type provides none.
	UISchema map[string]any `json:"uiSchema,omitempty" mapstructure:"uiSchema"`

	// Instructions are step-by-step import instructions, typically one entry
	// per workflow (CLI, console, etc.). Empty when the resource type
	// provides none.
	Instructions []ImportInstruction `json:"instructions,omitempty" mapstructure:"instructions"`

	// EffectiveAttributes are the auto-injected `md-*` system attributes for
	// this resource type (today: `md-id`).
	EffectiveAttributes map[string]any `json:"effectiveAttributes,omitempty" mapstructure:"effectiveAttributes"`

	CreatedAt time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}
