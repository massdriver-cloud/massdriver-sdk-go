package projects_test

import (
	"errors"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
)

// newService builds a *projects.Service backed by the provided gqltest mock,
// preconfigured with an organization ID so the wrapper has something to
// substitute into request variables.
func newService(gqlClient *gqltest.Client) *projects.Service {
	return projects.New(&client.Client{
		Config: config.Config{OrganizationID: "my-org"},
		GQLv2:  gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"project": map[string]any{
				"id":          "proj-1",
				"name":        "My Project",
				"description": "demo",
				"attributes":  map[string]any{"team": "eng"},
				"environments": map[string]any{
					"items": []map[string]any{
						{"id": "proj-1-staging", "name": "Staging"},
						{"id": "proj-1-prod", "name": "Production"},
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "proj-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "proj-1" {
		t.Errorf("ID = %q, want proj-1", got.ID)
	}
	if got.Name != "My Project" {
		t.Errorf("Name = %q, want My Project", got.Name)
	}
	if got.Attributes["team"] != "eng" {
		t.Errorf("Attributes[team] = %v, want eng", got.Attributes["team"])
	}
	// Environments should populate from the paginated `environments.items` page
	// — verifies the secondary-decode unwrap in toProject.
	if len(got.Environments) != 2 {
		t.Fatalf("Environments len = %d, want 2", len(got.Environments))
	}
	if got.Environments[0].ID != "proj-1-staging" || got.Environments[1].ID != "proj-1-prod" {
		t.Errorf("Environments IDs = %v, want [proj-1-staging, proj-1-prod]",
			[]string{got.Environments[0].ID, got.Environments[1].ID})
	}

	reqs := gqlClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].OpName != "GetProject" {
		t.Errorf("OpName = %q, want GetProject", reqs[0].OpName)
	}
	if reqs[0].Variables["organizationId"] != "my-org" {
		t.Errorf("organizationId = %v, want my-org", reqs[0].Variables["organizationId"])
	}
	if reqs[0].Variables["id"] != "proj-1" {
		t.Errorf("id variable = %v, want proj-1", reqs[0].Variables["id"])
	}
	if gqlClient.Pending() != 0 {
		t.Errorf("Pending = %d, want 0", gqlClient.Pending())
	}
}

// TestGet_WithComponentsAndLinks confirms that the eager-loaded components
// and links arrive populated on the embedded slices.
func TestGet_WithComponentsAndLinks(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"project": map[string]any{
				"id":   "ecomm",
				"name": "E-Commerce",
				"components": []map[string]any{
					{
						"id":      "ecomm-database",
						"name":    "Primary Database",
						"ociRepo": map[string]any{"id": "aws-aurora-postgres", "name": "aws-aurora-postgres"},
					},
					{
						"id":      "ecomm-app",
						"name":    "App",
						"ociRepo": map[string]any{"id": "kubernetes-deployment", "name": "kubernetes-deployment"},
					},
				},
				"links": []map[string]any{
					{
						"id":            "link-1",
						"fromField":     "authentication",
						"toField":       "database",
						"fromComponent": map[string]any{"id": "ecomm-database", "name": "Primary Database"},
						"toComponent":   map[string]any{"id": "ecomm-app", "name": "App"},
					},
				},
			},
		}),
	)

	got, err := newService(gqlClient).Get(t.Context(), "ecomm")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Components) != 2 {
		t.Fatalf("Components len = %d, want 2", len(got.Components))
	}
	if got.Components[0].ID != "ecomm-database" {
		t.Errorf("Components[0].ID = %q, want ecomm-database", got.Components[0].ID)
	}
	if got.Components[0].OciRepo == nil || got.Components[0].OciRepo.Name != "aws-aurora-postgres" {
		t.Errorf("Components[0].OciRepo = %+v, want name aws-aurora-postgres", got.Components[0].OciRepo)
	}
	if len(got.Links) != 1 {
		t.Fatalf("Links len = %d, want 1", len(got.Links))
	}
	if got.Links[0].ID != "link-1" {
		t.Errorf("Links[0].ID = %q, want link-1", got.Links[0].ID)
	}
	if got.Links[0].FromComponent == nil || got.Links[0].FromComponent.ID != "ecomm-database" {
		t.Errorf("Links[0].FromComponent = %+v, want ID ecomm-database", got.Links[0].FromComponent)
	}
}

// TestGet_NoEnvironments confirms the Environments slice is nil (not panicking)
// when the server returns an empty page.
func TestGet_NoEnvironments(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"project": map[string]any{
				"id":           "empty",
				"name":         "Empty",
				"environments": map[string]any{"items": []map[string]any{}},
			},
		}),
	)
	got, err := newService(gqlClient).Get(t.Context(), "empty")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Environments) != 0 {
		t.Errorf("Environments len = %d, want 0", len(got.Environments))
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing project (the schema's `project` field is
// nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"project": nil,
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
			"projects": map[string]any{
				"items": []map[string]any{
					{"id": "proj-a", "name": "A"},
					{"id": "proj-b", "name": "B"},
				},
			},
		}),
	)

	got, err := newService(gqlClient).List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2", len(got))
	}
	if got[0].ID != "proj-a" || got[1].ID != "proj-b" {
		t.Errorf("got IDs %v, want [proj-a, proj-b]", []string{got[0].ID, got[1].ID})
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createProject": map[string]any{
				"result": map[string]any{
					"id":          "new-proj",
					"name":        "New Project",
					"description": "A new project",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), projects.CreateInput{
		ID:          "new-proj",
		Name:        "New Project",
		Description: "A new project",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "new-proj" {
		t.Errorf("ID = %q, want new-proj", got.ID)
	}

	// Sanity-check the recorded variables: the wrapper must send the input fields
	// through to the server in the right shape.
	reqs := gqlClient.Requests()
	input, ok := reqs[0].Variables["input"].(map[string]any)
	if !ok {
		t.Fatalf("input variable missing or wrong type: %v", reqs[0].Variables)
	}
	if input["id"] != "new-proj" {
		t.Errorf("input.id = %v, want new-proj", input["id"])
	}
	if input["name"] != "New Project" {
		t.Errorf("input.name = %v, want New Project", input["name"])
	}
}

func TestCreate_ValidationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createProject": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "required", "field": "name", "message": "name is required"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).Create(t.Context(), projects.CreateInput{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	mf, ok := gql.AsMutationFailed(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailed, got %T: %v", err, err)
	}
	if mf.Op != "create project" {
		t.Errorf("Op = %q, want create project", mf.Op)
	}
	if len(mf.Messages) != 1 || mf.Messages[0].Field != "name" {
		t.Errorf("messages = %+v, want one message for field=name", mf.Messages)
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateProject": map[string]any{
				"result": map[string]any{
					"id":          "proj-1",
					"name":        "Renamed",
					"description": "updated",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), "proj-1", projects.UpdateInput{
		Name:        "Renamed",
		Description: "updated",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "Renamed" {
		t.Errorf("Name = %q, want Renamed", got.Name)
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteProject": map[string]any{
				"result": map[string]any{
					"id":   "proj-1",
					"name": "Removed",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Delete(t.Context(), "proj-1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got.ID != "proj-1" {
		t.Errorf("ID = %q, want proj-1", got.ID)
	}
}

func TestGet_TransportError(t *testing.T) {
	wantErr := errors.New("dial tcp: connection refused")
	gqlClient := gqltest.NewClient(gqltest.RespondWithTransportError(wantErr))

	_, err := newService(gqlClient).Get(t.Context(), "proj-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want it to wrap %v", err, wantErr)
	}
}

// TestGet_HTTP403Forbidden confirms that an HTTP 403 from the API
// surfaces gql.ErrForbidden via errors.Is — callers can distinguish
// "not allowed" from "doesn't exist" without parsing strings.
func TestGet_HTTP403Forbidden(t *testing.T) {
	gqlClient := gqltest.NewClient(gqltest.RespondWithTransportError(
		&graphql.HTTPError{StatusCode: 403},
	))

	_, err := newService(gqlClient).Get(t.Context(), "proj-1")
	if !errors.Is(err, gql.ErrForbidden) {
		t.Errorf("err = %v, want it to wrap gql.ErrForbidden", err)
	}
}
