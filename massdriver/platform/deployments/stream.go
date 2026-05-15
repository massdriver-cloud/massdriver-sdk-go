package deployments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/absinthe"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// LogBatch is one batch of deployment logs flushed by the provisioner —
// alias of [types.DeploymentLogBatch]. One batch corresponds to a single
// worker flush; the [LogBatch.Message] field may span multiple lines
// separated by `\n`.
type LogBatch = types.DeploymentLogBatch

// ErrStreamingRequiresPAT is returned by [Service.StreamLogs] (and by
// [Service.TailLogs] when streaming is needed) when the configured
// credentials are not a personal access token. WebSocket subscriptions
// authenticate via a query-string token, which only works for PATs
// (basic-auth API keys are rejected).
//
// Callers can [errors.Is] against this sentinel to surface a useful hint.
var ErrStreamingRequiresPAT = errors.New(
	"deployment log streaming requires a personal access token (set MASSDRIVER_API_KEY to a token starting with mds_/md_)",
)

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

// StreamLogs opens an Absinthe subscription for the named deployment's log
// stream. Each [LogBatch] yielded on the returned channel is one
// provisioner flush — the natural unit for live tailing.
//
// Lifetime is owned by ctx. Cancelling ctx tears down the subscription and
// the underlying WebSocket and closes the returned channel. The channel
// also closes if the server completes the subscription or the socket
// dies. Callers should follow the standard pattern:
//
//	ctx, cancel := context.WithCancel(parent)
//	defer cancel()
//	batches, err := svc.StreamLogs(ctx, deploymentID)
//	if err != nil {
//	    return err
//	}
//	for batch := range batches {
//	    fmt.Print(batch.Message)
//	}
//
// Streaming requires PAT (bearer) authentication; basic-auth callers get
// [ErrStreamingRequiresPAT] before any network I/O.
//
// Most callers want [Service.TailLogs] instead — it folds together backfill,
// terminal detection, and live tailing.
func (s *Service) StreamLogs(ctx context.Context, deploymentID string) (<-chan LogBatch, error) {
	socket, err := openLogStreamSocket(ctx, s.client)
	if err != nil {
		return nil, err
	}

	sub, err := socket.Subscribe(ctx, deploymentLogsSubscription, map[string]any{
		"organizationId": s.client.Config.OrganizationID,
		"deploymentId":   deploymentID,
	})
	if err != nil {
		_ = socket.Close()
		return nil, fmt.Errorf("subscribe to logs for deployment %s: %w", deploymentID, err)
	}

	// Tie socket cleanup to ctx so callers don't have to remember a separate
	// cancel function — `defer cancel()` on the parent ctx is enough.
	go func() {
		<-ctx.Done()
		_ = sub.Close()
		_ = socket.Close()
	}()

	out := make(chan LogBatch, cap(sub.Data))
	go func() {
		defer close(out)
		for raw := range sub.Data {
			batch, ok := unpackLogBatch(raw)
			if !ok {
				continue
			}
			select {
			case out <- batch:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
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

	socket, err := openLogStreamSocket(ctx, s.client)
	if err != nil {
		return err
	}
	defer socket.Close()

	vars := map[string]any{
		"organizationId": s.client.Config.OrganizationID,
		"deploymentId":   deploymentID,
	}
	logSub, err := socket.Subscribe(ctx, deploymentLogsSubscription, vars)
	if err != nil {
		return fmt.Errorf("subscribe to logs for deployment %s: %w", deploymentID, err)
	}
	eventSub, err := socket.Subscribe(ctx, deploymentEventsSubscription, vars)
	if err != nil {
		return fmt.Errorf("subscribe to events for deployment %s: %w", deploymentID, err)
	}

	// terminal closes when an event reports a terminal status. If the events
	// subscription closes without a terminal event (socket died), terminal
	// stays unclosed — but in that case logSub.Data will also have closed and
	// the main loop exits via the !ok branch.
	terminal := make(chan struct{})
	go func() {
		for raw := range eventSub.Data {
			status, ok := unpackEventStatus(raw)
			if ok && IsTerminal(status) {
				close(terminal)
				return
			}
		}
	}()

	for {
		select {
		case raw, ok := <-logSub.Data:
			if !ok {
				return nil
			}
			batch, ok := unpackLogBatch(raw)
			if !ok {
				continue
			}
			if werr := writeLogBatch(w, batch); werr != nil {
				return werr
			}
		case <-terminal:
			return drainLogs(ctx, logSub.Data, w)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// openLogStreamSocket gates streaming on PAT auth and opens an Absinthe
// socket. Shared between [StreamLogs] and [TailLogs] so the auth check
// stays in one place.
func openLogStreamSocket(ctx context.Context, mdClient *client.Client) (*absinthe.Socket, error) {
	if mdClient.Config.Credentials.Method != config.AuthPAT {
		return nil, ErrStreamingRequiresPAT
	}
	socket, err := absinthe.Dial(ctx, mdClient.Config.URL, mdClient.Config.Credentials.Secret)
	if err != nil {
		return nil, fmt.Errorf("open absinthe socket: %w", err)
	}
	return socket, nil
}

// drainLogs reads any in-transit log batches for a brief window after a
// terminal event has fired, then returns. Terminal events are emitted
// after the provisioner's final log flush, so anything still arriving is
// already buffered between the websocket and the absinthe forwarding
// goroutine — the grace window catches that hand-off without making
// callers wait noticeably.
func drainLogs(ctx context.Context, logsCh <-chan json.RawMessage, w io.Writer) error {
	timeout := time.After(drainGrace)
	for {
		select {
		case raw, ok := <-logsCh:
			if !ok {
				return nil
			}
			batch, ok := unpackLogBatch(raw)
			if !ok {
				continue
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
