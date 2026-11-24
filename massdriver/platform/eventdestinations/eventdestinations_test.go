package eventdestinations_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/eventdestinations"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *eventdestinations.Service {
	return eventdestinations.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func destination() map[string]any {
	return map[string]any{
		"id":            "4f9b2c1e-0000-0000-0000-000000000001",
		"name":          "Production deployments",
		"adapter":       "HTTP",
		"config":        map[string]any{"url": "https://hooks.example.com/massdriver"},
		"eventTypes":    []string{"deployment.*"},
		"attributes":    map[string]any{"md-environment": "prod"},
		"condition":     nil,
		"status":        "PAUSED",
		"signingSecret": "whsec_c2VjcmV0",
		"failingSince":  "2026-11-20T10:00:00Z",
		"lastError":     "HTTP 503",
		"createdAt":     "2026-11-01T00:00:00Z",
		"updatedAt":     "2026-11-20T10:00:00Z",
	}
}

func payload(result map[string]any) map[string]any {
	return map[string]any{"result": result, "successful": true}
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"eventDestination": destination()}))

	got, err := newService(gqlClient).Get(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Production deployments" || got.Adapter != "HTTP" || got.Status != "PAUSED" {
		t.Errorf("got %+v", got)
	}
	if got.Config["url"] != "https://hooks.example.com/massdriver" {
		t.Errorf("Config = %v", got.Config)
	}
	if got.Attributes["md-environment"] != "prod" {
		t.Errorf("Attributes = %v", got.Attributes)
	}
	if len(got.EventTypes) != 1 || got.EventTypes[0] != "deployment.*" {
		t.Errorf("EventTypes = %v", got.EventTypes)
	}
	if got.SigningSecret != "whsec_c2VjcmV0" {
		t.Errorf("SigningSecret = %q", got.SigningSecret)
	}
	if got.FailingSince.IsZero() || got.LastError != "HTTP 503" {
		t.Errorf("FailingSince = %v, LastError = %q", got.FailingSince, got.LastError)
	}
	if !got.LastDeliveredAt.IsZero() {
		t.Errorf("LastDeliveredAt = %v, want zero", got.LastDeliveredAt)
	}
}

func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"eventDestination": nil}))

	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestListPage_StatusFilter(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"eventDestinations": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items":  []map[string]any{destination()},
		},
	}))

	page, err := newService(gqlClient).ListPage(t.Context(), eventdestinations.ListInput{
		Statuses: []eventdestinations.Status{eventdestinations.StatusPaused},
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Production deployments" || page.Next != "page-2" {
		t.Errorf("page = %+v", page)
	}

	vars := gqlClient.Requests()[0].Variables
	filter, _ := vars["filter"].(map[string]any)
	status, _ := filter["status"].(map[string]any)
	in, _ := status["in"].([]any)
	if len(in) != 1 || in[0] != "PAUSED" {
		t.Errorf("filter = %v, want status in [PAUSED]", vars["filter"])
	}
	if _, present := vars["sort"]; present {
		t.Errorf("sort = %v, want it omitted", vars["sort"])
	}
}

func TestList_NoFilter(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"eventDestinations": map[string]any{
			"cursor": map[string]any{},
			"items":  []map[string]any{destination(), destination()},
		},
	}))

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), eventdestinations.ListInput{}))
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d, want 2", len(got))
	}
	if _, present := gqlClient.Requests()[0].Variables["filter"]; present {
		t.Error("filter should be omitted when no status is given")
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"createEventDestination": payload(destination()),
	}))

	got, err := newService(gqlClient).Create(t.Context(), eventdestinations.CreateInput{
		Name:       "Production deployments",
		Adapter:    eventdestinations.AdapterHTTP,
		Config:     map[string]any{"url": "https://hooks.example.com/massdriver"},
		Auth:       map[string]any{"headers": map[string]any{"Authorization": "Bearer token"}},
		EventTypes: []string{"deployment.*"},
		Attributes: map[string]string{"md-environment": "prod"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.SigningSecret != "whsec_c2VjcmV0" {
		t.Errorf("SigningSecret = %q", got.SigningSecret)
	}

	input, _ := gqlClient.Requests()[0].Variables["input"].(map[string]any)
	want := map[string]any{
		"name":       "Production deployments",
		"adapter":    "HTTP",
		"config":     `{"url":"https://hooks.example.com/massdriver"}`,
		"auth":       `{"headers":{"Authorization":"Bearer token"}}`,
		"attributes": `{"md-environment":"prod"}`,
	}
	for k, v := range want {
		if input[k] != v {
			t.Errorf("input.%s = %v, want %v", k, input[k], v)
		}
	}
	if eventTypes, _ := input["eventTypes"].([]any); len(eventTypes) != 1 || eventTypes[0] != "deployment.*" {
		t.Errorf("input.eventTypes = %v", input["eventTypes"])
	}
	if _, present := input["condition"]; present {
		t.Errorf("input.condition = %v, want it omitted", input["condition"])
	}
}

func TestCreate_MutationFailed(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"createEventDestination": map[string]any{
			"result":     nil,
			"successful": false,
			"messages":   []map[string]any{{"field": "name", "message": "has already been taken"}},
		},
	}))

	_, err := newService(gqlClient).Create(t.Context(), eventdestinations.CreateInput{
		Name:    "Production deployments",
		Adapter: eventdestinations.AdapterHTTP,
		Config:  map[string]any{"url": "https://hooks.example.com/massdriver"},
	})
	var mutErr *gql.MutationFailedError
	if !errors.As(err, &mutErr) {
		t.Fatalf("err = %v, want *gql.MutationFailedError", err)
	}
}

func TestUpdate_OmitsUnsetFields(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"updateEventDestination": payload(destination()),
	}))

	_, err := newService(gqlClient).Update(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001", eventdestinations.UpdateInput{
		Name: types.Ptr("Renamed"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	input, _ := gqlClient.Requests()[0].Variables["input"].(map[string]any)
	if len(input) != 1 || input["name"] != "Renamed" {
		t.Errorf("input = %v, want only name", input)
	}
}

func TestUpdate_EmptyValuesClear(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"updateEventDestination": payload(destination()),
	}))

	_, err := newService(gqlClient).Update(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001", eventdestinations.UpdateInput{
		Auth:       map[string]any{},
		EventTypes: &[]string{},
		Attributes: map[string]string{},
		Condition:  types.Ptr(""),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	input, _ := gqlClient.Requests()[0].Variables["input"].(map[string]any)
	if input["auth"] != "{}" {
		t.Errorf("input.auth = %v, want {}", input["auth"])
	}
	if input["attributes"] != "{}" {
		t.Errorf("input.attributes = %v, want {}", input["attributes"])
	}
	if eventTypes, ok := input["eventTypes"].([]any); !ok || len(eventTypes) != 0 {
		t.Errorf("input.eventTypes = %v, want []", input["eventTypes"])
	}
	if condition, present := input["condition"]; !present || condition != "" {
		t.Errorf("input.condition = %v, want empty string", input["condition"])
	}
}

func TestPauseAndResume(t *testing.T) {
	active := destination()
	active["status"] = "ACTIVE"
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{"pauseEventDestination": payload(destination())}),
		gqltest.RespondWithData(map[string]any{"resumeEventDestination": payload(active)}),
	)
	svc := newService(gqlClient)

	paused, err := svc.Pause(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if paused.Status != string(eventdestinations.StatusPaused) {
		t.Errorf("Pause status = %q", paused.Status)
	}

	resumed, err := svc.Resume(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if resumed.Status != string(eventdestinations.StatusActive) {
		t.Errorf("Resume status = %q", resumed.Status)
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"deleteEventDestination": payload(map[string]any{"id": "4f9b2c1e-0000-0000-0000-000000000001", "name": "Production deployments"}),
	}))

	got, err := newService(gqlClient).Delete(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got.Name != "Production deployments" {
		t.Errorf("Name = %q", got.Name)
	}
}

func TestRedeliver(t *testing.T) {
	result := map[string]any{"id": "4f9b2c1e-0000-0000-0000-000000000001", "name": "Production deployments"}
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{"redeliverEvents": payload(result)}),
		gqltest.RespondWithData(map[string]any{"redeliverEvents": payload(result)}),
	)
	svc := newService(gqlClient)

	if _, err := svc.Redeliver(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001", nil); err != nil {
		t.Fatalf("Redeliver all: %v", err)
	}
	if _, err := svc.Redeliver(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001", []string{"d-1"}); err != nil {
		t.Fatalf("Redeliver one: %v", err)
	}

	reqs := gqlClient.Requests()
	if _, present := reqs[0].Variables["deliveryIds"]; present {
		t.Errorf("deliveryIds = %v, want it omitted", reqs[0].Variables["deliveryIds"])
	}
	if ids, _ := reqs[1].Variables["deliveryIds"].([]any); len(ids) != 1 || ids[0] != "d-1" {
		t.Errorf("deliveryIds = %v, want [d-1]", reqs[1].Variables["deliveryIds"])
	}
}

func TestListDeliveriesPage(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"eventDestination": map[string]any{
			"id": "4f9b2c1e-0000-0000-0000-000000000001",
			"deliveries": map[string]any{
				"cursor": map[string]any{"next": "page-2"},
				"items": []map[string]any{
					{
						"id":            "d-1",
						"status":        "FAILED",
						"attempts":      8,
						"nextAttemptAt": "2026-11-20T10:00:00Z",
						"lastError":     "HTTP 503",
						"createdAt":     "2026-11-19T10:00:00Z",
						"updatedAt":     "2026-11-20T10:00:00Z",
						"event": map[string]any{
							"id":         "evt-1",
							"type":       "deployment.completed",
							"occurredAt": "2026-11-19T10:00:00Z",
						},
					},
					{
						"id":            "d-2",
						"status":        "FAILED",
						"attempts":      8,
						"nextAttemptAt": "2026-11-20T10:00:00Z",
						"createdAt":     "2026-11-19T10:00:00Z",
						"updatedAt":     "2026-11-20T10:00:00Z",
						"event":         nil,
					},
				},
			},
		},
	}))

	page, err := newService(gqlClient).ListDeliveriesPage(t.Context(), "4f9b2c1e-0000-0000-0000-000000000001", eventdestinations.ListDeliveriesInput{
		Statuses: []eventdestinations.DeliveryStatus{eventdestinations.DeliveryFailed},
	})
	if err != nil {
		t.Fatalf("ListDeliveriesPage: %v", err)
	}
	if len(page.Items) != 2 || page.Next != "page-2" {
		t.Fatalf("page = %+v", page)
	}
	first := page.Items[0]
	if first.Status != "FAILED" || first.Attempts != 8 || first.Event == nil || first.Event.Type != "deployment.completed" {
		t.Errorf("first = %+v", first)
	}
	if page.Items[1].Event != nil {
		t.Errorf("second.Event = %+v, want nil", page.Items[1].Event)
	}

	filter, _ := gqlClient.Requests()[0].Variables["filter"].(map[string]any)
	status, _ := filter["status"].(map[string]any)
	if in, _ := status["in"].([]any); len(in) != 1 || in[0] != "FAILED" {
		t.Errorf("filter = %v, want status in [FAILED]", filter)
	}
}

func TestListDeliveriesPage_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"eventDestination": nil}))

	_, err := newService(gqlClient).ListDeliveriesPage(t.Context(), "missing", eventdestinations.ListDeliveriesInput{})
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestPreviewPage(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"previewEventDestination": map[string]any{
			"cursor": map[string]any{},
			"items": []map[string]any{
				{"id": "evt-1", "type": "deployment.completed", "occurredAt": "2026-11-19T10:00:00Z"},
			},
		},
	}))

	page, err := newService(gqlClient).PreviewPage(t.Context(), eventdestinations.PreviewInput{
		EventTypes: []string{"deployment.*"},
		Attributes: map[string]string{"md-environment": "prod"},
		Condition:  `.data.metadata.deployment_action == "PROVISION"`,
	})
	if err != nil {
		t.Fatalf("PreviewPage: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Type != "deployment.completed" {
		t.Errorf("page = %+v", page)
	}

	vars := gqlClient.Requests()[0].Variables
	if vars["attributes"] != `{"md-environment":"prod"}` {
		t.Errorf("attributes = %v", vars["attributes"])
	}
	if vars["condition"] != `.data.metadata.deployment_action == "PROVISION"` {
		t.Errorf("condition = %v", vars["condition"])
	}
	if eventTypes, _ := vars["eventTypes"].([]any); len(eventTypes) != 1 || eventTypes[0] != "deployment.*" {
		t.Errorf("eventTypes = %v", vars["eventTypes"])
	}
}

func TestPreviewPage_EmptySelection(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"previewEventDestination": map[string]any{"cursor": map[string]any{}, "items": []map[string]any{}},
	}))

	if _, err := newService(gqlClient).PreviewPage(t.Context(), eventdestinations.PreviewInput{}); err != nil {
		t.Fatalf("PreviewPage: %v", err)
	}

	vars := gqlClient.Requests()[0].Variables
	for _, name := range []string{"eventTypes", "attributes", "condition"} {
		if v, present := vars[name]; present {
			t.Errorf("%s = %v, want it omitted", name, v)
		}
	}
}

func TestListAdapters(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"eventDestinationAdapters": map[string]any{
			"cursor": map[string]any{},
			"items": []map[string]any{
				{
					"id":           "HTTP",
					"name":         "HTTP",
					"description":  "POST each event to a URL.",
					"configSchema": map[string]any{"required": []string{"url"}},
					"authSchema":   map[string]any{"type": "object"},
					"signs":        true,
				},
			},
		},
	}))

	got, err := newService(gqlClient).ListAdapters(t.Context())
	if err != nil {
		t.Fatalf("ListAdapters: %v", err)
	}
	if len(got) != 1 || got[0].ID != "HTTP" || !got[0].Signs || got[0].AuthSchema["type"] != "object" {
		t.Errorf("got %+v", got)
	}
}

func TestListEventTypes(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"eventTypes": []map[string]any{
			{
				"id":          "deployment.completed",
				"description": "A deployment finished.",
				"entity":      "deployment",
				"action":      "completed",
				"changeType":  "UPDATED",
				"metadata": []map[string]any{
					{"name": "credential", "type": "STRING", "description": "How the actor authenticated."},
				},
			},
		},
	}))

	got, err := newService(gqlClient).ListEventTypes(t.Context(), eventdestinations.ListEventTypesInput{Entity: "deployment"})
	if err != nil {
		t.Fatalf("ListEventTypes: %v", err)
	}
	if len(got) != 1 || got[0].ID != "deployment.completed" || got[0].Metadata[0].Name != "credential" {
		t.Errorf("got %+v", got)
	}

	filter, _ := gqlClient.Requests()[0].Variables["filter"].(map[string]any)
	entity, _ := filter["entity"].(map[string]any)
	if entity["eq"] != "deployment" {
		t.Errorf("filter = %v, want entity eq deployment", filter)
	}
}
