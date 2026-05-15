package instances

import (
	"context"
	"encoding/json"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/stream"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

const instanceEventsSubscription = `subscription instanceEvents($organizationId: ID!, $instanceId: ID!) {
  instanceEvents(organizationId: $organizationId, instanceId: $instanceId) {
    __typename
    ... on Event { action timestamp }
    ... on InstanceEvent {
      instance { id name status }
    }
    ... on ConnectionEvent {
      connection { id fromField toField }
    }
    ... on AlarmEvent {
      alarm {
        id
        displayName
        currentState { id status message occurredAt }
      }
    }
    ... on DeploymentEvent {
      deployment { id status action elapsedTime }
    }
  }
}`

// StreamEvents opens an Absinthe subscription for the named instance's
// event feed. Each frame yields one of [*types.InstanceEvent],
// [*types.ConnectionEvent], [*types.AlarmEvent], or [*types.DeploymentEvent]
// — fires when the instance's configuration changes, an incoming
// connection is added or removed, an attached alarm is registered /
// updated / deleted (or changes firing state), or a deployment runs
// against the instance.
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription
// and the underlying WebSocket and closes the returned channel.
//
// Streaming requires PAT (bearer) authentication; basic-auth callers
// get [streaming.ErrRequiresPAT] before any network I/O.
func (s *Service) StreamEvents(ctx context.Context, instanceID string) (<-chan types.Event, error) {
	return stream.Events(
		ctx,
		s.client,
		"instance events for "+instanceID,
		instanceEventsSubscription,
		map[string]any{
			"organizationId": s.client.Config.OrganizationID,
			"instanceId":     instanceID,
		},
		unpackInstanceEvents,
	)
}

func unpackInstanceEvents(raw json.RawMessage) (types.Event, bool) {
	var env struct {
		Body json.RawMessage `json:"instanceEvents"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Body) == 0 {
		return nil, false
	}
	switch stream.Typename(env.Body) {
	case "InstanceEvent":
		var ev types.InstanceEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "ConnectionEvent":
		var ev types.ConnectionEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "AlarmEvent":
		var ev types.AlarmEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "DeploymentEvent":
		var ev types.DeploymentEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	}
	return nil, false
}
