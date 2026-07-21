package types

// VersionComparison is a single version string compared across the two sides
// of a comparison. Source/Target are empty when the corresponding side has no
// version to report (e.g. an instance that exists on only one side of an
// environment comparison).
type VersionComparison struct {
	Source string `json:"source,omitempty" mapstructure:"source"`
	Target string `json:"target,omitempty" mapstructure:"target"`
	Equal  bool   `json:"equal" mapstructure:"equal"`
}

// ParamValue is one side of a single leaf-level param comparison. Value is a
// display string — non-string leaves (numbers, booleans, arrays) are rendered
// as text ("5432", "true", "[1,2,3]"). An empty Value is ambiguous between "the
// key is missing" and "the value is empty/null" — disambiguate with Present.
type ParamValue struct {
	Present bool   `json:"present" mapstructure:"present"`
	Value   string `json:"value,omitempty" mapstructure:"value"`
}

// ParamComparison is a single leaf-level diff entry between two params maps.
// The list returned by a comparison is flat: every entry is a terminal leaf
// (maps and arrays are walked to the bottom), with Path holding the jq-style
// location (".database.port", ".containers[0].image"). Filter on !Equal to
// keep only the entries that differ.
type ParamComparison struct {
	Path   string     `json:"path" mapstructure:"path"`
	Source ParamValue `json:"source" mapstructure:"source"`
	Target ParamValue `json:"target" mapstructure:"target"`
	Equal  bool       `json:"equal" mapstructure:"equal"`
}

// DeploymentComparison is a side-by-side comparison of two deployments'
// snapshotted configuration — bundle version and params. Runtime state, logs,
// and produced artifacts are out of scope. Returned by deployments.Compare.
type DeploymentComparison struct {
	Source  Deployment        `json:"source" mapstructure:"source"`
	Target  Deployment        `json:"target" mapstructure:"target"`
	Version VersionComparison `json:"version" mapstructure:"version"`
	Params  []ParamComparison `json:"params" mapstructure:"params"`
}

// InstanceComparison is one component's entry in an [EnvironmentComparison].
// Instances are paired across environments by their underlying component;
// when only one side has an instance for the component, the other side's
// Source/Target is nil and every param appears as present on the populated
// side only.
type InstanceComparison struct {
	Component Component         `json:"component" mapstructure:"component"`
	Source    *Instance         `json:"source,omitempty" mapstructure:"source,omitempty"`
	Target    *Instance         `json:"target,omitempty" mapstructure:"target,omitempty"`
	Version   VersionComparison `json:"version" mapstructure:"version"`
	Params    []ParamComparison `json:"params" mapstructure:"params"`
	Equal     bool              `json:"equal" mapstructure:"equal"`
}

// EnvironmentComparison is a side-by-side comparison of two environments in
// the same project, instance-by-instance. Environment-level attributes and
// default resource wiring are out of scope. Returned by environments.Compare.
type EnvironmentComparison struct {
	Source    Environment          `json:"source" mapstructure:"source"`
	Target    Environment          `json:"target" mapstructure:"target"`
	Instances []InstanceComparison `json:"instances" mapstructure:"instances"`
}
