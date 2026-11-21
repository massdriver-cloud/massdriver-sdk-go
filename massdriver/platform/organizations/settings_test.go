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
	for _, field := range []string{"defaultBundleAccess", "namingConvention", "defaultTags"} {
		if v, present := input[field]; present {
			t.Errorf("%s should be omitted when unset, got %v", field, v)
		}
	}
}

func TestGetSettings_NamingConventionAndDefaultTags(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"organization": map[string]any{
				"id": "ecomm-corp",
				"settings": map[string]any{
					"defaultBundleAccess": "NONE",
					"namingConvention":    "{{attrs.team}}-{{instance.id}}",
					"defaultTags":         []any{"team", "cost_center"},
				},
			},
		}),
	)

	got, err := newService(gqlClient).GetSettings(t.Context())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.NamingConvention != "{{attrs.team}}-{{instance.id}}" {
		t.Errorf("NamingConvention = %q", got.NamingConvention)
	}
	if len(got.DefaultTags) != 2 || got.DefaultTags[0] != "team" || got.DefaultTags[1] != "cost_center" {
		t.Errorf("DefaultTags = %v, want [team cost_center]", got.DefaultTags)
	}
}

func TestGetSettings_LegacyDefaultTagsAreNil(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"organization": map[string]any{
				"id": "ecomm-corp",
				"settings": map[string]any{
					"defaultBundleAccess": "NONE",
					"namingConvention":    nil,
					"defaultTags":         nil,
				},
			},
		}),
	)

	got, err := newService(gqlClient).GetSettings(t.Context())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.NamingConvention != "" {
		t.Errorf("NamingConvention = %q, want empty", got.NamingConvention)
	}
	if got.DefaultTags != nil {
		t.Errorf("DefaultTags = %#v, want nil", got.DefaultTags)
	}
}

func TestUpdateSettings_NamingConventionAndDefaultTags(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateOrganizationSettings": map[string]any{
				"result": map[string]any{
					"id": "ecomm-corp",
					"settings": map[string]any{
						"defaultBundleAccess": "NONE",
						"namingConvention":    "{{attrs.team}}-{{instance.id}}",
						"defaultTags":         []any{"team"},
					},
				},
				"successful": true,
			},
		}),
	)

	convention := "{{attrs.team}}-{{instance.id}}"
	tags := []string{"team"}
	got, err := newService(gqlClient).UpdateSettings(t.Context(), organizations.UpdateSettingsInput{
		NamingConvention: &convention,
		DefaultTags:      &tags,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if got.NamingConvention != convention {
		t.Errorf("NamingConvention = %q", got.NamingConvention)
	}
	if len(got.DefaultTags) != 1 || got.DefaultTags[0] != "team" {
		t.Errorf("DefaultTags = %v, want [team]", got.DefaultTags)
	}

	input, _ := gqlClient.Requests()[0].Variables["input"].(map[string]any)
	if input["namingConvention"] != convention {
		t.Errorf("input.namingConvention = %v", input["namingConvention"])
	}
	sent, _ := input["defaultTags"].([]any)
	if len(sent) != 1 || sent[0] != "team" {
		t.Errorf("input.defaultTags = %v, want [team]", input["defaultTags"])
	}
	if _, present := input["defaultBundleAccess"]; present {
		t.Errorf("defaultBundleAccess should be omitted when unset, got %v", input["defaultBundleAccess"])
	}
}

func TestUpdateSettings_ClearsNamingConventionAndTags(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateOrganizationSettings": map[string]any{
				"result": map[string]any{
					"id": "ecomm-corp",
					"settings": map[string]any{
						"defaultBundleAccess": "NONE",
						"namingConvention":    nil,
						"defaultTags":         []any{},
					},
				},
				"successful": true,
			},
		}),
	)

	empty := ""
	var nilTags []string
	_, err := newService(gqlClient).UpdateSettings(t.Context(), organizations.UpdateSettingsInput{
		NamingConvention: &empty,
		DefaultTags:      &nilTags,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	input, _ := gqlClient.Requests()[0].Variables["input"].(map[string]any)
	if v, present := input["namingConvention"]; !present || v != "" {
		t.Errorf("input.namingConvention = %v (present=%v), want \"\"", v, present)
	}
	sent, ok := input["defaultTags"].([]any)
	if !ok || len(sent) != 0 {
		t.Errorf("input.defaultTags = %#v, want []", input["defaultTags"])
	}
}
