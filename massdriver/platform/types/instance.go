package types

import "time"

// Instance is a deployed bundle within an [Environment] — the runtime
// realization of a [Component]. An instance carries the configuration
// that will be (or has been) applied on the next deployment, the
// resolved bundle version, status, and the environment/component/bundle
// relationships.
//
// Reference fields (Environment, Bundle, Component) are populated when
// the underlying GraphQL query selected them. StatePaths and Resources
// are populated by instances.Get; instances.List leaves them empty to
// keep paginated responses small. Alarms and secrets are managed
// separately via instances.IterAlarms / instances.SetSecret etc.
type Instance struct {
	ID               string         `json:"id" mapstructure:"id"`
	Name             string         `json:"name" mapstructure:"name"`
	Status           string         `json:"status" mapstructure:"status"`
	Version          string         `json:"version" mapstructure:"version"`
	ResolvedVersion  string         `json:"resolvedVersion,omitempty" mapstructure:"resolvedVersion"`
	DeployedVersion  string         `json:"deployedVersion,omitempty" mapstructure:"deployedVersion"`
	AvailableUpgrade string         `json:"availableUpgrade,omitempty" mapstructure:"availableUpgrade"`
	Params           map[string]any `json:"params,omitempty" mapstructure:"params"`
	Attributes       map[string]any `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	CreatedAt        time.Time      `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	Cost             CostSummary    `json:"cost,omitzero" mapstructure:"cost"`

	Environment *Environment        `json:"environment,omitempty" mapstructure:"environment,omitempty"`
	Bundle      *Bundle             `json:"bundle,omitempty" mapstructure:"bundle,omitempty"`
	Component   *Component          `json:"component,omitempty" mapstructure:"component,omitempty"`
	StatePaths  []InstanceStatePath `json:"statePaths,omitempty" mapstructure:"statePaths,omitempty"`
	Resources   []Resource          `json:"resources,omitempty" mapstructure:"-"`
}

// InstanceStatePath is a Terraform/OpenTofu state path for a single
// deployment step.
type InstanceStatePath struct {
	StepName string `json:"stepName" mapstructure:"stepName"`
	StateURL string `json:"stateUrl" mapstructure:"stateUrl"`
}
