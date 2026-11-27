package deployments_test

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/absinthe/absinthetest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/deployments"
)

func TestIsTerminal(t *testing.T) {
	terminal := []string{"COMPLETED", "FAILED", "REJECTED", "ABORTED"}
	for _, s := range terminal {
		if !deployments.IsTerminal(s) {
			t.Errorf("IsTerminal(%q) = false, want true", s)
		}
	}
	nonTerminal := []string{"PROPOSED", "APPROVED", "PENDING", "RUNNING", "", "BOGUS"}
	for _, s := range nonTerminal {
		if deployments.IsTerminal(s) {
			t.Errorf("IsTerminal(%q) = true, want false", s)
		}
	}
}

// TestTailLogs_AlreadyComplete covers the no-streaming branch: when the
// deployment is already in a terminal state, TailLogs writes the backfill
// to w and returns without ever touching the websocket — so the test runs
// without any PAT credentials configured.
func TestTailLogs_AlreadyComplete(t *testing.T) {
	gqlClient := gqltest.NewClient(
		// (1) GetDeploymentLogs — backfill snapshot.
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{
				"id": "dep-1",
				"logs": []map[string]any{
					{"timestamp": "2026-05-08T10:00:00Z", "message": "Initializing\n"},
					{"timestamp": "2026-05-08T10:00:30Z", "message": "Apply complete\n"},
				},
			},
		}),
		// (2) GetDeployment — terminal status check.
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{
				"id":      "dep-1",
				"status":  "COMPLETED",
				"action":  "PROVISION",
				"version": "1.2.3",
			},
		}),
	)

	c := &client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	}

	var buf bytes.Buffer
	if err := deployments.New(c).TailLogs(t.Context(), "dep-1", &buf); err != nil {
		t.Fatalf("TailLogs: %v", err)
	}

	want := "Initializing\nApply complete\n"
	if buf.String() != want {
		t.Errorf("buf = %q, want %q", buf.String(), want)
	}

	// Both queries must have been consumed; no streaming attempt was made.
	if gqlClient.Pending() != 0 {
		t.Errorf("Pending = %d, want 0", gqlClient.Pending())
	}
	reqs := gqlClient.Requests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests (backfill + status), got %d", len(reqs))
	}
	if reqs[0].OpName != "GetDeploymentLogs" {
		t.Errorf("first request OpName = %q, want GetDeploymentLogs", reqs[0].OpName)
	}
	if reqs[1].OpName != "GetDeployment" {
		t.Errorf("second request OpName = %q, want GetDeployment", reqs[1].OpName)
	}
}

// TestTailLogs_RunningRejectsNonPAT confirms the auth gate fires when
// streaming is needed: backfill writes successfully, but the live stream
// can't open without a PAT, so TailLogs returns ErrStreamingRequiresPAT.
func TestTailLogs_RunningRejectsNonPAT(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{
				"id":   "dep-1",
				"logs": []map[string]any{{"timestamp": "2026-05-08T10:00:00Z", "message": "starting\n"}},
			},
		}),
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{
				"id":      "dep-1",
				"status":  "RUNNING",
				"action":  "PROVISION",
				"version": "1.2.3",
			},
		}),
	)

	c := &client.Client{
		Config: config.Config{
			OrganizationID: "my-org",
			URL:            "https://api.massdriver.cloud",
			Credentials:    config.Credentials{Method: config.AuthAPIKey, AuthHeaderValue: "Basic AAA="},
		},
		GQLv2: gqlClient,
	}

	var buf bytes.Buffer
	err := deployments.New(c).TailLogs(t.Context(), "dep-1", &buf)
	if err == nil {
		t.Fatal("expected ErrStreamingRequiresPAT, got nil")
	}
	if !errors.Is(err, deployments.ErrStreamingRequiresPAT) {
		t.Errorf("err = %v, want ErrStreamingRequiresPAT", err)
	}
	// Backfill should have been written before the auth gate fired.
	if buf.String() != "starting\n" {
		t.Errorf("buf = %q, want starting\\n (backfill should be written)", buf.String())
	}
}

// TestStreamLogs_RejectsNonPATAuth covers the auth-method gate that runs
// before any network I/O. Streaming uses a query-string token over
// WebSocket, which only PATs (bearer tokens) can satisfy — basic-auth
// API keys must be rejected up front.
func TestStreamLogs_RejectsNonPATAuth(t *testing.T) {
	tests := []struct {
		name  string
		creds config.Credentials
	}{
		{
			name: "api key (basic auth)",
			creds: config.Credentials{
				Method:          config.AuthAPIKey,
				AuthHeaderValue: "Basic dGVzdC1vcmc6dGVzdA==",
			},
		},
		{
			name: "deployment token (basic auth)",
			creds: config.Credentials{
				Method:          config.AuthDeployment,
				AuthHeaderValue: "Basic ZGVwbG95LXRva2VuOg==",
			},
		},
		{
			name:  "no credentials",
			creds: config.Credentials{}, // zero value — Method is empty, must be rejected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &client.Client{
				Config: config.Config{
					OrganizationID: "my-org",
					URL:            "https://api.massdriver.cloud",
					Credentials:    tt.creds,
				},
			}
			_, err := deployments.New(c).StreamLogs(t.Context(), "dep-1")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, deployments.ErrStreamingRequiresPAT) {
				t.Errorf("err = %v, want ErrStreamingRequiresPAT", err)
			}
		})
	}
}

func deploymentResponse(status string) gqltest.Response {
	return gqltest.RespondWithData(map[string]any{
		"deployment": map[string]any{"id": "dep-1", "status": status, "action": "PROVISION", "version": "1.2.3"},
	})
}

// startTailLogs runs TailLogs against a fake socket for a RUNNING
// deployment and returns the server, the live subscription IDs (logs, then
// events), TailLogs' output, and its result.
func startTailLogs(t *testing.T, extra ...gqltest.Response) (*absinthetest.Server, []string, *syncBuffer, <-chan error) {
	t.Helper()
	srv := absinthetest.NewServer(t)
	responses := append([]gqltest.Response{
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{
				"id":   "dep-1",
				"logs": []map[string]any{{"timestamp": "2026-05-08T10:00:00Z", "message": "starting\n"}},
			},
		}),
		deploymentResponse("RUNNING"),
	}, extra...)
	c := &client.Client{
		Config: config.Config{
			OrganizationID: "my-org",
			URL:            srv.URL,
			Credentials:    config.Credentials{Method: config.AuthPAT, Secret: "mds_test"},
		},
		GQLv2: gqltest.NewClient(responses...),
	}

	buf := &syncBuffer{}
	done := make(chan error, 1)
	go func() { done <- deployments.New(c).TailLogs(t.Context(), "dep-1", buf) }()
	return srv, srv.WaitForSubscriptions(2), buf, done
}

func waitResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("TailLogs did not return")
	}
	return nil
}

func TestTailLogs_LiveUntilTerminal(t *testing.T) {
	srv, ids, buf, done := startTailLogs(t)

	srv.Push(ids[0], map[string]any{"deploymentLogs": map[string]any{"timestamp": "2026-05-08T10:01:00Z", "message": "Apply complete"}})
	srv.Push(ids[1], map[string]any{"deploymentEvents": map[string]any{"deployment": map[string]any{"id": "dep-1", "status": "COMPLETED"}}})

	if err := waitResult(t, done); err != nil {
		t.Fatalf("TailLogs: %v", err)
	}
	if got := buf.String(); got != "starting\nApply complete\n" {
		t.Errorf("output = %q", got)
	}
}

func TestTailLogs_EventsFail(t *testing.T) {
	srv, ids, _, done := startTailLogs(t)

	srv.Fail(ids[1])

	if err := waitResult(t, done); err == nil {
		t.Error("TailLogs err = nil, want the events subscription's error")
	}
}

func TestTailLogs_EventsCompleteAfterDeploymentFinished(t *testing.T) {
	srv, ids, _, done := startTailLogs(t, deploymentResponse("COMPLETED"))

	srv.Complete(ids[1])

	if err := waitResult(t, done); err != nil {
		t.Errorf("TailLogs: %v, want nil when the deployment has finished", err)
	}
}

func TestTailLogs_EventsCompleteWhileRunning(t *testing.T) {
	srv, ids, _, done := startTailLogs(t, deploymentResponse("RUNNING"))

	srv.Complete(ids[1])

	err := waitResult(t, done)
	if err == nil || err.Error() != "events for deployment dep-1 ended while it is RUNNING" {
		t.Errorf("err = %v, want the events-ended error", err)
	}
}

func TestTailLogs_SocketDies(t *testing.T) {
	srv, _, _, done := startTailLogs(t)

	srv.Kill()

	if err := waitResult(t, done); err == nil {
		t.Error("TailLogs err = nil, want the socket's error")
	}
}

// syncBuffer is a bytes.Buffer that TailLogs can write while the test
// reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
