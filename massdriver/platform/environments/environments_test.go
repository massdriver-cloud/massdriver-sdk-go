package environments_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/environments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// newService builds a *environments.Service backed by the provided gqltest mock,
// preconfigured with an organization ID so the wrapper has something to
// substitute into request variables.
func newService(gqlClient *gqltest.Client) *environments.Service {
	return environments.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"environment": map[string]any{
				"id":          "ecomm-prod",
				"name":        "Production",
				"description": "prod env",
				"project": map[string]any{
					"id":          "ecomm",
					"name":        "E-Commerce",
					"description": "Storefront",
					"attributes":  map[string]any{"team": "platform"},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "ecomm-prod")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "ecomm-prod" {
		t.Errorf("ID = %q, want ecomm-prod", got.ID)
	}
	// The embedded Project is the same type as platform/projects.Project; assert
	// the fuller shape populates so callers can read description/attributes
	// without re-fetching.
	if got.Project == nil {
		t.Fatal("Project is nil")
	}
	if got.Project.ID != "ecomm" {
		t.Errorf("Project.ID = %q, want ecomm", got.Project.ID)
	}
	if got.Project.Name != "E-Commerce" {
		t.Errorf("Project.Name = %q, want E-Commerce", got.Project.Name)
	}
	if got.Project.Description != "Storefront" {
		t.Errorf("Project.Description = %q, want Storefront", got.Project.Description)
	}
	if got.Project.Attributes["team"] != "platform" {
		t.Errorf("Project.Attributes[team] = %v, want platform", got.Project.Attributes["team"])
	}
}

// TestGet_ProjectIsSharedType asserts Environment.Project is the same type as
// platform/projects.Project — caller code that took *projects.Project before
// keeps working without conversion.
func TestGet_ProjectIsSharedType(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"environment": map[string]any{
				"id":      "ecomm-prod",
				"project": map[string]any{"id": "ecomm", "name": "E-Commerce"},
			},
		}),
	)
	got, err := newService(gqlClient).Get(t.Context(), "ecomm-prod")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// If the type ever drifts from *projects.Project this assignment fails to
	// compile — the test exists to keep that contract enforced.
	var _ *projects.Project = got.Project
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing environment (the schema's `environment`
// field is nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"environment": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"environments": map[string]any{
				"items": []map[string]any{
					{"id": "ecomm-staging", "name": "Staging", "project": map[string]any{"id": "ecomm", "name": "E-Commerce"}},
					{"id": "ecomm-prod", "name": "Production", "project": map[string]any{"id": "ecomm", "name": "E-Commerce"}},
				},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), environments.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d environments, want 2", len(got))
	}
	if got[0].ID != "ecomm-staging" || got[1].ID != "ecomm-prod" {
		t.Errorf("got IDs %v, want [ecomm-staging, ecomm-prod]", []string{got[0].ID, got[1].ID})
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createEnvironment": map[string]any{
				"result": map[string]any{
					"id":   "ecomm-prod",
					"name": "Production",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), "ecomm", environments.CreateInput{
		ID:   "prod",
		Name: "Production",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "ecomm-prod" {
		t.Errorf("ID = %q, want ecomm-prod", got.ID)
	}

	// Verify the wrapper passed projectId through as a top-level variable.
	reqs := gqlClient.Requests()
	if reqs[0].Variables["projectId"] != "ecomm" {
		t.Errorf("projectId variable = %v, want ecomm", reqs[0].Variables["projectId"])
	}
}

func TestSetDefault(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"setEnvironmentDefault": map[string]any{
				"result": map[string]any{
					"id": "envdef-1",
					"resource": map[string]any{
						"id":   "res-1",
						"name": "default-vpc",
						"resourceType": map[string]any{
							"id":   "aws-vpc",
							"name": "AWS VPC",
						},
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).SetDefault(t.Context(), "ecomm-prod", "res-1")
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if got.ID != "envdef-1" {
		t.Errorf("ID = %q, want envdef-1", got.ID)
	}
	if got.Resource == nil || got.Resource.ID != "res-1" {
		t.Errorf("Resource = %+v, want ID res-1", got.Resource)
	}
	if got.Resource.ResourceType == nil || got.Resource.ResourceType.ID != "aws-vpc" {
		t.Errorf("ResourceType = %+v, want ID aws-vpc", got.Resource.ResourceType)
	}
}

func TestSetDefault_ValidationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"setEnvironmentDefault": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "conflict", "field": "resourceId", "message": "a default of this resource type already exists"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).SetDefault(t.Context(), "ecomm-prod", "res-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if mf.Op != "set environment default" {
		t.Errorf("Op = %q, want set environment default", mf.Op)
	}
}

func TestRemoveDefault(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"removeEnvironmentDefault": map[string]any{
				"result": map[string]any{
					"id": "envdef-1",
					"resource": map[string]any{
						"id":   "res-1",
						"name": "default-vpc",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).RemoveDefault(t.Context(), "envdef-1")
	if err != nil {
		t.Fatalf("RemoveDefault: %v", err)
	}
	if got.ID != "envdef-1" {
		t.Errorf("ID = %q, want envdef-1", got.ID)
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateEnvironment": map[string]any{
				"result": map[string]any{
					"id":   "ecomm-prod",
					"name": "Production (renamed)",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), "ecomm-prod", environments.UpdateInput{
		Name: "Production (renamed)",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "Production (renamed)" {
		t.Errorf("Name = %q, want Production (renamed)", got.Name)
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteEnvironment": map[string]any{
				"result": map[string]any{
					"id":   "ecomm-prod",
					"name": "Production",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Delete(t.Context(), "ecomm-prod")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got.ID != "ecomm-prod" {
		t.Errorf("ID = %q, want ecomm-prod", got.ID)
	}
}

func TestFork(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"forkEnvironment": map[string]any{
				"result": map[string]any{
					"id":   "ecomm-pr-123",
					"name": "PR-123 preview",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Fork(t.Context(), "ecomm-prod", environments.ForkInput{
		ID:          "ecomm-pr-123",
		Name:        "PR-123 preview",
		CopySecrets: true,
	})
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	if got.ID != "ecomm-pr-123" {
		t.Errorf("ID = %q, want ecomm-pr-123", got.ID)
	}
}

// TestFork_Idempotent confirms a re-fork returns the same environment
// (server's converge behavior). The SDK has nothing special to do here —
// this just locks in the contract from the caller's perspective.
func TestFork_Idempotent(t *testing.T) {
	resp := gqltest.RespondWithData(map[string]any{
		"forkEnvironment": map[string]any{
			"result":     map[string]any{"id": "ecomm-pr-123", "name": "PR-123 preview"},
			"successful": true,
		},
	})
	gqlClient := gqltest.NewClient(resp, resp)

	svc := newService(gqlClient)
	first, _ := svc.Fork(t.Context(), "ecomm-prod", environments.ForkInput{ID: "ecomm-pr-123", Name: "PR-123 preview"})
	second, err := svc.Fork(t.Context(), "ecomm-prod", environments.ForkInput{ID: "ecomm-pr-123", Name: "PR-123 preview"})
	if err != nil {
		t.Fatalf("re-Fork: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("re-fork returned different env: %q vs %q", first.ID, second.ID)
	}
}

// TestFork_ParentConflict locks in the *MutationFailedError surface for
// the "same id, different parent" rejection.
func TestFork_ParentConflict(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"forkEnvironment": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "IMMUTABLE", "field": "parentId", "message": "parent is immutable on an existing fork"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).Fork(t.Context(), "ecomm-staging", environments.ForkInput{ID: "ecomm-pr-123", Name: "PR-123"})
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if len(mf.Messages) != 1 || mf.Messages[0].Field != "parentId" {
		t.Errorf("messages = %+v, want one entry for parentId", mf.Messages)
	}
}

func TestDeploy(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deployEnvironment": map[string]any{
				"result":     map[string]any{"id": "ecomm-pr-123", "name": "PR-123 preview"},
				"successful": true,
			},
		}),
	)
	got, err := newService(gqlClient).Deploy(t.Context(), "ecomm-pr-123")
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if got.ID != "ecomm-pr-123" {
		t.Errorf("ID = %q, want ecomm-pr-123", got.ID)
	}
}

func TestDecommission(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"decommissionEnvironment": map[string]any{
				"result":     map[string]any{"id": "ecomm-pr-123", "name": "PR-123 preview"},
				"successful": true,
			},
		}),
	)
	got, err := newService(gqlClient).Decommission(t.Context(), "ecomm-pr-123")
	if err != nil {
		t.Fatalf("Decommission: %v", err)
	}
	if got.ID != "ecomm-pr-123" {
		t.Errorf("ID = %q, want ecomm-pr-123", got.ID)
	}
}

func TestDecommissionProtected(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"decommissionEnvironment": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "decommission_protected", "field": "decommissionProtection", "message": "decommission protection is enabled"},
				},
			},
		}),
	)
	_, err := newService(gqlClient).Decommission(t.Context(), "ecomm-prod")
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if len(mf.Messages) != 1 || mf.Messages[0].Field != "decommissionProtection" {
		t.Errorf("messages = %+v, want one entry for decommissionProtection", mf.Messages)
	}
}
