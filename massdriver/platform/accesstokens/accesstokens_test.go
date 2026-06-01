package accesstokens_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/accesstokens"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *accesstokens.Service {
	return accesstokens.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestList_FilterByActive(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"accessTokens": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "t1", "name": "ci-token", "prefix": "md_a1b2", "scopes": []string{"*"}},
				},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), accesstokens.ListInput{
		Status: accesstokens.StatusActive,
	}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}

	// Active = filter.revoked: false on the wire.
	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)
	if filter["revoked"] != false {
		t.Errorf("filter.revoked = %v, want false", filter["revoked"])
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createAccessToken": map[string]any{
				"result": map[string]any{
					"id":        "t-new",
					"name":      "deploy-token",
					"token":     "md_RAW_BEARER_VALUE_HERE",
					"prefix":    "md_a1b2c3d4",
					"scopes":    []string{"*"},
					"expiresAt": "2027-01-01T00:00:00Z",
					"createdAt": "2026-05-08T10:00:00Z",
				},
				"successful": true,
			},
		}),
	)

	created, err := newService(gqlClient).Create(t.Context(), accesstokens.CreateInput{
		Name:             "deploy-token",
		Scopes:           []string{"*"},
		ExpiresInMinutes: 30,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "t-new" {
		t.Errorf("ID = %q, want t-new", created.ID)
	}
	// The raw token surfaces here once and only here.
	if created.Token != "md_RAW_BEARER_VALUE_HERE" {
		t.Errorf("Token = %q, want raw bearer value", created.Token)
	}
	if created.Prefix != "md_a1b2c3d4" {
		t.Errorf("Prefix = %q, want md_a1b2c3d4", created.Prefix)
	}

	// expiresInMinutes was set, so it should round-trip on the wire.
	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["expiresInMinutes"] != float64(30) {
		t.Errorf("input.expiresInMinutes = %v, want 30", input["expiresInMinutes"])
	}
}

func TestCreate_OmitsExpiresWhenZero(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createAccessToken": map[string]any{
				"result":     map[string]any{"id": "t", "name": "t", "token": "tok", "prefix": "p", "scopes": []string{"*"}},
				"successful": true,
			},
		}),
	)

	_, err := newService(gqlClient).Create(t.Context(), accesstokens.CreateInput{
		Name:   "default-expiry",
		Scopes: []string{"*"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// expiresInMinutes was zero so the wrapper passed nil — the wire input
	// must not include the field (server uses its default).
	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if _, present := input["expiresInMinutes"]; present {
		t.Errorf("expiresInMinutes should be omitted when zero, got %v", input["expiresInMinutes"])
	}
}

func TestRevoke(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"revokeAccessToken": map[string]any{
				"result":     map[string]any{"id": "t-1", "name": "old-token", "prefix": "md_a1b2", "revokedAt": "2026-05-08T11:00:00Z"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Revoke(t.Context(), "t-1")
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if got.RevokedAt.IsZero() {
		t.Errorf("RevokedAt is zero; want non-zero after revoke")
	}
}
