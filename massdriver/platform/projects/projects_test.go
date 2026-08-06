package projects_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
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

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), projects.ListInput{}))
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

	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
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
		Name:        types.Ptr("Renamed"),
		Description: types.Ptr("updated"),
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

// TestList_ZeroInputOmitsFilter confirms a zero ListInput sends no filter at
// all — the server should see `filter` absent, not an empty object.
func TestList_ZeroInputOmitsFilter(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"projects": map[string]any{"items": []map[string]any{}},
		}),
	)

	if _, err := newService(gqlClient).ListPage(t.Context(), projects.ListInput{}); err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if f, ok := gqlClient.Requests()[0].Variables["filter"]; ok && f != nil {
		t.Errorf("filter variable = %v, want it omitted", f)
	}
}

func TestList_NameAndSearchFilters(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"projects": map[string]any{
				"items": []map[string]any{
					{"id": "ecomm", "name": "E-Commerce"},
				},
			},
		}),
	)

	got, err := newService(gqlClient).ListPage(t.Context(), projects.ListInput{
		Name:   "E-Commerce",
		Search: "commerce",
	})
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "ecomm" {
		t.Fatalf("Items = %+v, want one project ecomm", got.Items)
	}

	filter, _ := gqlClient.Requests()[0].Variables["filter"].(map[string]any)
	name, _ := filter["name"].(map[string]any)
	if name["eq"] != "E-Commerce" {
		t.Errorf("filter.name.eq = %v, want E-Commerce", name["eq"])
	}
	if filter["search"] != "commerce" {
		t.Errorf("filter.search = %v, want commerce", filter["search"])
	}
}

func TestList_NameInFilter(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"projects": map[string]any{"items": []map[string]any{}},
		}),
	)

	if _, err := newService(gqlClient).ListPage(t.Context(), projects.ListInput{
		NameIn: []string{"Staging", "Production"},
	}); err != nil {
		t.Fatalf("ListPage: %v", err)
	}

	filter, _ := gqlClient.Requests()[0].Variables["filter"].(map[string]any)
	name, _ := filter["name"].(map[string]any)
	in, _ := name["in"].([]any)
	if len(in) != 2 || in[0] != "Staging" || in[1] != "Production" {
		t.Errorf("filter.name.in = %v, want [Staging Production]", name["in"])
	}
	// An unset Name must not serialize: the server ANDs `eq` with `in`, so
	// sending `eq: ""` alongside `in` matches nothing.
	if _, present := name["eq"]; present {
		t.Errorf("filter.name.eq = %v, want the key absent", name["eq"])
	}
}

func TestList_CreatedAtFilter(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"projects": map[string]any{
				"cursor": map[string]any{},
				"items":  []map[string]any{{"id": "ecomm", "name": "E-Commerce"}},
			},
		}),
	)

	_, err := types.Collect(newService(gqlClient).Iter(t.Context(), projects.ListInput{
		CreatedAfter: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	reqs := gqlClient.Requests()
	filter, _ := reqs[0].Variables["filter"].(map[string]any)
	created, _ := filter["createdAt"].(map[string]any)
	if created["gte"] == nil {
		t.Errorf("createdAt = %v, want gte set", created)
	}
	// Only the lower bound was given — the open upper bound must stay off
	// the wire.
	if _, present := created["lte"]; present {
		t.Errorf("createdAt.lte should be omitted when CreatedBefore is zero, got %v", created["lte"])
	}
}

func TestUpdate_PartialOmitsUnsetFields(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateProject": map[string]any{
				"result": map[string]any{
					"id":          "proj-1",
					"name":        "Original",
					"description": "updated",
				},
				"successful": true,
			},
		}),
	)

	if _, err := newService(gqlClient).Update(t.Context(), "proj-1", projects.UpdateInput{
		Description: types.Ptr("updated"),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Nil fields must be omitted from the wire entirely — the server treats
	// an absent field as "leave unchanged" but rejects null and "" for name.
	input, _ := gqlClient.Requests()[0].Variables["input"].(map[string]any)
	if input["description"] != "updated" {
		t.Errorf("input.description = %v, want updated", input["description"])
	}
	for _, key := range []string{"name", "attributes"} {
		if _, present := input[key]; present {
			t.Errorf("input.%s = %v, want the key absent", key, input[key])
		}
	}
}

func TestClone(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"cloneProject": map[string]any{
				"result": map[string]any{
					"id":          "ecomm2",
					"name":        "E-Commerce EU",
					"description": "clone of ecomm",
					"components": []map[string]any{
						{
							"id":      "ecomm2-database",
							"name":    "Primary Database",
							"ociRepo": map[string]any{"id": "aws-aurora-postgres", "name": "aws-aurora-postgres"},
						},
					},
					"links": []map[string]any{
						{
							"id":            "link-1",
							"fromField":     "authentication",
							"toField":       "database",
							"fromComponent": map[string]any{"id": "ecomm2-database", "name": "Primary Database"},
							"toComponent":   map[string]any{"id": "ecomm2-app", "name": "App"},
						},
					},
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Clone(t.Context(), "ecomm", projects.CloneInput{
		ID:          "ecomm2",
		Name:        "E-Commerce EU",
		Description: "clone of ecomm",
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if got.ID != "ecomm2" {
		t.Errorf("ID = %q, want ecomm2", got.ID)
	}
	// The cloned blueprint must come back populated.
	if len(got.Components) != 1 || got.Components[0].ID != "ecomm2-database" {
		t.Errorf("Components = %+v, want one component ecomm2-database", got.Components)
	}
	if len(got.Links) != 1 || got.Links[0].FromField != "authentication" {
		t.Errorf("Links = %+v, want one link fromField=authentication", got.Links)
	}

	reqs := gqlClient.Requests()
	if reqs[0].Variables["sourceProjectId"] != "ecomm" {
		t.Errorf("sourceProjectId = %v, want ecomm", reqs[0].Variables["sourceProjectId"])
	}
	input, _ := reqs[0].Variables["input"].(map[string]any)
	if input["id"] != "ecomm2" || input["name"] != "E-Commerce EU" {
		t.Errorf("input = %v, want id=ecomm2 name=E-Commerce EU", input)
	}
}

func TestClone_ValidationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"cloneProject": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "taken", "field": "id", "message": "has already been taken"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).Clone(t.Context(), "ecomm", projects.CloneInput{ID: "ecomm"})
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if mf.Op != "clone project" {
		t.Errorf("Op = %q, want clone project", mf.Op)
	}
}
