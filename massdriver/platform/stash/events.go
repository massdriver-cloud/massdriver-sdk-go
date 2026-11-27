package stash

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/stream"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Event says a stash entry was set or deleted — alias of
// [types.StashEvent].
type Event = types.StashEvent

// EventAction is what happened to an entry — alias of
// [types.StashEventAction].
type EventAction = types.StashEventAction

const (
	// EventSet is the action of an [Event] for a key that was set.
	EventSet = types.StashEventSet
	// EventDeleted is the action of an [Event] for a key that was deleted.
	EventDeleted = types.StashEventDeleted
)

const stashEventsSubscription = `subscription alphaStashEvents($organizationId: ID!, $projectId: ID!, $environmentId: ID) {
  alphaStashEvents(organizationId: $organizationId, projectId: $projectId, environmentId: $environmentId) {
    action
    scope { kind id }
    key
    entry {
      scope { kind id }
      key
      label
      value
      format
      immutable
      sensitive
      expiresAt
      updatedAt
    }
    timestamp
  }
}`

// Subscribe streams the stash entries that are set or deleted in a project.
// Without environmentID it covers the project, its environments, and its
// components. With environmentID it also covers that environment's
// instances and their deployments.
//
// An event's Entry is nil for a delete. A sensitive entry's Value is always
// nil in an event; read it with [Service.Get]. A JSON entry's Data holds its
// parsed Value, and is nil when Value is nil or is not valid JSON. An entry
// that expires sends no event. A frame that does not decode is skipped.
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription and
// closes the returned channel. Streaming requires PAT (bearer)
// authentication; basic-auth callers get [streaming.ErrRequiresPAT] before
// any network I/O.
func (s *Service) Subscribe(ctx context.Context, projectID, environmentID string) (<-chan Event, error) {
	if projectID == "" {
		return nil, errors.New("subscribe to stash events: a project ID is required")
	}
	vars := map[string]any{
		"organizationId": s.client.Config.OrganizationID,
		"projectId":      projectID,
	}
	if environmentID != "" {
		vars["environmentId"] = environmentID
	}
	return stream.Frames(ctx, s.client, "stash events for "+projectID, stashEventsSubscription, vars, unpackEvent)
}

func unpackEvent(raw json.RawMessage) (Event, bool) {
	var env struct {
		Body *Event `json:"alphaStashEvents"`
	}
	if err := json.Unmarshal(raw, &env); err != nil || env.Body == nil {
		return Event{}, false
	}
	ev := *env.Body
	lowerScopeKind(&ev.Scope.Kind)
	if ev.Entry != nil {
		lowerScopeKind(&ev.Entry.Scope.Kind)
		if ev.Entry.Format == string(FormatJSON) && ev.Entry.Value != nil {
			_ = json.Unmarshal([]byte(*ev.Entry.Value), &ev.Entry.Data)
		}
	}
	return ev, true
}
