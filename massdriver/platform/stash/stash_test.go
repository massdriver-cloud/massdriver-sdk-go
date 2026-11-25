package stash_test

import (
	"errors"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/stash"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *stash.Service {
	return stash.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func textEntry() map[string]any {
	return map[string]any{
		"key":       "db/hostname",
		"value":     "db.internal",
		"data":      nil,
		"format":    "TEXT",
		"immutable": false,
		"sensitive": false,
		"expiresAt": nil,
		"updatedAt": "2026-11-20T10:00:00Z",
	}
}

func payload(result map[string]any) map[string]any {
	return map[string]any{"result": result, "successful": true}
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"stashEntry": textEntry()}))

	got, err := newService(gqlClient).Get(t.Context(), stash.Scope{Kind: stash.ScopeEnvironment, ID: "api-prod"}, "db/hostname")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Key != "db/hostname" || got.Value == nil || *got.Value != "db.internal" || got.Format != "TEXT" {
		t.Errorf("got %+v", got)
	}
	if !got.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt = %v, want zero", got.ExpiresAt)
	}

	vars := gqlClient.Requests()[0].Variables
	if vars["organizationId"] != "my-org" || vars["key"] != "db/hostname" {
		t.Errorf("vars = %v", vars)
	}
	scope, _ := vars["scope"].(map[string]any)
	if len(scope) != 1 || scope["environment"] != "api-prod" {
		t.Errorf("scope = %v, want only environment: api-prod", vars["scope"])
	}
}

func TestGet_JSONData(t *testing.T) {
	entry := textEntry()
	entry["key"] = "stats"
	entry["value"] = `{"changed":3}`
	entry["data"] = map[string]any{"changed": 3}
	entry["format"] = "JSON"
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"stashEntry": entry}))

	got, err := newService(gqlClient).Get(t.Context(), stash.Scope{Kind: stash.ScopeDeployment, ID: "a1b2c3d4-0000-0000-0000-000000000001"}, "stats")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Data["changed"] != float64(3) {
		t.Errorf("Data = %v, want changed: 3", got.Data)
	}
}

func TestGet_SensitiveRedacted(t *testing.T) {
	entry := textEntry()
	entry["value"] = nil
	entry["sensitive"] = true
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"stashEntry": entry}))

	got, err := newService(gqlClient).Get(t.Context(), stash.Scope{Kind: stash.ScopeProject, ID: "api"}, "db/hostname")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Value != nil || !got.Sensitive {
		t.Errorf("got Value = %v, Sensitive = %v; want nil, true", got.Value, got.Sensitive)
	}
}

func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"stashEntry": nil}))

	_, err := newService(gqlClient).Get(t.Context(), stash.Scope{Kind: stash.ScopeProject, ID: "api"}, "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestGet_InvalidScope(t *testing.T) {
	tests := []struct {
		name  string
		scope stash.Scope
	}{
		{"unknown kind", stash.Scope{Kind: "organization", ID: "my-org"}},
		{"empty id", stash.Scope{Kind: stash.ScopeProject}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := gqltest.NewClient()
			if _, err := newService(gqlClient).Get(t.Context(), tc.scope, "key"); err == nil {
				t.Error("err = nil, want an error")
			}
			if n := len(gqlClient.Requests()); n != 0 {
				t.Errorf("sent %d requests, want 0", n)
			}
		})
	}
}

func TestScopeVariables(t *testing.T) {
	tests := []struct {
		kind stash.ScopeKind
		want string
	}{
		{stash.ScopeProject, "project"},
		{stash.ScopeEnvironment, "environment"},
		{stash.ScopeComponent, "component"},
		{stash.ScopeInstance, "instance"},
		{stash.ScopeDeployment, "deployment"},
	}
	for _, tc := range tests {
		t.Run(string(tc.kind), func(t *testing.T) {
			gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"stashEntry": textEntry()}))
			if _, err := newService(gqlClient).Get(t.Context(), stash.Scope{Kind: tc.kind, ID: "api-prod-db"}, "db/hostname"); err != nil {
				t.Fatalf("Get: %v", err)
			}
			scope, _ := gqlClient.Requests()[0].Variables["scope"].(map[string]any)
			if len(scope) != 1 || scope[tc.want] != "api-prod-db" {
				t.Errorf("scope = %v, want only %s: api-prod-db", scope, tc.want)
			}
		})
	}
}

func TestListPage(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"stashEntries": map[string]any{
			"cursor": map[string]any{"next": "page-2"},
			"items":  []map[string]any{textEntry()},
		},
	}))

	page, err := newService(gqlClient).ListPage(t.Context(), stash.Scope{Kind: stash.ScopeInstance, ID: "api-prod-db"}, stash.ListInput{PageSize: 10})
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Key != "db/hostname" || page.Next != "page-2" {
		t.Errorf("page = %+v", page)
	}
	scope, _ := gqlClient.Requests()[0].Variables["scope"].(map[string]any)
	if scope["instance"] != "api-prod-db" {
		t.Errorf("scope = %v", scope)
	}
}

func TestIter_FollowsCursor(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"stashEntries": map[string]any{
				"cursor": map[string]any{"next": "page-2"},
				"items":  []map[string]any{textEntry()},
			},
		}),
		gqltest.RespondWithData(map[string]any{
			"stashEntries": map[string]any{
				"cursor": map[string]any{},
				"items":  []map[string]any{textEntry()},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), stash.Scope{Kind: stash.ScopeProject, ID: "api"}, stash.ListInput{}))
	if err != nil {
		t.Fatalf("Iter: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d entries, want 2", len(got))
	}
}

func TestSet(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"setStashEntry": payload(textEntry())}))

	got, err := newService(gqlClient).Set(t.Context(), stash.Scope{Kind: stash.ScopeEnvironment, ID: "api-staging"}, "ami-id", stash.SetInput{
		Value:     "ami-0abc",
		Format:    stash.FormatText,
		TTL:       24 * time.Hour,
		Immutable: true,
		Sensitive: true,
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got.Key != "db/hostname" {
		t.Errorf("got %+v", got)
	}

	vars := gqlClient.Requests()[0].Variables
	if vars["key"] != "ami-id" {
		t.Errorf("key = %v", vars["key"])
	}
	input, _ := vars["input"].(map[string]any)
	want := map[string]any{
		"value":     "ami-0abc",
		"format":    "TEXT",
		"ttl":       float64(86400),
		"immutable": true,
		"sensitive": true,
	}
	for k, v := range want {
		if input[k] != v {
			t.Errorf("input.%s = %v, want %v", k, input[k], v)
		}
	}
}

func TestSet_OmitsUnsetOptions(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{"setStashEntry": payload(textEntry())}))

	if _, err := newService(gqlClient).Set(t.Context(), stash.Scope{Kind: stash.ScopeProject, ID: "api"}, "db/hostname", stash.SetInput{Value: "db.internal"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	input, _ := gqlClient.Requests()[0].Variables["input"].(map[string]any)
	if len(input) != 1 || input["value"] != "db.internal" {
		t.Errorf("input = %v, want only value", input)
	}
}

func TestSet_InvalidTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{"negative", -time.Second},
		{"under one second", 500 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := gqltest.NewClient()
			_, err := newService(gqlClient).Set(t.Context(), stash.Scope{Kind: stash.ScopeProject, ID: "api"}, "k", stash.SetInput{Value: "v", TTL: tc.ttl})
			if err == nil {
				t.Error("err = nil, want an error")
			}
			if n := len(gqlClient.Requests()); n != 0 {
				t.Errorf("sent %d requests, want 0", n)
			}
		})
	}
}

func TestSet_MutationFailed(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"setStashEntry": map[string]any{
			"result":     nil,
			"successful": false,
			"messages":   []map[string]any{{"field": "key", "message": "is immutable"}},
		},
	}))

	_, err := newService(gqlClient).Set(t.Context(), stash.Scope{Kind: stash.ScopeDeployment, ID: "a1b2c3d4-0000-0000-0000-000000000001"}, "migrations/ran", stash.SetInput{Value: "true", Immutable: true})
	var mutErr *gql.MutationFailedError
	if !errors.As(err, &mutErr) {
		t.Fatalf("err = %v, want *gql.MutationFailedError", err)
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithData(map[string]any{
		"deleteStashEntry": payload(map[string]any{"key": "db/hostname"}),
	}))

	if err := newService(gqlClient).Delete(t.Context(), stash.Scope{Kind: stash.ScopeComponent, ID: "api-db"}, "db/hostname"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	vars := gqlClient.Requests()[0].Variables
	scope, _ := vars["scope"].(map[string]any)
	if scope["component"] != "api-db" || vars["key"] != "db/hostname" {
		t.Errorf("vars = %v", vars)
	}
}

func TestDelete_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithJSON(map[string]any{
		"data": map[string]any{"deleteStashEntry": nil},
		"errors": []map[string]any{{
			"message":    "not found",
			"path":       []string{"deleteStashEntry"},
			"extensions": map[string]any{"code": "NOT_FOUND"},
		}},
	}))

	err := newService(gqlClient).Delete(t.Context(), stash.Scope{Kind: stash.ScopeProject, ID: "api"}, "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}
