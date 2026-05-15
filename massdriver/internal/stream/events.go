// Package stream holds the shared plumbing every domain-level
// StreamEvents wrapper sits on top of: opening an Absinthe socket,
// pushing the subscription, dispatching each typed frame, and tying
// teardown to the caller's context.
//
// The package is internal — external callers consume it indirectly
// through e.g. instances.Service.StreamEvents.
package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Events opens an Absinthe subscription, decodes each frame via unpack,
// and forwards successful unpacks on the returned channel. The channel
// closes when ctx is cancelled, the server completes the subscription,
// or the socket dies.
//
// rootField is the GraphQL field name nested under `data` (e.g.
// "instanceEvents"); unpack reads the `__typename` inside that body and
// returns the matching concrete [types.Event]. Frames with an unknown
// __typename (or malformed JSON) are skipped without tearing down the
// stream.
//
// opName labels the operation for error wrapping (e.g.
// "instance events for inst-abc"). The socket and subscription are
// closed automatically on ctx cancel — callers only own the returned
// channel.
func Events(
	ctx context.Context,
	c *client.Client,
	opName, query string,
	vars map[string]any,
	unpack func(json.RawMessage) (types.Event, bool),
) (<-chan types.Event, error) {
	socket, err := c.OpenStreamSocket(ctx)
	if err != nil {
		return nil, err
	}
	sub, err := socket.Subscribe(ctx, query, vars)
	if err != nil {
		_ = socket.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", opName, err)
	}

	go func() {
		<-ctx.Done()
		_ = sub.Close()
		_ = socket.Close()
	}()

	out := make(chan types.Event, cap(sub.Data))
	go func() {
		defer close(out)
		for raw := range sub.Data {
			ev, ok := unpack(raw)
			if !ok {
				continue
			}
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// Typename extracts the __typename field from a subscription frame's
// inner body. Returns "" if the frame is malformed or has no
// __typename selection — callers treat that as a skip.
func Typename(body json.RawMessage) string {
	var probe struct {
		Typename string `json:"__typename"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return ""
	}
	return probe.Typename
}
