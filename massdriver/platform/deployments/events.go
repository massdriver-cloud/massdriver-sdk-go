package deployments

import (
	"context"
	"encoding/json"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/stream"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// deploymentEventsStreamSubscription is the GraphQL subscription pushed
// for [Service.StreamEvents]. The union has a single member
// ([types.DeploymentEvent]) but we still query `__typename` so the
// generic unpacker can route the frame uniformly with the other event
// streams.
const deploymentEventsStreamSubscription = `subscription deploymentEvents($organizationId: ID!, $deploymentId: ID!) {
  deploymentEvents(organizationId: $organizationId, deploymentId: $deploymentId) {
    __typename
    ... on Event { action timestamp }
    ... on DeploymentEvent {
      deployment {
        id
        status
        action
        elapsedTime
      }
    }
  }
}`

// StreamEvents opens an Absinthe subscription for the named deployment's
// lifecycle events. Each frame yields a [*types.DeploymentEvent] —
// fires on create and every status transition (PENDING → RUNNING →
// COMPLETED). Log content is not carried; use [Service.StreamLogs] for
// that.
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription
// and the underlying WebSocket and closes the returned channel.
//
// Streaming requires PAT (bearer) authentication; basic-auth callers
// get [streaming.ErrRequiresPAT] before any network I/O.
func (s *Service) StreamEvents(ctx context.Context, deploymentID string) (<-chan types.Event, error) {
	return stream.Events(
		ctx,
		s.client,
		"deployment events for "+deploymentID,
		deploymentEventsStreamSubscription,
		map[string]any{
			"organizationId": s.client.Config.OrganizationID,
			"deploymentId":   deploymentID,
		},
		unpackDeploymentEvents,
	)
}

func unpackDeploymentEvents(raw json.RawMessage) (types.Event, bool) {
	var env struct {
		Body json.RawMessage `json:"deploymentEvents"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Body) == 0 {
		return nil, false
	}
	switch stream.Typename(env.Body) {
	case "DeploymentEvent":
		var ev types.DeploymentEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	}
	return nil, false
}
