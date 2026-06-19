package viewer_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/viewer"
)

func newService(gqlClient *gqltest.Client) *viewer.Service {
	return viewer.New(&client.Client{Config: config.Config{}, GQLv2: gqlClient})
}

func TestGet_Account(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"viewer": map[string]any{
				"__typename": "AccountViewer",
				"id":         "user-123",
				"email":      "alice@example.com",
				"firstName":  "Alice",
				"lastName":   "Anderson",
				"defaultOrganization": map[string]any{
					"id":   "ecomm",
					"name": "E-Commerce",
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Kind != viewer.KindAccount {
		t.Errorf("Kind = %q, want %q", got.Kind, viewer.KindAccount)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", got.Email)
	}
	if got.FirstName != "Alice" || got.LastName != "Anderson" {
		t.Errorf("Name = (%q, %q), want (Alice, Anderson)", got.FirstName, got.LastName)
	}
	if got.Organization == nil || got.Organization.ID != "ecomm" {
		t.Errorf("Organization = %+v, want ID ecomm", got.Organization)
	}
	// The flattened type re-uses types.Organization across viewer kinds.
	var _ *types.Organization = got.Organization
}

func TestGet_Account_NoDefaultOrganization(t *testing.T) {
	// Users with no organization memberships have a null defaultOrganization.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"viewer": map[string]any{
				"__typename":          "AccountViewer",
				"id":                  "user-123",
				"email":               "newuser@example.com",
				"defaultOrganization": nil,
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Organization != nil {
		t.Errorf("Organization = %+v, want nil for user with no orgs", got.Organization)
	}
}

func TestGet_ServiceAccount(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"viewer": map[string]any{
				"__typename":  "ServiceAccountViewer",
				"id":          "sa-deploy-bot",
				"name":        "deploy-bot",
				"description": "GitHub Actions deployer",
				"organization": map[string]any{
					"id":   "ecomm",
					"name": "E-Commerce",
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Kind != viewer.KindServiceAccount {
		t.Errorf("Kind = %q, want %q", got.Kind, viewer.KindServiceAccount)
	}
	if got.Name != "deploy-bot" {
		t.Errorf("Name = %q, want deploy-bot", got.Name)
	}
	if got.Email != "" {
		t.Errorf("Email = %q, want empty for service account", got.Email)
	}
	if got.Organization == nil || got.Organization.ID != "ecomm" {
		t.Errorf("Organization = %+v, want ID ecomm", got.Organization)
	}
}
