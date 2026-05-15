package auditlogs_test

import (
	"errors"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/auditlogs"
)

func newService(gqlClient *gqltest.Client) *auditlogs.Service {
	return auditlogs.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"auditLog": map[string]any{
				"id":         "evt-1",
				"occurredAt": "2026-05-08T10:00:00Z",
				"type":       "project.created",
				"source":     "massdriver/api",
				"subject":    "mri://organization/my-org/project/ecomm",
				"data":       map[string]any{"projectId": "ecomm"},
				"actor": map[string]any{
					"id":   "u-alice",
					"type": "ACCOUNT",
					"name": "alice@example.com",
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "evt-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Type != "project.created" {
		t.Errorf("Type = %q, want project.created", got.Type)
	}
	if got.Subject != "mri://organization/my-org/project/ecomm" {
		t.Errorf("Subject = %q, want MRI string", got.Subject)
	}
	if got.Data["projectId"] != "ecomm" {
		t.Errorf("Data[projectId] = %v, want ecomm", got.Data["projectId"])
	}
	if got.Actor == nil || got.Actor.Type != string(auditlogs.ActorAccount) {
		t.Errorf("Actor = %+v, want type=ACCOUNT", got.Actor)
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing audit log event (the schema's `auditLog`
// field is nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"auditLog": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestList_FilterByTypeAndTimeRange(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"auditLogs": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "evt-1", "type": "project.created", "occurredAt": "2026-05-08T10:00:00Z"},
					{"id": "evt-2", "type": "project.created", "occurredAt": "2026-05-08T11:00:00Z"},
				},
			},
		}),
	)

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC)
	got, err := newService(gqlClient).List(t.Context(), auditlogs.ListInput{
		Type:           "project.created",
		TimeRangeStart: start,
		TimeRangeEnd:   end,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}

	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)
	occurred, _ := filter["occurredAt"].(map[string]any)
	if occurred["gte"] == nil || occurred["lte"] == nil {
		t.Errorf("occurredAt = %v, want both gte and lte set", occurred)
	}
	typeFilter, _ := filter["type"].(map[string]any)
	if typeFilter["eq"] != "project.created" {
		t.Errorf("type.eq = %v, want project.created", typeFilter["eq"])
	}
}

func TestList_FilterByActor(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"auditLogs": map[string]any{
				"cursor": map[string]any{},
				"items":  []map[string]any{{"id": "evt-1", "type": "project.created"}},
			},
		}),
	)

	_, err := newService(gqlClient).List(t.Context(), auditlogs.ListInput{
		ActorTypes:  []auditlogs.ActorType{auditlogs.ActorAccount, auditlogs.ActorServiceAccount},
		ActorSearch: "alice",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)

	// ActorTypes goes through actorType.in.
	actorType, _ := filter["actorType"].(map[string]any)
	in, _ := actorType["in"].([]any)
	if len(in) != 2 || in[0] != "ACCOUNT" || in[1] != "SERVICE_ACCOUNT" {
		t.Errorf("actorType.in = %v, want [ACCOUNT SERVICE_ACCOUNT]", in)
	}

	// ActorSearch goes through actor.search.
	actor, _ := filter["actor"].(map[string]any)
	if actor["search"] != "alice" {
		t.Errorf("actor.search = %v, want alice", actor["search"])
	}
}

func TestList_NoFilters(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"auditLogs": map[string]any{
				"cursor": map[string]any{},
				"items":  []map[string]any{{"id": "evt-1", "type": "project.created"}},
			},
		}),
	)

	if _, err := newService(gqlClient).List(t.Context(), auditlogs.ListInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	// Empty ListInput should produce no filter variable on the wire.
	reqs := gqlClient.Requests()
	if reqs[0].Variables["filter"] != nil {
		t.Errorf("filter = %v, want null for empty input", reqs[0].Variables["filter"])
	}
}

func TestList_AutoPaginates(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"auditLogs": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items":  []map[string]any{{"id": "evt-1", "type": "project.created"}},
		},
	})
	page2 := gqltest.RespondWithData(map[string]any{
		"auditLogs": map[string]any{
			"cursor": map[string]any{},
			"items":  []map[string]any{{"id": "evt-2", "type": "project.updated"}},
		},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	got, err := newService(gqlClient).List(t.Context(), auditlogs.ListInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 across two pages", len(got))
	}
}

func TestListEventTypes(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"auditLogEventTypes": []string{
				"deployment.completed",
				"deployment.failed",
				"project.created",
			},
		}),
	)

	got, err := newService(gqlClient).ListEventTypes(t.Context())
	if err != nil {
		t.Fatalf("ListEventTypes: %v", err)
	}
	if len(got) != 3 || got[0] != "deployment.completed" {
		t.Errorf("got = %v, want [deployment.completed, deployment.failed, project.created]", got)
	}
}

// TestIter_StopsEarly confirms that breaking out of the range loop
// stops further page requests — the iterator is lazy.
func TestIter_StopsEarly(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"auditLogs": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{
				{"id": "evt-1", "occurredAt": "2026-05-08T10:00:00Z", "type": "project.created"},
				{"id": "evt-2", "occurredAt": "2026-05-08T10:01:00Z", "type": "project.updated"},
			},
		},
	})
	// page2 would only be requested if the iterator follows the cursor.
	page2 := gqltest.RespondWithData(map[string]any{
		"auditLogs": map[string]any{"cursor": map[string]any{}, "items": []map[string]any{}},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	count := 0
	for ev, err := range newService(gqlClient).Iter(t.Context(), auditlogs.ListInput{}) {
		if err != nil {
			t.Fatalf("iter err: %v", err)
		}
		count++
		if ev.ID == "evt-1" {
			break // caller bails after the first event
		}
	}

	if count != 1 {
		t.Errorf("iterated %d items, want 1 (caller bailed after evt-1)", count)
	}
	// Critical invariant: page2 was never requested because we stopped early.
	if got := len(gqlClient.Requests()); got != 1 {
		t.Errorf("issued %d requests, want 1 — iterator should not pre-fetch", got)
	}
	if pending := gqlClient.Pending(); pending != 1 {
		t.Errorf("expected 1 unconsumed mock response (page2), got %d", pending)
	}
}

// TestIter_TransportErrorYieldsOnce confirms a transport error is
// surfaced through the yielded error and the iterator stops.
func TestIter_TransportErrorYieldsOnce(t *testing.T) {
	wantErr := errors.New("dial tcp: refused")
	gqlClient := gqltest.NewClient(gqltest.RespondWithTransportError(wantErr))

	var observed []error
	for _, err := range newService(gqlClient).Iter(t.Context(), auditlogs.ListInput{}) {
		observed = append(observed, err)
	}
	if len(observed) != 1 {
		t.Fatalf("got %d yields, want 1", len(observed))
	}
	if !errors.Is(observed[0], wantErr) {
		t.Errorf("err = %v, want it to wrap %v", observed[0], wantErr)
	}
}

// TestIter_StopsAfterMidIterationError confirms the iterator surfaces a
// page-fetch error mid-iteration and exits — page 1's items are yielded
// successfully, page 2's transport error is yielded once, and the loop
// exits without further yields.
func TestIter_StopsAfterMidIterationError(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"auditLogs": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{
				{"id": "evt-1", "occurredAt": "2026-05-08T10:00:00Z", "type": "project.created"},
			},
		},
	})
	wantErr := errors.New("dial tcp: refused")
	gqlClient := gqltest.NewClient(page1, gqltest.RespondWithTransportError(wantErr))

	type yielded struct {
		log auditlogs.AuditLog
		err error
	}
	var observed []yielded
	for ev, err := range newService(gqlClient).Iter(t.Context(), auditlogs.ListInput{}) {
		observed = append(observed, yielded{ev, err})
	}

	if len(observed) != 2 {
		t.Fatalf("got %d yields, want 2 (one item, one error)", len(observed))
	}
	if observed[0].err != nil || observed[0].log.ID != "evt-1" {
		t.Errorf("yield[0] = (%+v, %v), want (evt-1, nil)", observed[0].log, observed[0].err)
	}
	if observed[1].err == nil {
		t.Fatalf("yield[1].err = nil, want non-nil")
	}
	if !errors.Is(observed[1].err, wantErr) {
		t.Errorf("yield[1].err = %v, want it to wrap %v", observed[1].err, wantErr)
	}
	if observed[1].log.ID != "" {
		t.Errorf("yield[1].log = %+v, want zero value alongside the error", observed[1].log)
	}
}

