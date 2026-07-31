package viewer_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/viewer"
)

func newService(gqlClient *gqltest.Client, orgID string) *viewer.Service {
	return viewer.New(&client.Client{Config: config.Config{OrganizationID: orgID}, GQLv2: gqlClient})
}

func TestGet_Account(t *testing.T) {
	// An account viewer's org comes from the client's configured
	// organization id, resolved with a follow-up lookup.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"viewer": map[string]any{
				"__typename": "AccountViewer",
				"id":         "user-123",
				"email":      "alice@example.com",
				"firstName":  "Alice",
				"lastName":   "Anderson",
			},
		}),
		gqltest.RespondWithData(map[string]any{
			"organization": map[string]any{
				"id":   "ecomm",
				"name": "E-Commerce",
			},
		}),
	)

	got, err := newService(gqlClient, "ecomm").Get(t.Context())
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
	reqs := gqlClient.Requests()
	if len(reqs) != 2 || reqs[1].OpName != "GetOrganization" {
		t.Errorf("requests = %+v, want GetViewer then GetOrganization", reqs)
	}
	if id := reqs[1].Variables["organizationId"]; id != "ecomm" {
		t.Errorf("GetOrganization organizationId = %v, want ecomm", id)
	}
	// The flattened type re-uses types.Organization across viewer kinds.
	var _ *types.Organization = got.Organization
}

func TestGet_Account_NoOrganizationConfigured(t *testing.T) {
	// Without a configured organization id there is nothing to resolve:
	// Organization stays nil and no lookup request is made.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"viewer": map[string]any{
				"__typename": "AccountViewer",
				"id":         "user-123",
				"email":      "newuser@example.com",
			},
		}),
	)

	got, err := newService(gqlClient, "").Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Organization != nil {
		t.Errorf("Organization = %+v, want nil when no org id is configured", got.Organization)
	}
	if n := len(gqlClient.Requests()); n != 1 {
		t.Errorf("issued %d requests, want 1 (no org lookup)", n)
	}
}

func TestGet_Account_ConfiguredOrgInaccessible(t *testing.T) {
	// A configured org the credentials can't see resolves to null — Get
	// surfaces the mismatch instead of silently reporting no org.
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"viewer": map[string]any{
				"__typename": "AccountViewer",
				"id":         "user-123",
				"email":      "alice@example.com",
			},
		}),
		gqltest.RespondWithData(map[string]any{
			"organization": nil,
		}),
	)

	_, err := newService(gqlClient, "other-org").Get(t.Context())
	if !errors.Is(err, gql.ErrNotFound) {
		t.Fatalf("Get error = %v, want gql.ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "other-org") {
		t.Errorf("Get error = %q, want it to name the configured org", err)
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

	got, err := newService(gqlClient, "ecomm").Get(t.Context())
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
	// Service accounts carry their org — no follow-up lookup.
	if n := len(gqlClient.Requests()); n != 1 {
		t.Errorf("issued %d requests, want 1 (no org lookup)", n)
	}
}
