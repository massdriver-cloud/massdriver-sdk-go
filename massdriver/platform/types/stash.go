package types

import "time"

// StashScopeKind is the kind of resource that holds stash entries.
type StashScopeKind string

const (
	StashScopeProject     StashScopeKind = "project"
	StashScopeEnvironment StashScopeKind = "environment"
	StashScopeComponent   StashScopeKind = "component"
	StashScopeInstance    StashScopeKind = "instance"
	StashScopeDeployment  StashScopeKind = "deployment"
)

// StashScope is the one resource that holds a stash entry. ID is the
// resource's identifier, such as "api-prod" for an environment, or the
// deployment's UUID.
type StashScope struct {
	Kind StashScopeKind `json:"kind" mapstructure:"kind"`
	ID   string         `json:"id" mapstructure:"id"`
}

// String renders the scope as `<kind>:<id>`.
func (s StashScope) String() string { return string(s.Kind) + ":" + s.ID }

// StashEntry is a key and value stored in one stash scope: a project,
// environment, component, instance, or deployment.
//
// Value is nil when the entry is sensitive and the caller cannot write its
// scope. When Format is JSON, Value is a JSON object and Data holds it
// parsed. Label is a human-readable name, empty when none is set.
// ExpiresAt is zero for an entry that does not expire.
type StashEntry struct {
	Scope     StashScope     `json:"scope" mapstructure:"scope"`
	Key       string         `json:"key" mapstructure:"key"`
	Label     string         `json:"label" mapstructure:"label"`
	Value     *string        `json:"value" mapstructure:"value"`
	Data      map[string]any `json:"data,omitempty" mapstructure:"data"`
	Format    string         `json:"format" mapstructure:"format"`
	Immutable bool           `json:"immutable" mapstructure:"immutable"`
	Sensitive bool           `json:"sensitive" mapstructure:"sensitive"`
	ExpiresAt time.Time      `json:"expiresAt,omitzero" mapstructure:"expiresAt"`
	UpdatedAt time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}

// StashEventAction is what happened to a stash entry.
type StashEventAction string

const (
	StashEventSet     StashEventAction = "SET"
	StashEventDeleted StashEventAction = "DELETED"
)

// StashEvent says a stash entry was set or deleted. Entry is nil for
// [StashEventDeleted], and its Value is nil when the entry is sensitive.
type StashEvent struct {
	Action    StashEventAction `json:"action"`
	Scope     StashScope       `json:"scope"`
	Key       string           `json:"key"`
	Entry     *StashEntry      `json:"entry,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
}

// WidgetDefinition is one version of a widget that the Massdriver UI draws
// from a well-known stash key.
//
// Entity is the kind of thing the widget shows on: PROJECT, ENVIRONMENT,
// COMPONENT, INSTANCE, DEPLOYMENT, or LINK. Scope is the kind of resource
// whose stash holds Key. Schema is the JSON Schema the key's value must
// match.
//
// Key is a fixed key such as `md/v1/usage`, or a pattern with named
// segments in braces such as `md/v1/step/{step}`. KeySegments names the
// segments of a pattern, and is empty for a fixed key.
type WidgetDefinition struct {
	Name        string         `json:"name" mapstructure:"name"`
	Description string         `json:"description" mapstructure:"description"`
	Entity      string         `json:"entity" mapstructure:"entity"`
	Version     int            `json:"version" mapstructure:"version"`
	Key         string         `json:"key" mapstructure:"key"`
	KeySegments []string       `json:"keySegments" mapstructure:"keySegments"`
	Scope       StashScopeKind `json:"scope" mapstructure:"scope"`
	Format      string         `json:"format" mapstructure:"format"`
	Schema      map[string]any `json:"schema" mapstructure:"schema"`
	Inputs      []WidgetInput  `json:"inputs" mapstructure:"inputs"`
}

// WidgetInput is one input of a [WidgetDefinition]. Type is STRING,
// NUMBER, INTEGER, BOOLEAN, ARRAY, or OBJECT. Field is the field of the
// key's JSON value the input reads, empty when the key is TEXT and the
// input reads the whole value.
type WidgetInput struct {
	Name        string `json:"name" mapstructure:"name"`
	Type        string `json:"type" mapstructure:"type"`
	Required    bool   `json:"required" mapstructure:"required"`
	Description string `json:"description" mapstructure:"description"`
	Field       string `json:"field,omitempty" mapstructure:"field"`
}

// Widget is a widget drawn from a well-known key in one stash. Values maps
// each input name to its value. ExpiresAt is zero when the key does not
// expire.
//
// Scope is the resource whose stash has the key. For a deployment result
// read through an instance, it is the deployment the result came from.
// Segments holds the values of the key's named segments, such as
// {"step": "src"} for `md/v1/step/src`, and is empty for a fixed key.
type Widget struct {
	Name      string            `json:"name" mapstructure:"name"`
	Scope     StashScope        `json:"scope" mapstructure:"scope"`
	Segments  map[string]string `json:"segments" mapstructure:"segments"`
	Values    map[string]any    `json:"values" mapstructure:"values"`
	UpdatedAt time.Time         `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	ExpiresAt time.Time         `json:"expiresAt,omitzero" mapstructure:"expiresAt"`
}
