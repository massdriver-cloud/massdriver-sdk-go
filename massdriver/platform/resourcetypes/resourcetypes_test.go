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
				"id":                    "aws-iam-role@1.2.3",
				"name":                  "AWS IAM Role",
				"version":               "1.2.3",
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
	if got.ID != "aws-iam-role@1.2.3" {
		t.Errorf("ID = %q, want aws-iam-role@1.2.3", got.ID)
	}
	if got.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", got.Version)
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

func TestDependents(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"resourceTypeDependents": []map[string]any{
				{
					"instance": map[string]any{"id": "ecomm-prod-api", "name": "api"},
					"field":    "network",
					"resourceType": map[string]any{
						"id":   "aws-vpc@0.0.0",
						"name": "AWS VPC",
						"icon": "https://cdn.example.com/aws-vpc.svg",
					},
				},
				{
					"instance": map[string]any{"id": "ecomm-prod-db", "name": "db"},
					"field":    "vpc",
					"resourceType": map[string]any{
						"id":   "aws-vpc@0.0.0",
						"name": "AWS VPC",
						"icon": "https://cdn.example.com/aws-vpc.svg",
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Dependents(t.Context(), "ecomm-prod", "aws-vpc")
	if err != nil {
		t.Fatalf("Dependents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Instance.ID != "ecomm-prod-api" || got[0].Field != "network" {
		t.Errorf("got[0] = instance %q field %q, want ecomm-prod-api / network", got[0].Instance.ID, got[0].Field)
	}
	if got[1].ResourceType.ID != "aws-vpc@0.0.0" {
		t.Errorf("got[1].ResourceType.ID = %q, want aws-vpc@0.0.0", got[1].ResourceType.ID)
	}

	reqs := gqlClient.Requests()
	if reqs[0].OpName != "ListResourceTypeDependents" {
		t.Errorf("OpName = %q, want ListResourceTypeDependents", reqs[0].OpName)
	}
	if reqs[0].Variables["environmentId"] != "ecomm-prod" || reqs[0].Variables["resourceTypeId"] != "aws-vpc" {
		t.Errorf("variables = %v, want environmentId ecomm-prod and resourceTypeId aws-vpc", reqs[0].Variables)
	}
}

func TestDependents_Empty(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"resourceTypeDependents": []map[string]any{},
		}),
	)
	got, err := newService(gqlClient).Dependents(t.Context(), "ecomm-prod", "aws-vpc")
	if err != nil {
		t.Fatalf("Dependents: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}
