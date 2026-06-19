package organizations_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/organizations"
)

func newService(gqlClient *gqltest.Client) *organizations.Service {
	return organizations.New(&client.Client{
		Config: config.Config{OrganizationID: "ecomm-corp"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"organization": map[string]any{
				"id":                 "ecomm-corp",
				"name":               "E-Commerce Corp",
				"subscriptionStatus": "ACTIVE",
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SubscriptionStatus != string(organizations.SubscriptionActive) {
		t.Errorf("SubscriptionStatus = %q, want ACTIVE", got.SubscriptionStatus)
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing organization.
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"organization": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context())
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateOrganization": map[string]any{
				"result":     map[string]any{"id": "ecomm-corp", "name": "E-Commerce Corp (renamed)"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), organizations.UpdateInput{
		Name: "E-Commerce Corp (renamed)",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "E-Commerce Corp (renamed)" {
		t.Errorf("Name = %q, want E-Commerce Corp (renamed)", got.Name)
	}
}

func TestRemoveMember(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteOrganizationMember": map[string]any{
				"result":     map[string]any{"email": "alice@example.com"},
				"successful": true,
			},
		}),
	)

	if err := newService(gqlClient).RemoveMember(t.Context(), "alice@example.com"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
}

func TestCreateCustomAttribute(t *testing.T) {
	required := true
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createCustomAttribute": map[string]any{
				"result": map[string]any{
					"id":       "attr-1",
					"key":      "TEAM",
					"scope":    "PROJECT",
					"required": true,
					"values":   []string{"platform", "data", "frontend"},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).CreateCustomAttribute(t.Context(), organizations.CreateCustomAttributeInput{
		Key:      "TEAM",
		Scope:    organizations.AttributeScopeProject,
		Required: &required,
		Values:   []string{"platform", "data", "frontend"},
	})
	if err != nil {
		t.Fatalf("CreateCustomAttribute: %v", err)
	}
	if got.Key != "TEAM" {
		t.Errorf("Key = %q, want TEAM", got.Key)
	}
	if got.Scope != string(organizations.AttributeScopeProject) {
		t.Errorf("Scope = %q, want PROJECT", got.Scope)
	}
	if !got.Required {
		t.Errorf("Required = false, want true")
	}
}

func TestUpdateCustomAttribute(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateCustomAttribute": map[string]any{
				"result": map[string]any{
					"id":       "attr-1",
					"key":      "TEAM",
					"scope":    "PROJECT",
					"required": true,
					"values":   []string{"platform", "data", "frontend", "ml"},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).UpdateCustomAttribute(t.Context(), "attr-1", organizations.UpdateCustomAttributeInput{
		Values: []string{"platform", "data", "frontend", "ml"},
	})
	if err != nil {
		t.Fatalf("UpdateCustomAttribute: %v", err)
	}
	if len(got.Values) != 4 {
		t.Errorf("Values len = %d, want 4", len(got.Values))
	}
}

func TestDeleteCustomAttribute(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteCustomAttribute": map[string]any{
				"result":     map[string]any{"id": "attr-1", "key": "TEAM", "scope": "PROJECT"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).DeleteCustomAttribute(t.Context(), "attr-1")
	if err != nil {
		t.Fatalf("DeleteCustomAttribute: %v", err)
	}
	if got.Key != "TEAM" {
		t.Errorf("Key = %q, want TEAM", got.Key)
	}
}
