package instances_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
)

func TestSetSecret(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"setInstanceSecret": map[string]any{
				"result": map[string]any{
					"name":   "DB_PASSWORD",
					"sha256": "abc123def456",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).SetSecret(t.Context(), "ecomm-prod-database", "DB_PASSWORD", "supersecret")
	if err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	if got.Name != "DB_PASSWORD" {
		t.Errorf("Name = %q, want DB_PASSWORD", got.Name)
	}
	if got.SHA256 != "abc123def456" {
		t.Errorf("SHA256 = %q, want abc123def456", got.SHA256)
	}

	// The secret VALUE must reach the wire — confirm it's in the input.
	reqs := gqlClient.Requests()
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["value"] != "supersecret" {
		t.Errorf("input.value = %v, want supersecret", input["value"])
	}
}

func TestRemoveSecret(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"removeInstanceSecret": map[string]any{
				"result": map[string]any{
					"name":   "DB_PASSWORD",
					"sha256": "abc123def456",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).RemoveSecret(t.Context(), "ecomm-prod-database", "DB_PASSWORD")
	if err != nil {
		t.Fatalf("RemoveSecret: %v", err)
	}
	if got.Name != "DB_PASSWORD" {
		t.Errorf("Name = %q, want DB_PASSWORD", got.Name)
	}

	// The secret name reaches the wire as a top-level variable, not as input.
	reqs := gqlClient.Requests()
	if reqs[0].Variables["name"] != "DB_PASSWORD" {
		t.Errorf("name variable = %v, want DB_PASSWORD", reqs[0].Variables["name"])
	}
}
