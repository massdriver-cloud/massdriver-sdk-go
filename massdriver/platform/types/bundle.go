package types

import "time"

// Bundle is a published OCI artifact in an [OciRepo] — a versioned bundle
// release that an [Instance] resolves to.
//
// The [Bundle.ID] is the composite `name@version` form (e.g.
// `aws-aurora-postgres@1.2.3`); fetching by ID also accepts release
// channels (`name@~1`, `name@latest`) per the schema's BundleId scalar.
//
// Dependencies and Resources are populated by bundles.Get (full bundle
// shape); they are nil on slim refs embedded in other types (e.g.
// Instance.Bundle).
type Bundle struct {
	ID          string    `json:"id" mapstructure:"id"`
	Name        string    `json:"name" mapstructure:"name"`
	Version     string    `json:"version" mapstructure:"version"`
	Description string    `json:"description,omitempty" mapstructure:"description"`
	Icon        string    `json:"icon,omitempty" mapstructure:"icon"`
	SourceURL   string    `json:"sourceUrl,omitempty" mapstructure:"sourceUrl"`
	Repo        string    `json:"repo,omitempty" mapstructure:"repo"`
	CreatedAt   time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`

	// Dependencies are inputs this bundle requires from other bundles'
	// outputs at deploy time.
	Dependencies []BundleDependency `json:"dependencies,omitempty" mapstructure:"dependencies,omitempty"`

	// Resources are outputs this bundle produces when deployed. Other
	// bundles can consume these as Dependencies.
	Resources []BundleResource `json:"resources,omitempty" mapstructure:"resources,omitempty"`
}

// BundleDependency declares one named slot a bundle requires at deploy
// time. ResourceType is the contract the connected resource must satisfy.
type BundleDependency struct {
	Name         string        `json:"name" mapstructure:"name"`
	Required     bool          `json:"required" mapstructure:"required"`
	ResourceType *ResourceType `json:"resourceType,omitempty" mapstructure:"resourceType,omitempty"`
}

// BundleResource declares one named output a bundle produces on a
// successful deployment. ResourceType is the contract this output
// satisfies.
type BundleResource struct {
	Name         string        `json:"name" mapstructure:"name"`
	Required     bool          `json:"required" mapstructure:"required"`
	ResourceType *ResourceType `json:"resourceType,omitempty" mapstructure:"resourceType,omitempty"`
}
