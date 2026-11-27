package stash_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/stash"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func TestWidgetCatalog(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"alphaWidgetCatalog": []map[string]any{{
			"name":        "instance.usage_bar",
			"description": "A bar on an instance that shows how much of a limit is used.",
			"entity":      "INSTANCE",
			"version":     1,
			"key":         "md/v1/usage",
			"scope":       "INSTANCE",
			"format":      "JSON",
			"schema":      map[string]any{"type": "object", "required": []string{"value", "max"}},
			"inputs": []map[string]any{
				{"name": "value", "type": "NUMBER", "required": true, "description": "The amount used.", "field": "value"},
				{"name": "label", "type": "STRING", "required": false, "description": "Text shown with the bar.", "field": "label"},
			},
		}},
	}))

	got, err := newService(gqlClient).WidgetCatalog(t.Context())
	if err != nil {
		t.Fatalf("WidgetCatalog: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d widgets, want 1", len(got))
	}
	w := got[0]
	if w.Name != "instance.usage_bar" || w.Entity != "INSTANCE" || w.Version != 1 || w.Key != stash.KeyUsageV1 || w.Format != "JSON" {
		t.Errorf("widget = %+v", w)
	}
	if w.Scope != stash.ScopeInstance {
		t.Errorf("Scope = %q, want instance", w.Scope)
	}
	if w.Schema["type"] != "object" {
		t.Errorf("Schema = %v", w.Schema)
	}
	if len(w.Inputs) != 2 || w.Inputs[0].Name != "value" || w.Inputs[0].Type != "NUMBER" || !w.Inputs[0].Required || w.Inputs[1].Field != "label" {
		t.Errorf("Inputs = %+v", w.Inputs)
	}
	if _, present := gqlClient.Requests()[0].Variables["organizationId"]; present {
		t.Error("the catalog query takes no organizationId")
	}
}

func TestWidgetsForScopes(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"alphaWidgetsForScopes": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items": []map[string]any{{
				"name":      "instance.usage_bar",
				"scope":     map[string]any{"kind": "INSTANCE", "id": "api-prod-db"},
				"values":    map[string]any{"value": 42, "max": 100},
				"updatedAt": "2026-11-20T10:00:00Z",
				"expiresAt": nil,
			}},
		},
	}))

	page, err := newService(gqlClient).WidgetsForScopes(t.Context(), []stash.Scope{
		{Kind: stash.ScopeEnvironment, ID: "api-prod"},
		{Kind: stash.ScopeInstance, ID: "api-prod-db"},
	}, stash.WidgetsInput{PageSize: 20})
	if err != nil {
		t.Fatalf("WidgetsForScopes: %v", err)
	}
	if len(page.Items) != 1 || page.Next != "page-2" {
		t.Fatalf("page = %+v", page)
	}
	w := page.Items[0]
	if w.Name != "instance.usage_bar" || w.Scope != (stash.Scope{Kind: stash.ScopeInstance, ID: "api-prod-db"}) {
		t.Errorf("widget = %+v", w)
	}
	if w.Values["value"] != float64(42) || w.Values["max"] != float64(100) {
		t.Errorf("Values = %v", w.Values)
	}
	if w.UpdatedAt.IsZero() || !w.ExpiresAt.IsZero() {
		t.Errorf("UpdatedAt = %v, ExpiresAt = %v", w.UpdatedAt, w.ExpiresAt)
	}

	scopes, _ := gqlClient.Requests()[0].Variables["scopes"].([]any)
	if len(scopes) != 2 {
		t.Errorf("scopes = %v", scopes)
	}
}

func TestIterWidgetsForScopes_EmptyPageFollowsCursor(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"alphaWidgetsForScopes": map[string]any{
				"cursor": map[string]any{"next": "page-2"},
				"items":  []map[string]any{},
			},
		}),
		gqltest.RespondWithData(map[string]any{
			"alphaWidgetsForScopes": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{{
					"name":      "environment.banner",
					"scope":     map[string]any{"kind": "ENVIRONMENT", "id": "api-prod"},
					"values":    map[string]any{"message": "Freeze until Monday"},
					"updatedAt": "2026-11-20T10:00:00Z",
				}},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).IterWidgetsForScopes(t.Context(), []stash.Scope{{Kind: stash.ScopeEnvironment, ID: "api-prod"}}, stash.WidgetsInput{}))
	if err != nil {
		t.Fatalf("IterWidgetsForScopes: %v", err)
	}
	if len(got) != 1 || got[0].Name != "environment.banner" {
		t.Errorf("got %+v", got)
	}
}
