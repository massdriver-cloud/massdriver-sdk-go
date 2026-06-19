package ocirepos_test

import (
	"errors"
	"testing"

	"oras.land/oras-go/v2/registry/remote"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/ocirepos"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// newService builds a *ocirepos.Service backed by the provided gqltest mock,
// preconfigured with an organization ID, base URL, and bearer credentials so
// the wrapper has enough context for both the GraphQL CRUD and Target paths.
func newService(gqlClient *gqltest.Client) *ocirepos.Service {
	return ocirepos.New(&client.Client{
		Config: config.Config{
			OrganizationID: "my-org",
			URL:            "https://api.massdriver.cloud",
			Credentials:    config.Credentials{AuthHeaderValue: "Bearer test-token"},
		},
		GQLv2: gqlClient,
	})
}

func TestGet(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"ociRepo": map[string]any{
				"id":           "aws-aurora-postgres",
				"name":         "aws-aurora-postgres",
				"reference":    "api.massdriver.cloud/my-org/aws-aurora-postgres",
				"artifactType": "application/vnd.massdriver.bundle.v1+json",
				"attributes":   map[string]any{"team": "platform"},
			},
		}),
	)
	got, err := newService(gqlClient).Get(t.Context(), "aws-aurora-postgres")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "aws-aurora-postgres" {
		t.Errorf("ID = %q, want aws-aurora-postgres", got.ID)
	}
	if got.Reference != "api.massdriver.cloud/my-org/aws-aurora-postgres" {
		t.Errorf("Reference = %q, want api.massdriver.cloud/my-org/aws-aurora-postgres", got.Reference)
	}
	if got.Attributes["team"] != "platform" {
		t.Errorf("Attributes[team] = %v, want platform", got.Attributes["team"])
	}
}

// TestGet_NotFound confirms the wrapper surfaces gql.ErrNotFound when the
// API returns null for a missing repository (the schema's `ociRepo` field is
// nullable, so a 404 manifests as a zero-valued struct on the wire).
func TestGet_NotFound(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"ociRepo": nil,
		}),
	)
	_, err := newService(gqlClient).Get(t.Context(), "missing")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("err = %v, want it to wrap gql.ErrNotFound", err)
	}
}

func TestList_NoFilter(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"ociRepos": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "aws-aurora-postgres", "name": "aws-aurora-postgres", "artifactType": "application/vnd.massdriver.bundle.v1+json"},
					{"id": "aws-s3-bucket", "name": "aws-s3-bucket", "artifactType": "application/vnd.massdriver.bundle.v1+json"},
				},
			},
		}),
	)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), ocirepos.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d repos, want 2", len(got))
	}

	// With zero ListInput, filter and sort variables should be omitted
	// (sent as null). The wrapper returning nil for both is what produces
	// that on the wire.
	vars := gqlClient.Requests()[0].Variables
	if vars["filter"] != nil {
		t.Errorf("filter variable should be null for empty ListInput, got %v", vars["filter"])
	}
	if vars["sort"] != nil {
		t.Errorf("sort variable should be null for empty ListInput, got %v", vars["sort"])
	}
}

func TestList_WithNameFilter(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"ociRepos": map[string]any{
				"cursor": map[string]any{},
				"items": []map[string]any{
					{"id": "aws-rds", "name": "aws-rds", "artifactType": "application/vnd.massdriver.bundle.v1+json"},
				},
			},
		}),
	)

	_, err := types.Collect(newService(gqlClient).Iter(t.Context(), ocirepos.ListInput{
		NameStartsWith: "aws-",
	}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// The wrapper must translate NameStartsWith into the nested
	// filter.name.startsWith path on the wire.
	vars := gqlClient.Requests()[0].Variables
	filter, ok := vars["filter"].(map[string]any)
	if !ok {
		t.Fatalf("filter = %v, want map", vars["filter"])
	}
	name, ok := filter["name"].(map[string]any)
	if !ok {
		t.Fatalf("filter.name = %v, want map", filter["name"])
	}
	if name["startsWith"] != "aws-" {
		t.Errorf("filter.name.startsWith = %v, want aws-", name["startsWith"])
	}
}

func TestList_AutoPaginates(t *testing.T) {
	// Page 1: 2 items + next cursor.
	page1 := gqltest.RespondWithData(map[string]any{
		"ociRepos": map[string]any{
			"cursor": map[string]any{"next": "cursor-page-2"},
			"items": []map[string]any{
				{"id": "aws-rds", "name": "aws-rds"},
				{"id": "aws-s3", "name": "aws-s3"},
			},
		},
	})
	// Page 2: 1 item, no next cursor — terminates the loop.
	page2 := gqltest.RespondWithData(map[string]any{
		"ociRepos": map[string]any{
			"cursor": map[string]any{},
			"items": []map[string]any{
				{"id": "azure-blob", "name": "azure-blob"},
			},
		},
	})
	gqlClient := gqltest.NewClient(page1, page2)

	got, err := types.Collect(newService(gqlClient).Iter(t.Context(), ocirepos.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d repos, want 3 (across two pages)", len(got))
	}

	reqs := gqlClient.Requests()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(reqs))
	}
	// Page 2 must carry the cursor handed back from page 1.
	cursor, ok := reqs[1].Variables["cursor"].(map[string]any)
	if !ok {
		t.Fatalf("page 2 cursor = %v, want map", reqs[1].Variables["cursor"])
	}
	if cursor["next"] != "cursor-page-2" {
		t.Errorf("page 2 cursor.next = %v, want cursor-page-2", cursor["next"])
	}
	if gqlClient.Pending() != 0 {
		t.Errorf("Pending = %d, want 0 (all queued responses consumed)", gqlClient.Pending())
	}
}

func TestCreate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createOciRepo": map[string]any{
				"result": map[string]any{
					"id":           "new-repo",
					"name":         "new-repo",
					"artifactType": "application/vnd.massdriver.bundle.v1+json",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Create(t.Context(), ocirepos.CreateInput{
		ID:           "new-repo",
		ArtifactType: ocirepos.ArtifactTypeBundle,
		Attributes:   map[string]any{"team": "platform"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "new-repo" {
		t.Errorf("ID = %q, want new-repo", got.ID)
	}

	// CreateOciRepoInput.attributes is JSON-scalar-encoded by the genqlient
	// scalars marshaler — verify the input round-trips through that path.
	vars := gqlClient.Requests()[0].Variables
	input, ok := vars["input"].(map[string]any)
	if !ok {
		t.Fatalf("input = %v, want map", vars["input"])
	}
	if input["id"] != "new-repo" {
		t.Errorf("input.id = %v, want new-repo", input["id"])
	}
	if input["artifactType"] != "BUNDLE" {
		t.Errorf("input.artifactType = %v, want BUNDLE", input["artifactType"])
	}
}

func TestCreate_ValidationFailure(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"createOciRepo": map[string]any{
				"result":     nil,
				"successful": false,
				"messages": []map[string]any{
					{"code": "format", "field": "id", "message": "id must be lowercase"},
				},
			},
		}),
	)

	_, err := newService(gqlClient).Create(t.Context(), ocirepos.CreateInput{
		ID:           "INVALID",
		ArtifactType: ocirepos.ArtifactTypeBundle,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	mf, ok := gql.AsMutationFailedError(err)
	if !ok {
		t.Fatalf("expected *gql.MutationFailedError, got %T: %v", err, err)
	}
	if mf.Op != "create oci repo" {
		t.Errorf("Op = %q, want create oci repo", mf.Op)
	}
}

func TestUpdate(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"updateOciRepo": map[string]any{
				"result": map[string]any{
					"id":   "aws-rds",
					"name": "aws-rds",
				},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Update(t.Context(), "aws-rds", ocirepos.UpdateInput{
		Attributes: map[string]any{"team": "data"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.ID != "aws-rds" {
		t.Errorf("ID = %q, want aws-rds", got.ID)
	}
}

func TestDelete(t *testing.T) {
	gqlClient := gqltest.NewClient(
		gqltest.RespondWithData(map[string]any{
			"deleteOciRepo": map[string]any{
				"result":     map[string]any{"id": "aws-rds", "name": "aws-rds"},
				"successful": true,
			},
		}),
	)

	got, err := newService(gqlClient).Delete(t.Context(), "aws-rds")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got.ID != "aws-rds" {
		t.Errorf("ID = %q, want aws-rds", got.ID)
	}
}

// TestTarget covers the non-GraphQL path: the function should produce a
// usable oras.Target wired with the expected registry/repo path, auth
// header, and the PlainHTTP flag toggled appropriately for http vs https.
func TestTarget(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		repo          string
		wantRegistry  string
		wantRepoPath  string
		wantPlainHTTP bool
	}{
		{
			name:          "https",
			url:           "https://registry.example.com",
			repo:          "aws-aurora-postgres",
			wantRegistry:  "registry.example.com",
			wantRepoPath:  "test-org/aws-aurora-postgres",
			wantPlainHTTP: false,
		},
		{
			name:          "http enables PlainHTTP",
			url:           "http://localhost:9000",
			repo:          "local-bundle",
			wantRegistry:  "localhost:9000",
			wantRepoPath:  "test-org/local-bundle",
			wantPlainHTTP: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := ocirepos.New(&client.Client{
				Config: config.Config{
					OrganizationID: "test-org",
					URL:            tt.url,
					Credentials:    config.Credentials{AuthHeaderValue: "Bearer test-token"},
				},
			})
			target, err := svc.Target(tt.repo)
			if err != nil {
				t.Fatalf("Target: %v", err)
			}
			remoteRepo, ok := target.(*remote.Repository)
			if !ok {
				t.Fatalf("target type = %T, want *remote.Repository", target)
			}
			if remoteRepo.Reference.Registry != tt.wantRegistry {
				t.Errorf("registry = %q, want %q", remoteRepo.Reference.Registry, tt.wantRegistry)
			}
			if remoteRepo.Reference.Repository != tt.wantRepoPath {
				t.Errorf("repo path = %q, want %q", remoteRepo.Reference.Repository, tt.wantRepoPath)
			}
			if remoteRepo.PlainHTTP != tt.wantPlainHTTP {
				t.Errorf("PlainHTTP = %v, want %v", remoteRepo.PlainHTTP, tt.wantPlainHTTP)
			}
			if remoteRepo.Client == nil {
				t.Error("expected auth client to be set")
			}
		})
	}
}

func TestTarget_BadURL(t *testing.T) {
	svc := ocirepos.New(&client.Client{
		Config: config.Config{
			OrganizationID: "test-org",
			URL:            "://bad-url",
			Credentials:    config.Credentials{AuthHeaderValue: "Bearer test-token"},
		},
	})
	if _, err := svc.Target("anything"); err == nil {
		t.Error("expected error from malformed URL, got nil")
	}
}
