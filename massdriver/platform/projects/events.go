package projects

import (
	"context"
	"encoding/json"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/stream"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

const projectEventsSubscription = `subscription projectEvents($organizationId: ID!, $projectId: ID!) {
  projectEvents(organizationId: $organizationId, projectId: $projectId) {
    __typename
    ... on Event { action timestamp }
    ... on ProjectEvent {
      project { id name }
    }
    ... on EnvironmentEvent {
      environment { id name }
    }
    ... on ComponentEvent {
      component { id name }
    }
    ... on LinkEvent {
      link { id fromField toField }
    }
  }
}`

// StreamEvents opens an Absinthe subscription for events within the
// named project. Each frame yields one of [*types.ProjectEvent],
// [*types.EnvironmentEvent], [*types.ComponentEvent], or
// [*types.LinkEvent] — covering the project itself, its environments,
// and its blueprint (components and links).
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription
// and the underlying WebSocket and closes the returned channel.
//
// Streaming requires PAT (bearer) authentication; basic-auth callers
// get [streaming.ErrRequiresPAT] before any network I/O.
func (s *Service) StreamEvents(ctx context.Context, projectID string) (<-chan types.Event, error) {
	return stream.Events(
		ctx,
		s.client,
		"project events for "+projectID,
		projectEventsSubscription,
		map[string]any{
			"organizationId": s.client.Config.OrganizationID,
			"projectId":      projectID,
		},
		unpackProjectEvents,
	)
}

func unpackProjectEvents(raw json.RawMessage) (types.Event, bool) {
	var env struct {
		Body json.RawMessage `json:"projectEvents"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Body) == 0 {
		return nil, false
	}
	switch stream.Typename(env.Body) {
	case "ProjectEvent":
		var ev types.ProjectEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "EnvironmentEvent":
		var ev types.EnvironmentEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "ComponentEvent":
		var ev types.ComponentEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "LinkEvent":
		var ev types.LinkEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	}
	return nil, false
}
