package deployments_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/deployments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *deployments.Service {
	return deployments.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{
				"id":          "dep-uuid-1",
				"status":      "RUNNING",
				"action":      "PROVISION",
				"version":     "1.2.3",
				"params":      map[string]any{"size": "large"},
				"message":     "Scale up",
				"elapsedTime": 120,
				"deployedBy":  "alice@example.com",
				"instance": map[string]any{
					"id":   "ecomm-prod-database",
					"name": "Primary Database",
					"environment": map[string]any{
						"id":   "ecomm-prod",
						"name": "Production",
						"project": map[string]any{
							"id":   "ecomm",
							"name": "E-Commerce",
						},
					},
					"bundle": map[string]any{
						"id":      "aws-aurora-postgres@1.2.3",
						"name":    "aws-aurora-postgres",
						"version": "1.2.3",
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "dep-uuid-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "dep-uuid-1" {
		t.Errorf("ID = %q, want dep-uuid-1", got.ID)
	}
	if got.Status != "RUNNING" {
		t.Errorf("Status = %q, want RUNNING", got.Status)
	}
	if got.ElapsedTime != 120 {
		t.Errorf("ElapsedTime = %d, want 120", got.ElapsedTime)
	}
	if got.Instance == nil || got.Instance.ID != "ecomm-prod-database" {
		t.Errorf("Instance = %+v, want ID ecomm-prod-database", got.Instance)
	}
	if got.Instance.Environment == nil || got.Instance.Environment.Project == nil {
		t.Errorf("Instance.Environment.Project missing: %+v", got.Instance.Environment)
	}

	// Compile-time check: instance ref is canonical types.Instance.
	var _ *types.Instance = got.Instance
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing deployment (the schema's `deployment`
// field is nullable, so a 404 manifests as a zero-valued struct on the
// wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deployment": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestGetLogs(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{
				"id": "dep-uuid-1",
				"logs": []map[string]any{
					// First batch ends in a newline — should pass through verbatim.
					{"timestamp": "2026-01-15T10:00:00Z", "message": "Initializing\nLoading state\n"},
					// Second batch has no trailing newline — wrapper must insert one
					// so the next batch doesn't fuse onto the same line.
					{"timestamp": "2026-01-15T10:00:30Z", "message": "Plan: 3 to add, 0 to change"},
					{"timestamp": "2026-01-15T10:01:00Z", "message": "Apply complete\n"},
				},
			},
		}),
	)

	got, err := newService(gqlClient).GetLogs(t.Context(), "dep-uuid-1")
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	want := "Initializing\nLoading state\nPlan: 3 to add, 0 to change\nApply complete\n"
	if got != want {
		t.Errorf("GetLogs = %q,\n want %q", got, want)
	}
}

func TestGetLogs_Empty(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deployment": map[string]any{"id": "dep-uuid-1", "logs": []map[string]any{}},
		}),
	)
	got, err := newService(gqlClient).GetLogs(t.Context(), "dep-uuid-1")
	if err != nil {
		t.Fatalf("GetLogs: %v", err)
	}
	if got != "" {
		t.Errorf("GetLogs = %q, want empty string", got)
	}
}

func TestList_FilterByInstanceAndStatus(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deployments": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "dep-1", "status": "COMPLETED", "action": "PROVISION"},
					{"id": "dep-2", "status": "COMPLETED", "action": "PROVISION"},
				},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), deployments.ListInput{
		InstanceID: "ecomm-prod-database",
		Status:     deployments.StatusCompleted,
	}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d deployments, want 2", len(got))
	}

	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)
	instID, _ := filter["instanceId"].(map[string]any)
	if instID["eq"] != "ecomm-prod-database" {
		t.Errorf("filter.instanceId.eq = %v, want ecomm-prod-database", instID["eq"])
	}
	status, _ := filter["status"].(map[string]any)
	if status["eq"] != "COMPLETED" {
		t.Errorf("filter.status.eq = %v, want COMPLETED", status["eq"])
	}
}

func TestList_AutoPaginates(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"deployments": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items":  []map[string]any{{"id": "dep-1", "status": "COMPLETED"}},
		},
	})
	page2 := gqltest.RespondWithData(map[string]any{
		"deployments": map[string]any{
			"cursor": map[string]any{},
			"items":  []map[string]any{{"id": "dep-2", "status": "RUNNING"}},
		},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), deployments.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d deployments across 2 pages, want 2", len(got))
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createDeployment": map[string]any{
				"result": map[string]any{
					"id":      "dep-new",
					"status":  "PENDING",
					"action":  "PROVISION",
					"version": "1.2.3",
					"message": "Scale up",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), "ecomm-prod-database", deployments.CreateInput{
		Action:  deployments.ActionProvision,
		Params:  map[string]any{"size": "large"},
		Message: "Scale up",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Status != "PENDING" {
		t.Errorf("Status = %q, want PENDING", got.Status)
	}

	// Verify the input round-trips through the JSON-double-encoded params scalar.
	reqs := gqlClient.Requests()
	if reqs[0].Variables["id"] != "ecomm-prod-database" {
		t.Errorf("id variable = %v, want ecomm-prod-database", reqs[0].Variables["id"])
	}
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["action"] != "PROVISION" {
		t.Errorf("input.action = %v, want PROVISION", input["action"])
	}
}

func TestCreate_ValidationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createDeployment": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "invalid", "field": "params.size", "message": "size must be one of small, medium, large"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).Create(t.Context(), "ecomm-prod-database", deployments.CreateInput{
		Action: deployments.ActionProvision,
		Params: map[string]any{"size": "huge"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T", err)
	}
	if mf.Op != "create deployment" {
		t.Errorf("Op = %q, want create deployment", mf.Op)
	}
}

func TestPropose(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"proposeDeployment": map[string]any{
				"result": map[string]any{
					"id":      "dep-proposed",
					"status":  "PROPOSED",
					"action":  "PROVISION",
					"version": "1.2.3",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Propose(t.Context(), "ecomm-prod-database", deployments.ProposeInput{
		Action:  deployments.ActionProvision,
		Params:  map[string]any{"size": "large"},
		Message: "Black Friday scale up",
	})
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if got.Status != "PROPOSED" {
		t.Errorf("Status = %q, want PROPOSED", got.Status)
	}
}

// TestIter_StopsEarly confirms that breaking out of the range loop
// stops further page requests — the iterator is lazy.
func TestIter_StopsEarly(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"deployments": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{
				{"id": "dep-1", "status": "COMPLETED", "action": "PROVISION"},
				{"id": "dep-2", "status": "RUNNING", "action": "PROVISION"},
			},
		},
	})
	// page2 would only be requested if the iterator follows the cursor.
	page2 := gqltest.RespondWithData(map[string]any{
		"deployments": map[string]any{"cursor": map[string]any{}, "items": []map[string]any{}},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	count := 0
	for dep, err := range newService(gqlClient).Iter(t.Context(), deployments.ListInput{}) {
		if err != nil {
			t.Fatalf("iter err: %v", err)
		}
		count++
		if dep.ID == "dep-1" {
			break // caller bails after the first deployment
		}
	}

	if count != 1 {
		t.Errorf("iterated %d items, want 1 (caller bailed after dep-1)", count)
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
	for _, err := range newService(gqlClient).Iter(t.Context(), deployments.ListInput{}) {
		observed = append(observed, err)
	}
	if len(observed) != 1 {
		t.Fatalf("got %d yields, want 1", len(observed))
	}
	if !errors.Is(observed[0], wantErr) {
		t.Errorf("err = %v, want it to wrap %v", observed[0], wantErr)
	}
}

func TestApproveRejectAbort(t *testing.T) {
	tests := []struct {
		name           string
		responseField  string
		wantStatusName string
		fn             func(*deployments.Service) (*deployments.Deployment, error)
	}{
		{
			name:           "approve",
			responseField:  "approveDeployment",
			wantStatusName: "APPROVED",
			fn: func(s *deployments.Service) (*deployments.Deployment, error) {
				return s.Approve(t.Context(), "dep-1")
			},
		},
		{
			name:           "reject",
			responseField:  "rejectDeployment",
			wantStatusName: "REJECTED",
			fn: func(s *deployments.Service) (*deployments.Deployment, error) {
				return s.Reject(t.Context(), "dep-1")
			},
		},
		{
			name:           "abort",
			responseField:  "abortDeployment",
			wantStatusName: "ABORTED",
			fn: func(s *deployments.Service) (*deployments.Deployment, error) {
				return s.Abort(t.Context(), "dep-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gqlClient := gqltest.NewClient(
				gqltest.RespondWithData(map[string]any{
					tt.responseField: map[string]any{
						"result":     map[string]any{"id": "dep-1", "status": tt.wantStatusName, "action": "PROVISION"},
						"successful": true,
					},
				}),
			)
			got, err := tt.fn(newService(gqlClient))
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if got.Status != tt.wantStatusName {
				t.Errorf("Status = %q, want %q", got.Status, tt.wantStatusName)
			}
		})
	}
}
