package types

import "time"

// EventDestination is a place outside Massdriver that receives the
// organization's audit log events.
//
// EventTypes selects the events, and Attributes keeps the events whose
// entity had all of those attribute values. Condition, a jq expression, can
// drop more of them. Config holds the adapter's settings. The destination's
// auth credentials are write-only, so they are not part of this type.
//
// SigningSecret signs each request (Standard Webhooks `whsec_` form). It is
// empty for adapters that do not sign. Treat it as a credential.
type EventDestination struct {
	ID              string            `json:"id" mapstructure:"id"`
	Name            string            `json:"name" mapstructure:"name"`
	Adapter         string            `json:"adapter" mapstructure:"adapter"`
	Config          map[string]any    `json:"config" mapstructure:"config"`
	EventTypes      []string          `json:"eventTypes" mapstructure:"eventTypes"`
	Attributes      map[string]string `json:"attributes" mapstructure:"attributes"`
	Condition       string            `json:"condition,omitempty" mapstructure:"condition"`
	Status          string            `json:"status" mapstructure:"status"`
	SigningSecret   string            `json:"signingSecret,omitempty" mapstructure:"signingSecret"`
	FailingSince    time.Time         `json:"failingSince,omitzero" mapstructure:"failingSince"`
	LastDeliveredAt time.Time         `json:"lastDeliveredAt,omitzero" mapstructure:"lastDeliveredAt"`
	LastError       string            `json:"lastError,omitempty" mapstructure:"lastError"`
	CreatedAt       time.Time         `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}

// EventDelivery is one event's delivery to an [EventDestination].
//
// NextAttemptAt is set while Status is PENDING. Event is nil when the
// event is older than the audit log retention.
type EventDelivery struct {
	ID            string    `json:"id" mapstructure:"id"`
	Status        string    `json:"status" mapstructure:"status"`
	Attempts      int       `json:"attempts" mapstructure:"attempts"`
	NextAttemptAt time.Time `json:"nextAttemptAt,omitzero" mapstructure:"nextAttemptAt"`
	LastError     string    `json:"lastError,omitempty" mapstructure:"lastError"`
	DeliveredAt   time.Time `json:"deliveredAt,omitzero" mapstructure:"deliveredAt"`
	CreatedAt     time.Time `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	Event         *AuditLog `json:"event,omitempty" mapstructure:"event,omitempty"`
}

// EventDestinationAdapter is a way to send events. ConfigSchema and
// AuthSchema are the JSON schemas of a destination's config and auth.
// Signs reports whether destinations of the adapter get a signing secret.
type EventDestinationAdapter struct {
	ID           string         `json:"id" mapstructure:"id"`
	Name         string         `json:"name" mapstructure:"name"`
	Description  string         `json:"description" mapstructure:"description"`
	ConfigSchema map[string]any `json:"configSchema" mapstructure:"configSchema"`
	AuthSchema   map[string]any `json:"authSchema" mapstructure:"authSchema"`
	Signs        bool           `json:"signs" mapstructure:"signs"`
}

// EventType is one entry in the Massdriver event catalog. ID is a dotted,
// past-tense `entity.action` id such as "deployment.completed".
type EventType struct {
	ID          string               `json:"id" mapstructure:"id"`
	Description string               `json:"description" mapstructure:"description"`
	Entity      string               `json:"entity" mapstructure:"entity"`
	Action      string               `json:"action" mapstructure:"action"`
	ChangeType  string               `json:"changeType" mapstructure:"changeType"`
	Metadata    []EventMetadataField `json:"metadata" mapstructure:"metadata"`
}

// EventMetadataField is a fact Massdriver records with every event of an
// [EventType], such as how the actor authenticated.
type EventMetadataField struct {
	Name        string `json:"name" mapstructure:"name"`
	Type        string `json:"type" mapstructure:"type"`
	Description string `json:"description" mapstructure:"description"`
}
