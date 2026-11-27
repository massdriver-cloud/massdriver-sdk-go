package deployments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/stream"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/streaming"
)

// LogBatch is one batch of deployment logs flushed by the provisioner —
// alias of [types.DeploymentLogBatch]. One batch corresponds to a single
// worker flush; the [LogBatch.Message] field may span multiple lines
// separated by `\n`.
type LogBatch = types.DeploymentLogBatch

// ErrStreamingRequiresPAT aliases [streaming.ErrRequiresPAT] so existing
// callers that match against this name still classify the error
// correctly under [errors.Is]. New code should reference
// [streaming.ErrRequiresPAT] directly.
var ErrStreamingRequiresPAT = streaming.ErrRequiresPAT

// deploymentLogsSubscription is the GraphQL operation pushed on the
// Absinthe control channel for log batches. Kept in lockstep with the
// schema's `deploymentLogs` subscription field — the server replies with
// one [LogBatch] per worker flush.
const deploymentLogsSubscription = `subscription deploymentLogs($organizationId: ID!, $deploymentId: ID!) {
  deploymentLogs(organizationId: $organizationId, deploymentId: $deploymentId) {
    timestamp
    message
  }
}`

// deploymentEventsSubscription is the GraphQL operation that fires on
// status transitions. [Service.TailLogs] uses it to detect terminal state
// without polling.
const deploymentEventsSubscription = `subscription deploymentEvents($organizationId: ID!, $deploymentId: ID!) {
  deploymentEvents(organizationId: $organizationId, deploymentId: $deploymentId) {
    ... on DeploymentEvent {
      action
      timestamp
      deployment {
        id
        status
      }
    }
  }
}`

// drainGrace is how long [Service.TailLogs] keeps reading from the log
// subscription after observing a terminal event, to capture any in-transit
// batches the websocket has already received but not yet handed up to the
// goroutine. The window is intentionally short — terminal events fire
// after the provisioner's final log flush, so anything still arriving is
// already buffered locally.
const drainGrace = 250 * time.Millisecond

// StreamLogs opens a subscription for the named deployment's log stream.
// Each [LogBatch] yielded on the returned stream is one provisioner flush —
// the natural unit for live tailing.
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription and
// closes the stream's channel. The channel also closes if the server
// completes the subscription or the connection fails; [streaming.Stream.Err]
// says which. Callers should follow the standard pattern:
//
//	ctx, cancel := context.WithCancel(parent)
//	defer cancel()
//	logs, err := svc.StreamLogs(ctx, deploymentID)
//	if err != nil {
//	    return err
//	}
//	for batch := range logs.C {
//	    fmt.Print(batch.Message)
//	}
//	if err := logs.Err(); err != nil {
//	    return err
//	}
//
// Streaming requires PAT (bearer) authentication; basic-auth callers get
// [ErrStreamingRequiresPAT] before any network I/O.
//
// Most callers want [Service.TailLogs] instead — it folds together backfill,
// terminal detection, and live tailing.
func (s *Service) StreamLogs(ctx context.Context, deploymentID string) (*streaming.Stream[LogBatch], error) {
	return stream.Frames(ctx, s.client, "logs for deployment "+deploymentID, deploymentLogsSubscription, map[string]any{
		"organizationId": s.client.Config.OrganizationID,
		"deploymentId":   deploymentID,
	}, unpackLogBatch)
}

// TailLogs writes the deployment's logs to w as they're produced, returning
// when the deployment reaches a terminal state, ctx is cancelled, the
// stream is closed by the server, or w returns an error.
//
// This is the high-level "just print my logs" path. It folds together:
//
//   - [Service.GetLogs] — emits the existing backfill before subscribing.
//   - [Service.Get] — short-circuits if the deployment is already terminal
//     so no stream is opened (and the auth gate doesn't fire).
//   - A live `deploymentLogs` subscription for new batches.
//   - A parallel `deploymentEvents` subscription that signals terminal
//     completion the moment the server records it — no polling, no
//     guessing how long to wait after status flips.
//
// Behavior:
//
//   - Always writes the existing log backfill first.
//   - If the deployment is already in a terminal state, returns nil after
//     the backfill.
//   - Otherwise opens both subscriptions, writes each log batch's message
//     to w as it arrives, and exits cleanly when an event subscription
//     reports terminal status. Reads any in-transit batches for a brief
//     grace window before returning.
//   - Inserts a trailing newline between batches that don't end in one.
//
// Streaming requires PAT (bearer) authentication when the deployment is
// still running; terminal-state callers don't open the stream and so don't
// hit the auth gate.
func (s *Service) TailLogs(ctx context.Context, deploymentID string, w io.Writer) error {
	// Backfill: emit everything already flushed.
	text, err := s.GetLogs(ctx, deploymentID)
	if err != nil {
		return err
	}
	if _, werr := io.WriteString(w, text); werr != nil {
		return werr
	}

	// If the deployment is already done, no streaming needed.
	dep, err := s.Get(ctx, deploymentID)
	if err != nil {
		return err
	}
	if IsTerminal(dep.Status) {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logs, err := s.StreamLogs(ctx, deploymentID)
	if err != nil {
		return err
	}
	statuses, err := stream.Frames(ctx, s.client, "events for deployment "+deploymentID, deploymentEventsSubscription, map[string]any{
		"organizationId": s.client.Config.OrganizationID,
		"deploymentId":   deploymentID,
	}, unpackEventStatus)
	if err != nil {
		return err
	}

	// terminal closes when an event reports a terminal status. eventsEnded
	// receives once if the events stream closes first, with its Err: the
	// server can end the events subscription while the log subscription
	// stays open.
	terminal := make(chan struct{})
	eventsEnded := make(chan error, 1)
	go func() {
		for status := range statuses.C {
			if IsTerminal(status) {
				close(terminal)
				return
			}
		}
		eventsEnded <- statuses.Err()
	}()

	for {
		select {
		case batch, ok := <-logs.C:
			if !ok {
				if err := ctx.Err(); err != nil {
					return err
				}
				return logs.Err()
			}
			if werr := writeLogBatch(w, batch); werr != nil {
				return werr
			}
		case <-terminal:
			return drainLogs(ctx, logs.C, w)
		case err := <-eventsEnded:
			return s.afterEventsEnded(ctx, deploymentID, err, logs.C, w)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// afterEventsEnded handles the events stream closing before a terminal
// status. Without the events, TailLogs cannot tell when the deployment
// finishes, so it returns the stream's error, or drains the logs when the
// deployment has finished, or reports that it stopped early.
func (s *Service) afterEventsEnded(ctx context.Context, deploymentID string, eventsErr error, logs <-chan LogBatch, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if eventsErr != nil {
		return fmt.Errorf("events for deployment %s: %w", deploymentID, eventsErr)
	}
	dep, err := s.Get(ctx, deploymentID)
	if err != nil {
		return err
	}
	if IsTerminal(dep.Status) {
		return drainLogs(ctx, logs, w)
	}
	return fmt.Errorf("events for deployment %s ended while it is %s", deploymentID, dep.Status)
}

// drainLogs reads any in-transit log batches for a brief window after a
// terminal event has fired, then returns. Terminal events are emitted
// after the provisioner's final log flush, so anything still arriving is
// already buffered between the websocket and the stream's forwarding
// goroutine — the grace window catches that hand-off without making
// callers wait noticeably.
func drainLogs(ctx context.Context, logsCh <-chan LogBatch, w io.Writer) error {
	timeout := time.After(drainGrace)
	for {
		select {
		case batch, ok := <-logsCh:
			if !ok {
				return nil
			}
			if werr := writeLogBatch(w, batch); werr != nil {
				return werr
			}
		case <-timeout:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// unpackLogBatch decodes one Absinthe data payload into a [LogBatch].
// Returns ok=false on a malformed frame so the caller can skip it without
// tearing down the stream.
func unpackLogBatch(raw json.RawMessage) (LogBatch, bool) {
	var env struct {
		DeploymentLogs LogBatch `json:"deploymentLogs"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return LogBatch{}, false
	}
	return env.DeploymentLogs, true
}

// unpackEventStatus pulls the deployment status out of one
// `deploymentEvents` data payload. Returns ok=false on malformed frames.
func unpackEventStatus(raw json.RawMessage) (string, bool) {
	var env struct {
		DeploymentEvents struct {
			Deployment struct {
				Status string `json:"status"`
			} `json:"deployment"`
		} `json:"deploymentEvents"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", false
	}
	return env.DeploymentEvents.Deployment.Status, true
}

// writeLogBatch writes one batch to w, appending a newline only when the
// provisioner's flush didn't already end in one. Empty messages are a
// no-op (no spurious blank line).
func writeLogBatch(w io.Writer, b LogBatch) error {
	if b.Message == "" {
		return nil
	}
	if _, err := io.WriteString(w, b.Message); err != nil {
		return err
	}
	if !strings.HasSuffix(b.Message, "\n") {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}
