package organizations

import (
	"context"
	"encoding/json"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/stream"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

const organizationEventsSubscription = `subscription organizationEvents($organizationId: ID!) {
  organizationEvents(organizationId: $organizationId) {
    __typename
    ... on Event { action timestamp }
    ... on ProjectEvent {
      project { id name }
    }
    ... on OciRepoEvent {
      ociRepo { id name reference }
    }
    ... on BundleEvent {
      bundle { id name version }
    }
  }
}`

// StreamEvents opens an Absinthe subscription for organization-level
// events. Each frame yields one of [*types.ProjectEvent],
// [*types.OciRepoEvent], or [*types.BundleEvent] — fires when projects
// are created, OCI repositories are created in the bundle catalog, or
// bundle versions are published to those repositories.
//
// The configured organization id is used implicitly; no organization
// parameter is required.
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription
// and the underlying WebSocket and closes the returned channel.
//
// Streaming requires PAT (bearer) authentication; basic-auth callers
// get [streaming.ErrRequiresPAT] before any network I/O.
func (s *Service) StreamEvents(ctx context.Context) (<-chan types.Event, error) {
	return stream.Events(
		ctx,
		s.client,
		"organization events for "+s.client.Config.OrganizationID,
		organizationEventsSubscription,
		map[string]any{
			"organizationId": s.client.Config.OrganizationID,
		},
		unpackOrganizationEvents,
	)
}

func unpackOrganizationEvents(raw json.RawMessage) (types.Event, bool) {
	var env struct {
		Body json.RawMessage `json:"organizationEvents"`
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
	case "OciRepoEvent":
		var ev types.OciRepoEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	case "BundleEvent":
		var ev types.BundleEvent
		if err := json.Unmarshal(env.Body, &ev); err != nil {
			return nil, false
		}
		return &ev, true
	}
	return nil, false
}
