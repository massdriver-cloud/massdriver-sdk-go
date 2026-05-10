package types

import "time"

// AuditLog is one event in the organization's audit trail. Every
// state-changing operation in Massdriver is recorded as an audit log
// event following the CloudEvents specification.
//
// Event types use dot notation to categorize actions
// (e.g. "project.created", "deployment.completed", "group.member_added").
// The Subject field uses MRI (Massdriver Resource Identifier) format
// to identify the affected resource (e.g.
// "mri://organization/my-org/project/backend"). Subject can be empty
// for organization-wide events.
//
// Data carries event-specific context as a free-form map; its shape
// varies by event type.
type AuditLog struct {
	ID         string         `json:"id" mapstructure:"id"`
	OccurredAt time.Time      `json:"occurredAt,omitzero" mapstructure:"occurredAt"`
	Type       string         `json:"type" mapstructure:"type"`
	Source     string         `json:"source" mapstructure:"source"`
	Subject    string         `json:"subject,omitempty" mapstructure:"subject"`
	Data       map[string]any `json:"data,omitempty" mapstructure:"data,omitempty"`
	Actor      *AuditLogActor `json:"actor,omitempty" mapstructure:"actor,omitempty"`
}

// AuditLogActor identifies the entity that performed an audit-logged
// action. Inspect Type to know which kind of actor it is.
//
// Name is a human-readable label whose meaning depends on Type:
//   - ACCOUNT — the user's email address
//   - SERVICE_ACCOUNT — the service account's name
//   - DEPLOYMENT — a reference like "deployment:<id>"
//   - SYSTEM — "system" for internal actions
//
// If the actor has been deleted, Name will indicate it (e.g. "deleted account").
type AuditLogActor struct {
	ID   string `json:"id" mapstructure:"id"`
	Type string `json:"type" mapstructure:"type"`
	Name string `json:"name" mapstructure:"name"`
}
