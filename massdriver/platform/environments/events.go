package environments

import (
	"context"
	"encoding/json"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/stream"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

const environmentEventsSubscription = `subscription environmentEvents($organizationId: ID!, $environmentId: ID!) {
  environmentEvents(organizationId: $organizationId, environmentId: $environmentId) {
    __typename
    ... on Event { action timestamp }
    ... on EnvironmentEvent {
      environment { id name }
    }
    ... on EnvironmentDefaultEvent {
      environmentDefault {
        id
        resource { id name }
      }
    }
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

// StreamEvents opens an Absinthe subscription for events within the
// named environment. Each frame yields one of [*types.EnvironmentEvent],
// [*types.EnvironmentDefaultEvent], [*types.InstanceEvent],
// [*types.ConnectionEvent], [*types.AlarmEvent], or
// [*types.DeploymentEvent] — covering updates to the environment, its
// default resources, every instance / connection / alarm in it, and
// every deployment run against those instances.
//
// Environment creation events are delivered on the parent project's
// subscription (the environment must exist before you can subscribe to
// it), so callers wanting full lifecycle coverage should listen to both.
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription
// and the underlying WebSocket and closes the returned channel.
//
// Streaming requires PAT (bearer) authentication; basic-auth callers
// get [streaming.ErrRequiresPAT] before any network I/O.
func (s *Service) StreamEvents(ctx context.Context, environmentID string) (<-chan types.Event, error) {
	return stream.Events(
		ctx,
		s.client,
		"environment events for "+environmentID,
		environmentEventsSubscription,
		map[string]any{
			"organizationId": s.client.Config.OrganizationID,
			"environmentId":  environmentID,
		},
		unpackEnvironmentEvents,
	)
}

func unpackEnvironmentEvents(raw json.RawMessage) (types.Event, bool) {
	var env struct {
		Body json.RawMessage `json:"environmentEvents"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Body) == 0 {
		return nil, false
	}
	switch stream.Typename(env.Body) {
	case "EnvironmentEvent":
		var ev types.EnvironmentEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "EnvironmentDefaultEvent":
		var ev types.EnvironmentDefaultEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
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
