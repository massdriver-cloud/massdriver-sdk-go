package types

// PolicyAction is one entry in the ABAC action catalog. Actions follow
// the `{entity}:{verb}` format (e.g. "project:view", "instance:deploy")
// and are the building blocks of [Policy]s.
//
// The catalog is exposed at runtime via platform/policies.ListActions
// rather than as a static enum, so the server can grow the action set
// without breaking clients.
type PolicyAction struct {
	// ID is the canonical action identifier, e.g. "project:view".
	ID string `json:"id" mapstructure:"id"`
	// Verb is the action portion of the id, e.g. "view".
	Verb string `json:"verb" mapstructure:"verb"`
	// Entity identifies the entity this action applies to (id matches
	// the prefix of [PolicyAction.ID]).
	Entity *PolicyEntity `json:"entity,omitempty" mapstructure:"entity,omitempty"`
	// Description is a human-readable explanation of what the action
	// permits — written for end-user docs and policy-authoring UIs.
	Description string `json:"description" mapstructure:"description"`
}

// PolicyEntity is one of the entity kinds an action can apply to (e.g.
// "project", "environment"). Surfaced via platform/policies.ListEntities
// when grouping actions by what they apply to in a UI.
type PolicyEntity struct {
	ID          string `json:"id" mapstructure:"id"`
	Description string `json:"description" mapstructure:"description"`
}

// PolicyDecision is the result of [platform/policies.Evaluate] or one
// element of a [platform/policies.EvaluateBatch] result. Action and
// EntityID echo the request inputs so batch callers can correlate
// decisions without tracking positions.
type PolicyDecision struct {
	Allowed  bool   `json:"allowed" mapstructure:"allowed"`
	Action   string `json:"action" mapstructure:"action"`
	EntityID string `json:"entityId" mapstructure:"entityId"`
}
