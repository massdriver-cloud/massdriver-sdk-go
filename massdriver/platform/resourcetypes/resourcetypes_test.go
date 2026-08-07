package resourcetypes_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/resourcetypes"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

func newService(gqlClient *gqltest.Client) *resourcetypes.Service {
	return resourcetypes.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"resourceType": map[string]any{
				"id":                    "aws-iam-role",
				"name":                  "AWS IAM Role",
				"icon":                  "https://cdn.example.com/aws-iam-role.svg",
				"connectionOrientation": "LINK",
				"schema": map[string]any{
					"properties": map[string]any{
						"data": map[string]any{"type": "object"},
					},
				},
				"uiSchema": map[string]any{"data": map[string]any{"ui:order": []string{"arn"}}},
				"instructions": []map[string]any{
					{"label": "AWS CLI", "content": "Run `aws iam get-role`..."},
					{"label": "AWS Console", "content": "Open the IAM console..."},
				},
				"effectiveAttributes": map[string]any{"md-id": map[string]any{"type": "string"}},
				"createdAt":           "2026-05-08T10:00:00Z",
				"updatedAt":           "2026-05-08T11:00:00Z",
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "aws-iam-role")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "aws-iam-role" {
		t.Errorf("ID = %q, want aws-iam-role", got.ID)
	}
	if got.ConnectionOrientation != types.ConnectionOrientationLink {
		t.Errorf("ConnectionOrientation = %q, want LINK", got.ConnectionOrientation)
	}
	if _, ok := got.Schema["properties"]; !ok {
		t.Errorf("Schema = %v, want properties key", got.Schema)
	}
	if len(got.Instructions) != 2 || got.Instructions[0].Label != "AWS CLI" {
		t.Errorf("Instructions = %+v, want 2 entries starting with AWS CLI", got.Instructions)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt is zero; want parsed timestamp")
	}

	reqs := gqlClient.Requests()
	if reqs[0].OpName != "GetResourceType" {
		t.Errorf("OpName = %q, want GetResourceType", reqs[0].OpName)
	}
	if reqs[0].Variables["id"] != "aws-iam-role" {
		t.Errorf("id variable = %v, want aws-iam-role", reqs[0].Variables["id"])
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing resource type (the schema's `resourceType`
// field is nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"resourceType": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}
