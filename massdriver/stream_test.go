package massdriver_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/absinthe/absinthetest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/stash"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/streaming"
)

func newStreamingClient(t *testing.T, srv *absinthetest.Server) *massdriver.Client {
	t.Helper()
	isolateEnv(t)
	c, err := massdriver.NewClient(
		massdriver.WithAPIKey("mds_test"),
		massdriver.WithOrganizationID("my-org"),
		massdriver.WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func subscribe(ctx context.Context, t *testing.T, c *massdriver.Client) *streaming.Stream[stash.Event] {
	t.Helper()
	s, err := c.Stash.Subscribe(ctx, "api", "")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	return s
}

func stashEvent(key string) map[string]any {
	return map[string]any{"alphaStashEvents": map[string]any{
		"action":    "SET",
		"scope":     map[string]any{"kind": "PROJECT", "id": "api"},
		"key":       key,
		"entry":     nil,
		"timestamp": "2026-11-20T10:00:00Z",
	}}
}

func receive(t *testing.T, s *streaming.Stream[stash.Event]) stash.Event {
	t.Helper()
	select {
	case ev, ok := <-s.C:
		if !ok {
			t.Fatalf("stream closed, want an event (Err = %v)", s.Err())
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an event")
	}
	return stash.Event{}
}

// waitClosed drains s until its channel closes and returns its Err.
func waitClosed(t *testing.T, s *streaming.Stream[stash.Event]) error {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-s.C:
			if !ok {
				return s.Err()
			}
		case <-timeout:
			t.Fatal("timed out waiting for the stream to close")
		}
	}
}

func TestStreams_ShareOneSocket(t *testing.T) {
	srv := absinthetest.NewServer(t)
	c := newStreamingClient(t, srv)

	ctx1, cancel1 := context.WithCancel(t.Context())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(t.Context())
	defer cancel2()
	first := subscribe(ctx1, t, c)
	second := subscribe(ctx2, t, c)
	ids := srv.WaitForSubscriptions(2)
	if n := srv.Dials(); n != 1 {
		t.Fatalf("Dials = %d, want 1 shared socket", n)
	}

	cancel1()
	if err := waitClosed(t, first); err != nil {
		t.Errorf("first.Err = %v, want nil after cancel", err)
	}
	srv.WaitForUnsubscribe(ids[0])

	srv.Push(ids[1], stashEvent("still-open"))
	if ev := receive(t, second); ev.Key != "still-open" {
		t.Errorf("second got %+v", ev)
	}

	cancel2()
	if err := waitClosed(t, second); err != nil {
		t.Errorf("second.Err = %v, want nil after cancel", err)
	}
	srv.WaitForOpenConns(0)
}

func TestStreams_ServerCompletesOne(t *testing.T) {
	srv := absinthetest.NewServer(t)
	c := newStreamingClient(t, srv)

	first := subscribe(t.Context(), t, c)
	second := subscribe(t.Context(), t, c)
	ids := srv.WaitForSubscriptions(2)

	srv.Complete(ids[0])
	if err := waitClosed(t, first); err != nil {
		t.Errorf("first.Err = %v, want nil when the server completes it", err)
	}

	srv.Push(ids[1], stashEvent("still-open"))
	if ev := receive(t, second); ev.Key != "still-open" {
		t.Errorf("second got %+v", ev)
	}
}

func TestStreams_SocketDies(t *testing.T) {
	srv := absinthetest.NewServer(t)
	c := newStreamingClient(t, srv)

	first := subscribe(t.Context(), t, c)
	second := subscribe(t.Context(), t, c)
	srv.WaitForSubscriptions(2)

	srv.Kill()
	for name, s := range map[string]*streaming.Stream[stash.Event]{"first": first, "second": second} {
		if err := waitClosed(t, s); err == nil {
			t.Errorf("%s.Err = nil, want the socket's error", name)
		}
	}

	third := subscribe(t.Context(), t, c)
	ids := srv.WaitForSubscriptions(3)
	if n := srv.Dials(); n != 2 {
		t.Errorf("Dials = %d, want a new socket after the first died", n)
	}
	srv.Push(ids[2], stashEvent("after-reconnect"))
	if ev := receive(t, third); ev.Key != "after-reconnect" {
		t.Errorf("third got %+v", ev)
	}
}

func TestStreams_ConcurrentSubscribe(t *testing.T) {
	srv := absinthetest.NewServer(t)
	c := newStreamingClient(t, srv)

	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Stash.Subscribe(t.Context(), "api", ""); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("Subscribe: %v", err)
	}

	srv.WaitForSubscriptions(n)
	if got := srv.Dials(); got != 1 {
		t.Errorf("Dials = %d, want 1", got)
	}
}
