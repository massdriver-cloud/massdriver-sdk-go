// Package types holds the shared domain structs for the Massdriver platform.
//
// Operations live in their per-domain packages (massdriver/platform/projects,
// massdriver/platform/environments, etc.); types live here so that domain
// structs can reference each other without import cycles. Each domain package
// re-exports its namesake type as a Go type alias for ergonomic access at the
// call site (`projects.Project` resolves to `types.Project`).
package types

import "time"

// Project is a Massdriver project — the top-level container for related
// infrastructure. Owns a blueprint (architecture) and one or more environments
// (deployments).
//
// Embedded slices (Environments, Components, Links) are populated only when
// the underlying GraphQL query selected them. Operations that fetch these by
// default are noted on the relevant function (e.g. projects.Get).
type Project struct {
	ID          string         `json:"id" mapstructure:"id"`
	Name        string         `json:"name" mapstructure:"name"`
	Description string         `json:"description,omitempty" mapstructure:"description"`
	Attributes  map[string]any `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	CreatedAt   time.Time      `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	Cost        CostSummary    `json:"cost,omitzero" mapstructure:"cost"`

	// Environments are the deployment contexts (production, staging, ...) in
	// the project. Populated by projects.Get/projects.List when the wrapper
	// performs the page unwrap; nil otherwise.
	//
	// The mapstructure:"-" tag opts out of the primary decode pass — the
	// project's `environments` field comes back as an EnvironmentsPage
	// (`{items: [...], cursor: ...}`) on the wire, so the wrapper does a
	// second decode pass to flatten items into this slice.
	Environments []Environment `json:"environments,omitempty" mapstructure:"-"`

	// Components are the bundle slots that make up this project's blueprint.
	// Populated only when the query selects them. Currently no SDK operation
	// selects this field by default — reserved for when platform/components
	// lands.
	Components []Component `json:"components,omitempty" mapstructure:"components,omitempty"`

	// Links are the design-time wires between components in this project.
	// Reserved like Components above.
	Links []Link `json:"links,omitempty" mapstructure:"links,omitempty"`
}
