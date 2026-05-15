package instances_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *instances.Service {
	return instances.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"instance": map[string]any{
				"id":              "ecomm-prod-database",
				"name":            "Primary Database",
				"status":          "PROVISIONED",
				"version":         "~1.0",
				"resolvedVersion": "1.2.3",
				"deployedVersion": "1.2.3",
				"params":          map[string]any{"size": "small", "version": 14},
				"attributes":      map[string]any{"team": "platform"},
				"statePaths": []map[string]any{
					{"stepName": "core", "stateUrl": "https://state.example.com/core.tfstate"},
				},
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
				"component": map[string]any{
					"id":   "ecomm-database",
					"name": "Primary Database",
				},
				"resources": []map[string]any{
					{
						"resource": map[string]any{
							"id":     "res-auth",
							"name":   "auth",
							"origin": "PROVISIONED",
							"field":  "authentication",
							"resourceType": map[string]any{
								"id":   "aws-iam-role",
								"name": "AWS IAM Role",
							},
						},
					},
					{
						"resource": map[string]any{
							"id":     "res-endpoint",
							"name":   "endpoint",
							"origin": "PROVISIONED",
							"field":  "endpoint",
							"resourceType": map[string]any{
								"id":   "aws-rds-postgres",
								"name": "AWS RDS Postgres",
							},
						},
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "ecomm-prod-database")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "ecomm-prod-database" {
		t.Errorf("ID = %q, want ecomm-prod-database", got.ID)
	}
	if got.Status != "PROVISIONED" {
		t.Errorf("Status = %q, want PROVISIONED", got.Status)
	}
	if got.Params["size"] != "small" {
		t.Errorf("Params[size] = %v, want small", got.Params["size"])
	}
	if len(got.StatePaths) != 1 || got.StatePaths[0].StepName != "core" {
		t.Errorf("StatePaths = %+v, want one step named core", got.StatePaths)
	}
	if got.Environment == nil || got.Environment.ID != "ecomm-prod" {
		t.Errorf("Environment = %+v, want ID ecomm-prod", got.Environment)
	}
	if got.Bundle == nil || got.Bundle.Version != "1.2.3" {
		t.Errorf("Bundle = %+v, want version 1.2.3", got.Bundle)
	}
	if got.Component == nil || got.Component.ID != "ecomm-database" {
		t.Errorf("Component = %+v, want ID ecomm-database", got.Component)
	}

	// Resources should be unwrapped from the InstanceResource[] wire shape
	// to a flat []Resource — caller doesn't see the wrapper.
	if len(got.Resources) != 2 {
		t.Fatalf("Resources len = %d, want 2", len(got.Resources))
	}
	if got.Resources[0].ID != "res-auth" || got.Resources[0].Field != "authentication" {
		t.Errorf("Resources[0] = %+v, want id=res-auth field=authentication", got.Resources[0])
	}
	if got.Resources[0].ResourceType == nil || got.Resources[0].ResourceType.ID != "aws-iam-role" {
		t.Errorf("Resources[0].ResourceType = %+v, want ID aws-iam-role", got.Resources[0].ResourceType)
	}

	// Compile-time checks: embedded refs are the canonical types.* types.
	var _ *types.Environment = got.Environment
	var _ *types.Bundle = got.Bundle
	var _ *types.Component = got.Component
	var _ types.Resource = got.Resources[0]
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing instance.
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"instance": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestList_FilterAndPaginate(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"instances": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{
				{"id": "ecomm-prod-database", "name": "Primary Database", "status": "PROVISIONED"},
			},
		},
	})
	page2 := gqltest.RespondWithData(map[string]any{
		"instances": map[string]any{
			"cursor": map[string]any{},
			"items": []map[string]any{
				{"id": "ecomm-prod-app", "name": "App", "status": "PROVISIONED"},
			},
		},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	got, err := newService(gqlClient).List(t.Context(), instances.ListInput{
		EnvironmentID: "ecomm-prod",
		Status:        instances.StatusProvisioned,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d instances, want 2", len(got))
	}

	reqs := gqlClient.Requests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(reqs))
	}

	// Filter shape: environmentId.eq + status.eq.
	filter, ok := reqs[0].Variables["filter"].(map[string]any)
	if !ok {
		t.Fatalf("filter = %v, want map", reqs[0].Variables["filter"])
	}
	envID, _ := filter["environmentId"].(map[string]any)
	if envID["eq"] != "ecomm-prod" {
		t.Errorf("filter.environmentId.eq = %v, want ecomm-prod", envID["eq"])
	}
	status, _ := filter["status"].(map[string]any)
	if status["eq"] != "PROVISIONED" {
		t.Errorf("filter.status.eq = %v, want PROVISIONED", status["eq"])
	}

	// Page 2 must thread the cursor.
	cursor, _ := reqs[1].Variables["cursor"].(map[string]any)
	if cursor["next"] != "page-2" {
		t.Errorf("page 2 cursor.next = %v, want page-2", cursor["next"])
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateInstance": map[string]any{
				"result": map[string]any{
					"id":              "ecomm-prod-database",
					"name":            "Primary Database",
					"status":          "PROVISIONED",
					"version":         "~2.0",
					"resolvedVersion": "2.1.0",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), "ecomm-prod-database", instances.UpdateInput{
		Version: "~2.0",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Version != "~2.0" {
		t.Errorf("Version = %q, want ~2.0", got.Version)
	}
	if got.ResolvedVersion != "2.1.0" {
		t.Errorf("ResolvedVersion = %q, want 2.1.0", got.ResolvedVersion)
	}
}

func TestOrphan(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"orphanInstance": map[string]any{
				"result": map[string]any{
					"id":     "ecomm-prod-database",
					"name":   "Primary Database",
					"status": "INITIALIZED",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Orphan(t.Context(), "ecomm-prod-database", instances.OrphanInput{
		DeleteState: true,
	})
	if err != nil {
		t.Fatalf("Orphan: %v", err)
	}
	if got.Status != "INITIALIZED" {
		t.Errorf("Status = %q, want INITIALIZED", got.Status)
	}

	reqs := gqlClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].Variables["id"] != "ecomm-prod-database" {
		t.Errorf("id variable = %v, want ecomm-prod-database", reqs[0].Variables["id"])
	}
	input, ok := reqs[0].Variables["input"].(map[string]any)
	if !ok {
		t.Fatalf("input variable missing or wrong type: %v", reqs[0].Variables["input"])
	}
	if input["deleteState"] != true {
		t.Errorf("input.deleteState = %v, want true", input["deleteState"])
	}
}

func TestOrphan_MutationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"orphanInstance": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "conflict", "field": "id", "message": "instance has no recoverable state"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).Orphan(t.Context(), "ecomm-prod-database", instances.OrphanInput{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if mf.Op != "orphan instance" {
		t.Errorf("Op = %q, want orphan instance", mf.Op)
	}
	if len(mf.Messages) != 1 || mf.Messages[0].Field != "id" {
		t.Errorf("messages = %+v, want one message for field=id", mf.Messages)
	}
}

// TestIter_StopsEarly confirms that breaking out of the range loop
// stops further page requests — the iterator is lazy.
func TestIter_StopsEarly(t *testing.T) {
	page1 := gqltest.RespondWithData(map[string]any{
		"instances": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{
				{"id": "ecomm-prod-database", "name": "Primary Database", "status": "PROVISIONED"},
				{"id": "ecomm-prod-app", "name": "App", "status": "PROVISIONED"},
			},
		},
	})
	// page2 would only be requested if the iterator follows the cursor.
	page2 := gqltest.RespondWithData(map[string]any{
		"instances": map[string]any{"cursor": map[string]any{}, "items": []map[string]any{}},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	count := 0
	for inst, err := range newService(gqlClient).Iter(t.Context(), instances.ListInput{}) {
		if err != nil {
			t.Fatalf("iter err: %v", err)
		}
		count++
		if inst.ID == "ecomm-prod-database" {
			break // caller bails after the first instance
		}
	}

	if count != 1 {
		t.Errorf("iterated %d items, want 1 (caller bailed after first)", count)
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
	for _, err := range newService(gqlClient).Iter(t.Context(), instances.ListInput{}) {
		observed = append(observed, err)
	}
	if len(observed) != 1 {
		t.Fatalf("got %d yields, want 1", len(observed))
	}
	if !errors.Is(observed[0], wantErr) {
		t.Errorf("err = %v, want it to wrap %v", observed[0], wantErr)
	}
}

func TestCopy(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"copyInstance": map[string]any{
				"result": map[string]any{
					"id":              "ecomm-staging-db",
					"name":            "Staging DB",
					"status":          "PROVISIONED",
					"version":         "~1.0",
					"resolvedVersion": "1.2.3",
					"params":          map[string]any{"size": "small"},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Copy(t.Context(), "ecomm-prod-db", "ecomm-staging-db", instances.CopyInput{
		Overrides:   map[string]any{"size": "small"},
		CopySecrets: true,
	})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got.ID != "ecomm-staging-db" {
		t.Errorf("ID = %q, want ecomm-staging-db", got.ID)
	}
	if got.Params["size"] != "small" {
		t.Errorf("Params[size] = %v, want small", got.Params["size"])
	}
}
