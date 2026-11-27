// Package stream holds the shared plumbing every domain-level Stream*
// method sits on top of: subscribing on the client's shared Absinthe
// socket, decoding each frame, recording why the stream ended, and tying
// teardown to the caller's context.
//
// The package is internal — external callers consume it indirectly
// through e.g. instances.Service.StreamEvents.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/streaming"
)

// Events is [Frames] for the lifecycle-event subscriptions, whose frames
// decode to a [types.Event] variant.
//
// unpack reads the `__typename` inside the frame body and returns the
// matching concrete [types.Event]. Frames with an unknown __typename (or
// malformed JSON) are skipped without tearing down the stream.
func Events(
	ctx context.Context,
	c *client.Client,
	opName, query string,
	vars map[string]any,
	unpack func(json.RawMessage) (types.Event, bool),
) (*streaming.Stream[types.Event], error) {
	return Frames(ctx, c, opName, query, vars, unpack)
}

// Frames subscribes on c's shared socket, decodes each frame via unpack,
// and forwards successful unpacks on the returned stream. Frames that
// unpack rejects are skipped. The stream closes when ctx is cancelled, the
// server completes the subscription, or the socket dies; its Err is nil in
// the first two cases and the socket's error in the last.
//
// opName labels the operation for error wrapping (e.g. "instance events
// for inst-abc"). The subscription is closed and the socket released
// automatically — callers only own the returned stream.
func Frames[T any](
	ctx context.Context,
	c *client.Client,
	opName, query string,
	vars map[string]any,
	unpack func(json.RawMessage) (T, bool),
) (*streaming.Stream[T], error) {
	sub, release, err := c.Subscribe(ctx, query, vars)
	if err != nil {
		if errors.Is(err, streaming.ErrRequiresPAT) {
			return nil, err
		}
		return nil, fmt.Errorf("subscribe to %s: %w", opName, err)
	}

	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			_ = sub.Close()
			release()
		})
	}
	finished := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			stop()
		case <-finished:
		}
	}()

	var (
		mu     sync.Mutex
		endErr error
	)
	out := make(chan T, cap(sub.Data))
	go func() {
		defer stop()
		defer close(finished)
		defer close(out)
		for {
			var raw json.RawMessage
			select {
			case r, ok := <-sub.Data:
				if !ok {
					if ctx.Err() == nil {
						mu.Lock()
						endErr = sub.Err()
						mu.Unlock()
					}
					return
				}
				raw = r
			case <-ctx.Done():
				return
			}
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

	return streaming.New(out, func() error {
		mu.Lock()
		defer mu.Unlock()
		return endErr
	}), nil
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
