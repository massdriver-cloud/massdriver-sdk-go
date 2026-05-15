package serviceaccounts_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/serviceaccounts"
)

// newService builds a *serviceaccounts.Service backed by the provided gqltest
// mock, preconfigured with an organization ID so the wrapper has something to
// substitute into request variables.
func newService(gqlClient *gqltest.Client) *serviceaccounts.Service {
	return serviceaccounts.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"serviceAccount": map[string]any{
				"id":          "sa-1",
				"name":        "ci-bot",
				"description": "GitHub Actions deployer",
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "sa-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "ci-bot" {
		t.Errorf("Name = %q, want ci-bot", got.Name)
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing service account (the schema's
// `serviceAccount` field is nullable, so a 404 manifests as a zero-valued
// struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"serviceAccount": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestList_Search(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"serviceAccounts": map[string]any{
				"cursor": map[string]any{},
				"items":  []map[string]any{{"id": "sa-1", "name": "deploy-bot"}},
			},
		}),
	)

	_, err := newService(gqlClient).List(t.Context(), serviceaccounts.ListInput{
		Search: "deploy",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)
	if filter["search"] != "deploy" {
		t.Errorf("filter.search = %v, want deploy", filter["search"])
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createServiceAccount": map[string]any{
				"result": map[string]any{
					"id":          "sa-new",
					"name":        "ci-bot",
					"description": "GitHub Actions deployer",
					"defaultAccessToken": map[string]any{
						"id":     "tok-new",
						"name":   "default",
						"token":  "md_RAW_DEFAULT_TOKEN",
						"prefix": "md_d3f4",
						"scopes": []string{"*"},
					},
				},
				"successful": true,
			},
		}),
	)

	created, err := newService(gqlClient).Create(t.Context(), serviceaccounts.CreateInput{
		Name:                                  "ci-bot",
		Description:                           "GitHub Actions deployer",
		DefaultAccessTokenExpirationInMinutes: 525600,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "sa-new" {
		t.Errorf("ID = %q, want sa-new", created.ID)
	}
	if created.DefaultToken != "md_RAW_DEFAULT_TOKEN" {
		t.Errorf("DefaultToken = %q, want md_RAW_DEFAULT_TOKEN", created.DefaultToken)
	}
	if created.DefaultTokenID != "tok-new" {
		t.Errorf("DefaultTokenID = %q, want tok-new", created.DefaultTokenID)
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateServiceAccount": map[string]any{
				"result":     map[string]any{"id": "sa-1", "name": "ci-bot-renamed"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), "sa-1", serviceaccounts.UpdateInput{
		Name: "ci-bot-renamed",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "ci-bot-renamed" {
		t.Errorf("Name = %q, want ci-bot-renamed", got.Name)
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteServiceAccount": map[string]any{
				"result":     map[string]any{"id": "sa-1", "name": "ci-bot"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Delete(t.Context(), "sa-1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got.ID != "sa-1" {
		t.Errorf("ID = %q, want sa-1", got.ID)
	}
}
