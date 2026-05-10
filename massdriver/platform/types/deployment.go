package types

import "time"

// Deployment is a record of an infrastructure provisioning operation against
// an [Instance]. Each deployment carries a single action (PROVISION,
// DECOMMISSION, or PLAN), the bundle version that ran, the snapshotted
// params, and the lifecycle state.
//
// Deployments are immutable once created — modifications happen by creating
// new deployments. Logs are not embedded on this type; use
// deployments.GetLogs for a one-shot snapshot or deployments.StreamLogs to
// tail live updates.
type Deployment struct {
	ID                 string         `json:"id" mapstructure:"id"`
	Status             string         `json:"status" mapstructure:"status"`
	Action             string         `json:"action" mapstructure:"action"`
	Version            string         `json:"version" mapstructure:"version"`
	Params             map[string]any `json:"params,omitempty" mapstructure:"params"`
	Message            string         `json:"message,omitempty" mapstructure:"message"`
	DeployedBy         string         `json:"deployedBy,omitempty" mapstructure:"deployedBy"`
	ElapsedTime        int            `json:"elapsedTime" mapstructure:"elapsedTime"`
	CreatedAt          time.Time      `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
	LastTransitionedAt time.Time      `json:"lastTransitionedAt,omitzero" mapstructure:"lastTransitionedAt"`

	// Instance is the parent instance this deployment operated on. Populated
	// by deployments.Get / deployments.List with id/name and slim
	// environment/bundle/component refs; nested fields (params, secrets,
	// resources) are not populated.
	Instance *Instance `json:"instance,omitempty" mapstructure:"instance,omitempty"`
}

// DeploymentLogBatch is a single batch of deployment logs flushed by the
// provisioner during a deployment. One batch corresponds to one worker
// flush — the message may span multiple lines separated by `\n`.
//
// This is the streaming unit. For one-shot fetches that just want the text,
// use deployments.GetLogs (which concatenates batches into a single string);
// for live tailing that surfaces per-batch timestamps, use
// deployments.StreamLogs.
type DeploymentLogBatch struct {
	Timestamp time.Time `json:"timestamp,omitzero" mapstructure:"timestamp"`
	Message   string    `json:"message" mapstructure:"message"`
}
