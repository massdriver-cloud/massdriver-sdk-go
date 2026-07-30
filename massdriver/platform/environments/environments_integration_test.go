//go:build integration

package environments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/environments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// projectFixtureID is the parent project for environment fixtures.
// Fixed short name (project IDs are capped at 20 chars); different
// per package so parallel package runs don't collide.
const projectFixtureID = "inttestenv"

func newProjectFixture(t *testing.T, ctx context.Context) string {
	t.Helper()
	c := inttest.Client(t)
	_, _ = c.Projects.Delete(ctx, projectFixtureID) // best-effort pre-clean
	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:   projectFixtureID,
		Name: "Integration test parent",
	}); err != nil {
		t.Fatalf("create parent project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Projects.Delete(ctx, projectFixtureID)
	})
	return projectFixtureID
}

// TestIntegration_Environments_CRUD walks Create → Get → Update → Delete
// for an environment scoped to a freshly-created parent project.
// Each fixture is registered with t.Cleanup so a mid-test crash still
// leaves the sandbox grep-able for orphans.
func TestIntegration_Environments_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	projectID := newProjectFixture(t, ctx)

	created, err := c.Environments.Create(ctx, projectID, environments.CreateInput{
		ID:          "inttestenv",
		Name:        "Integration test env",
		Description: "Created by SDK integration test; safe to delete.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	envID := created.ID
	t.Cleanup(func() {
		_, _ = c.Environments.Delete(ctx, envID)
	})

	got, err := c.Environments.Get(ctx, envID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Created by SDK integration test; safe to delete." {
		t.Errorf("Get description = %q, want the create-time value", got.Description)
	}

	// Partial update: only Description is set, so Name must be left
	// unchanged by the server (nil fields are omitted from the request).
	updated, err := c.Environments.Update(ctx, envID, environments.UpdateInput{
		Description: types.Ptr("Updated by SDK integration test."),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "Updated by SDK integration test." {
		t.Errorf("Update description = %q, want the updated value", updated.Description)
	}
	if updated.Name != got.Name {
		t.Errorf("Update name = %q, want unchanged %q", updated.Name, got.Name)
	}

	if _, err := c.Environments.Delete(ctx, envID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Subsequent Get should now return ErrNotFound — the SDK's
	// not-found classification working end-to-end.
	if _, err := c.Environments.Get(ctx, envID); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Environments_List confirms List returns at least the
// fixture we created. We don't assert exact counts because the
// sandbox may have other environments. The SDK's List call has no
// project filter — it returns every environment in the configured
// organization — so we scan for our fixture by ID.
func TestIntegration_Environments_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	projectID := newProjectFixture(t, ctx)

	created, err := c.Environments.Create(ctx, projectID, environments.CreateInput{
		ID:   "inttestenv",
		Name: "List-test fixture",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	envID := created.ID
	t.Cleanup(func() {
		_, _ = c.Environments.Delete(ctx, envID)
	})

	all, err := types.Collect(c.Environments.Iter(ctx, environments.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found bool
	for _, e := range all {
		if e.ID == envID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List did not contain freshly-created environment %s; got %d environments total", envID, len(all))
	}
}

// TestIntegration_Environments_Compare diffs two freshly-created sibling
// environments. The parent project has no components, so the comparison's
// instance list is empty — the test exercises the query wiring and the
// same-project pairing end-to-end; per-instance diffing is covered by the
// unit test.
func TestIntegration_Environments_Compare(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	projectID := newProjectFixture(t, ctx)

	source, err := c.Environments.Create(ctx, projectID, environments.CreateInput{
		ID:   "cmpsrc",
		Name: "Compare source",
	})
	if err != nil {
		t.Fatalf("Create source env: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Environments.Delete(ctx, source.ID)
	})

	target, err := c.Environments.Create(ctx, projectID, environments.CreateInput{
		ID:   "cmptgt",
		Name: "Compare target",
	})
	if err != nil {
		t.Fatalf("Create target env: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Environments.Delete(ctx, target.ID)
	})

	cmp, err := c.Environments.Compare(ctx, source.ID, target.ID)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if cmp.Source.ID != source.ID || cmp.Target.ID != target.ID {
		t.Errorf("Compare Source/Target IDs = %q/%q, want %q/%q", cmp.Source.ID, cmp.Target.ID, source.ID, target.ID)
	}
	if len(cmp.Instances) != 0 {
		t.Errorf("Instances len = %d, want 0 for a component-less project", len(cmp.Instances))
	}
}

// TestIntegration_Environments_NotFoundClassification confirms Get for
// a non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
func TestIntegration_Environments_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Environments.Get(ctx, "definitely-not-a-real-environment-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
