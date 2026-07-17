package bundles_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/bundles"
)

func newService(gqlClient *gqltest.Client) *bundles.Service {
	return bundles.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"bundle": map[string]any{
				"id":          "aws-aurora-postgres@1.2.3",
				"name":        "aws-aurora-postgres",
				"version":     "1.2.3",
				"description": "Aurora Postgres cluster",
				"repo":        "aws-aurora-postgres",
				"dependencies": []map[string]any{
					{
						"name":         "aws_authentication",
						"required":     true,
						"resourceType": map[string]any{"id": "aws-iam-role", "name": "AWS IAM Role"},
					},
				},
				"resources": []map[string]any{
					{
						"name":         "database",
						"required":     true,
						"resourceType": map[string]any{"id": "aws-rds-postgres", "name": "AWS RDS Postgres"},
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "aws-aurora-postgres@1.2.3")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "aws-aurora-postgres@1.2.3" {
		t.Errorf("ID = %q, want aws-aurora-postgres@1.2.3", got.ID)
	}
	if got.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", got.Version)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0].Name != "aws_authentication" {
		t.Errorf("Dependencies = %+v, want one named aws_authentication", got.Dependencies)
	}
	if len(got.Resources) != 1 || got.Resources[0].ResourceType == nil || got.Resources[0].ResourceType.ID != "aws-rds-postgres" {
		t.Errorf("Resources = %+v, want one with type aws-rds-postgres", got.Resources)
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing bundle (the schema's `bundle` field is
// nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"bundle": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing@1.0.0")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}
