//go:build integration

package eventdestinations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/eventdestinations"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func TestIntegration_EventDestinations_Catalog(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	eventTypes, err := c.EventDestinations.ListEventTypes(ctx, eventdestinations.ListEventTypesInput{})
	if err != nil {
		t.Fatalf("ListEventTypes: %v", err)
	}
	if len(eventTypes) == 0 {
		t.Error("ListEventTypes is empty; want the event catalog")
	}

	adapters, err := c.EventDestinations.ListAdapters(ctx)
	if err != nil {
		t.Fatalf("ListAdapters: %v", err)
	}
	foundHTTP := false
	for _, a := range adapters {
		if a.ID == eventdestinations.AdapterHTTP {
			foundHTTP = true
		}
	}
	if !foundHTTP {
		t.Errorf("ListAdapters = %+v, want it to include HTTP", adapters)
	}

	if _, err := c.EventDestinations.PreviewPage(ctx, eventdestinations.PreviewInput{
		EventTypes: []string{"deployment.*"},
		PageSize:   5,
	}); err != nil {
		t.Fatalf("PreviewPage: %v", err)
	}
}

// TestIntegration_EventDestinations_CRUD walks Create → Get → Update →
// Pause → Resume → deliveries → Delete against a live API. The URL's host
// does not resolve, so nothing is ever sent.
func TestIntegration_EventDestinations_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "event-destination")

	created, err := c.EventDestinations.Create(ctx, eventdestinations.CreateInput{
		Name:       name,
		Adapter:    eventdestinations.AdapterHTTP,
		Config:     map[string]any{"url": "https://sdk-integration-test.invalid/massdriver"},
		Auth:       map[string]any{"headers": map[string]any{"Authorization": "Bearer test"}},
		EventTypes: []string{"deployment.*"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.EventDestinations.Delete(ctx, created.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", created.ID, err)
		}
	})
	if created.SigningSecret == "" {
		t.Error("Create returned no signing secret for an HTTP destination")
	}

	got, err := c.EventDestinations.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != name || got.Status != string(eventdestinations.StatusActive) {
		t.Errorf("Get = %+v, want %s ACTIVE", got, name)
	}

	updated, err := c.EventDestinations.Update(ctx, created.ID, eventdestinations.UpdateInput{
		EventTypes: &[]string{},
		Condition:  types.Ptr(`.type == "deployment.completed"`),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated.EventTypes) != 0 || updated.Condition != `.type == "deployment.completed"` {
		t.Errorf("Update = %+v, want every event type and the new condition", updated)
	}
	if updated.Name != name {
		t.Errorf("Update name = %q, want unchanged %q", updated.Name, name)
	}

	paused, err := c.EventDestinations.Pause(ctx, created.ID)
	if err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if paused.Status != string(eventdestinations.StatusPaused) {
		t.Errorf("Pause status = %q, want PAUSED", paused.Status)
	}
	resumed, err := c.EventDestinations.Resume(ctx, created.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if resumed.Status != string(eventdestinations.StatusActive) {
		t.Errorf("Resume status = %q, want ACTIVE", resumed.Status)
	}

	if _, err := c.EventDestinations.ListDeliveriesPage(ctx, created.ID, eventdestinations.ListDeliveriesInput{}); err != nil {
		t.Fatalf("ListDeliveriesPage: %v", err)
	}
	if _, err := c.EventDestinations.Redeliver(ctx, created.ID, nil); err != nil {
		t.Fatalf("Redeliver: %v", err)
	}

	if _, err := c.EventDestinations.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := c.EventDestinations.Get(ctx, created.ID); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Delete err = %v, want gql.ErrNotFound", err)
	}
}
