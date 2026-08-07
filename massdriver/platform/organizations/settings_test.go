package organizations_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/organizations"
)

func TestGetSettings(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"organization": map[string]any{
				"id": "ecomm-corp",
				"settings": map[string]any{
					"defaultBundleAccess": "ALL_PROJECTS",
				},
			},
		}),
	)

	got, err := newService(gqlClient).GetSettings(t.Context())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.DefaultBundleAccess != organizations.DefaultBundleAccessAllProjects {
		t.Errorf("DefaultBundleAccess = %q, want ALL_PROJECTS", got.DefaultBundleAccess)
	}
}

func TestUpdateSettings(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateOrganizationSettings": map[string]any{
				"result": map[string]any{
					"id": "ecomm-corp",
					"settings": map[string]any{
						"defaultBundleAccess": "ALL_PROJECTS",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).UpdateSettings(t.Context(), organizations.UpdateSettingsInput{
		DefaultBundleAccess: organizations.DefaultBundleAccessAllProjects,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if got.DefaultBundleAccess != organizations.DefaultBundleAccessAllProjects {
		t.Errorf("DefaultBundleAccess = %q, want ALL_PROJECTS", got.DefaultBundleAccess)
	}

	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["defaultBundleAccess"] != "ALL_PROJECTS" {
		t.Errorf("input.defaultBundleAccess = %v, want ALL_PROJECTS", input["defaultBundleAccess"])
	}
}

func TestUpdateSettings_OmitsUnsetFields(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateOrganizationSettings": map[string]any{
				"result": map[string]any{
					"id":       "ecomm-corp",
					"settings": map[string]any{"defaultBundleAccess": "NONE"},
				},
				"successful": true,
			},
		}),
	)

	_, err := newService(gqlClient).UpdateSettings(t.Context(), organizations.UpdateSettingsInput{})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	// The API treats an absent field as "leave unchanged", so an unset
	// setting must stay off the wire entirely.
	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if _, present := input["defaultBundleAccess"]; present {
		t.Errorf("defaultBundleAccess should be omitted when unset, got %v", input["defaultBundleAccess"])
	}
}
