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

	Environments []Environment `json:"environments,omitempty" mapstructure:"-"`
	Components   []Component   `json:"components,omitempty" mapstructure:"components,omitempty"`
	Links        []Link        `json:"links,omitempty" mapstructure:"links,omitempty"`
}
