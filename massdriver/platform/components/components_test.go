package components_test

import (
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/components"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// newService builds a *components.Service backed by the provided gqltest mock,
// preconfigured with an organization ID so the wrapper has something to
// substitute into request variables.
func newService(gqlClient *gqltest.Client) *components.Service {
	return components.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"component": map[string]any{
				"id":          "ecomm-database",
				"name":        "Primary Database",
				"description": "User data",
				"attributes":  map[string]any{"team": "platform"},
				"ociRepo": map[string]any{
					"id":           "aws-aurora-postgres",
					"name":         "aws-aurora-postgres",
					"reference":    "api.massdriver.cloud/my-org/aws-aurora-postgres",
					"artifactType": "application/vnd.massdriver.bundle.v1+json",
				},
				"project": map[string]any{
					"id":   "ecomm",
					"name": "E-Commerce",
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "ecomm-database")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "ecomm-database" {
		t.Errorf("ID = %q, want ecomm-database", got.ID)
	}
	if got.Name != "Primary Database" {
		t.Errorf("Name = %q, want Primary Database", got.Name)
	}
	// OciRepo and Project must be the canonical types — same identity as
	// ocirepos.OciRepo and projects.Project.
	if got.OciRepo == nil || got.OciRepo.ID != "aws-aurora-postgres" {
		t.Errorf("OciRepo = %+v, want ID aws-aurora-postgres", got.OciRepo)
	}
	if got.Project == nil || got.Project.ID != "ecomm" {
		t.Errorf("Project = %+v, want ID ecomm", got.Project)
	}

	// Compile-time check: the embedded types are the shared types.* types.
	var _ *types.OciRepo = got.OciRepo
	var _ *types.Project = got.Project
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing component (the schema's `component` field
// is nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"component": nil,
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
			"project": map[string]any{
				"id": "ecomm",
				"components": []map[string]any{
					{"id": "ecomm-database", "name": "Primary Database",
						"ociRepo": map[string]any{"id": "aws-aurora-postgres", "name": "aws-aurora-postgres"}},
					{"id": "ecomm-cache", "name": "Cache",
						"ociRepo": map[string]any{"id": "redis", "name": "redis"}},
				},
			},
		}),
	)

	got, err := newService(gqlClient).List(t.Context(), components.ListInput{ProjectID: "ecomm"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "ecomm-database" {
		t.Errorf("got[0].ID = %q, want ecomm-database", got[0].ID)
	}
}

func TestList_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{"project": nil}),
	)
	_, err := newService(gqlClient).List(t.Context(), components.ListInput{ProjectID: "nope"})
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestAdd(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"addComponent": map[string]any{
				"result": map[string]any{
					"id":   "ecomm-cache",
					"name": "Cache",
					"ociRepo": map[string]any{
						"id":   "redis",
						"name": "redis",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Add(t.Context(), "ecomm", components.AddInput{
		OciRepoName: "redis",
		ID:          "cache",
		Name:        "Cache",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if got.ID != "ecomm-cache" {
		t.Errorf("ID = %q, want ecomm-cache", got.ID)
	}

	// Wrapper must pass projectId and ociRepoName as top-level variables, not
	// as fields on input.
	reqs := gqlClient.Requests()
	if reqs[0].Variables["projectId"] != "ecomm" {
		t.Errorf("projectId variable = %v, want ecomm", reqs[0].Variables["projectId"])
	}
	if reqs[0].Variables["ociRepoName"] != "redis" {
		t.Errorf("ociRepoName variable = %v, want redis", reqs[0].Variables["ociRepoName"])
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateComponent": map[string]any{
				"result": map[string]any{
					"id":   "ecomm-database",
					"name": "Primary Database (renamed)",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), "ecomm-database", components.UpdateInput{
		Name: "Primary Database (renamed)",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "Primary Database (renamed)" {
		t.Errorf("Name = %q, want Primary Database (renamed)", got.Name)
	}
}

func TestRemove(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"removeComponent": map[string]any{
				"result":     map[string]any{"id": "ecomm-cache", "name": "Cache"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Remove(t.Context(), "ecomm-cache")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got.ID != "ecomm-cache" {
		t.Errorf("ID = %q, want ecomm-cache", got.ID)
	}
}

func TestAddLink(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"linkComponents": map[string]any{
				"result": map[string]any{
					"id":        "link-1",
					"fromField": "authentication",
					"toField":   "database",
					"fromComponent": map[string]any{
						"id":   "ecomm-database",
						"name": "Primary Database",
					},
					"toComponent": map[string]any{
						"id":   "ecomm-app",
						"name": "App",
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).AddLink(t.Context(), components.AddLinkInput{
		FromComponentID: "ecomm-database",
		FromField:       "authentication",
		FromVersion:     "~1.0",
		ToComponentID:   "ecomm-app",
		ToField:         "database",
		ToVersion:       "~2.0",
	})
	if err != nil {
		t.Fatalf("AddLink: %v", err)
	}
	if got.ID != "link-1" {
		t.Errorf("ID = %q, want link-1", got.ID)
	}
	if got.FromField != "authentication" || got.ToField != "database" {
		t.Errorf("Fields = (%q, %q), want (authentication, database)", got.FromField, got.ToField)
	}
	if got.FromComponent == nil || got.FromComponent.ID != "ecomm-database" {
		t.Errorf("FromComponent = %+v, want ID ecomm-database", got.FromComponent)
	}
	if got.ToComponent == nil || got.ToComponent.ID != "ecomm-app" {
		t.Errorf("ToComponent = %+v, want ID ecomm-app", got.ToComponent)
	}

	// Compile-time check: From/ToComponent are the shared *types.Component.
	var _ *types.Component = got.FromComponent
	var _ *types.Component = got.ToComponent
}

func TestAddLink_ValidationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"linkComponents": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "incompatible", "field": "fromField", "message": "field types don't match"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).AddLink(t.Context(), components.AddLinkInput{
		FromComponentID: "a", FromField: "x", FromVersion: "1",
		ToComponentID: "b", ToField: "y", ToVersion: "1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if mf.Op != "link components" {
		t.Errorf("Op = %q, want link components", mf.Op)
	}
}

func TestRemoveLink(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"unlinkComponents": map[string]any{
				"result":     map[string]any{"id": "link-1", "fromField": "authentication", "toField": "database"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).RemoveLink(t.Context(), "link-1")
	if err != nil {
		t.Fatalf("RemoveLink: %v", err)
	}
	if got.ID != "link-1" {
		t.Errorf("ID = %q, want link-1", got.ID)
	}
}
