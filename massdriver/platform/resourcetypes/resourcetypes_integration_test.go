//go:build integration

package resourcetypes_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/environments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
)

// TestIntegration_ResourceTypes_Get fetches a public Massdriver-provided
// resource type, which every organization can see.
func TestIntegration_ResourceTypes_Get(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	rt, err := c.ResourceTypes.Get(ctx, "aws-iam-role")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.HasPrefix(rt.ID, "aws-iam-role@") {
		t.Errorf("ID = %q, want aws-iam-role@<resolved version>", rt.ID)
	}
	if rt.Version == "" {
		t.Errorf("Version is empty; want the resolved semver")
	}
	if rt.Name == "" {
		t.Errorf("Name is empty; want a display name")
	}
	if len(rt.Schema) == 0 {
		t.Errorf("Schema is empty; want the type's JSON Schema")
	}
}

// TestIntegration_ResourceTypes_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound — the live-API counterpart to the
// unit test that mocks the wire-level nil.
func TestIntegration_ResourceTypes_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.ResourceTypes.Get(ctx, "definitely-not-a-real-resource-type-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_ResourceTypes_GetChannel confirms release-channel
// resolution: `@latest` must resolve to the same fully-versioned document
// a bare identifier does.
func TestIntegration_ResourceTypes_GetChannel(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	rt, err := c.ResourceTypes.Get(ctx, "aws-iam-role@latest")
	if err != nil {
		t.Fatalf("Get @latest: %v", err)
	}
	if !strings.HasPrefix(rt.ID, "aws-iam-role@") || rt.Version == "" {
		t.Errorf("Get @latest: ID = %q Version = %q, want a resolved composite ID and version", rt.ID, rt.Version)
	}
}

// TestIntegration_ResourceTypes_Dependents exercises the dependents query
// end-to-end against a fresh, empty environment: the query must be accepted
// by the server and decode to an empty (not error) result when nothing in
// the environment depends on the type.
func TestIntegration_ResourceTypes_Dependents(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, _ = c.Projects.Delete(ctx, "inttestrtdep") // best-effort pre-clean
	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:   "inttestrtdep",
		Name: "Integration test rt dependents",
	}); err != nil {
		t.Fatalf("create parent project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Projects.Delete(ctx, "inttestrtdep")
	})
	env, err := c.Environments.Create(ctx, "inttestrtdep", environments.CreateInput{
		ID:   "inttestrtdep",
		Name: "Integration test rt dependents env",
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Environments.Delete(ctx, env.ID)
	})

	deps, err := c.ResourceTypes.Dependents(ctx, env.ID, "aws-iam-role")
	if err != nil {
		t.Fatalf("Dependents: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("Dependents in empty environment = %d entries, want 0", len(deps))
	}
}
