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
)

// newProjectFixture creates a parent project for an environment fixture.
// Environments are sub-resources of projects, so every CRUD-style test
// must own a project to scope the environment under. The returned
// project ID is registered for cleanup; callers register their
// environment cleanup separately so a crash mid-test still tears
// everything down in the right order (env first, then project).
func newProjectFixture(t *testing.T, ctx context.Context) string {
	t.Helper()
	c := inttest.Client(t)
	id := inttest.FixtureName(t, "project")
	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:          id,
		Name:        "Integration test parent " + id,
		Description: "Parent project for environments integration test; safe to delete.",
	}); err != nil {
		t.Fatalf("create parent project: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Projects.Delete(ctx, id); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete parent project %s: %v", id, err)
		}
	})
	return id
}

// TestIntegration_Environments_CRUD walks Create → Get → Update → Delete
// for an environment scoped to a freshly-created parent project.
// Each fixture is registered with t.Cleanup so a mid-test crash still
// leaves the sandbox grep-able for orphans.
func TestIntegration_Environments_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	projectID := newProjectFixture(t, ctx)
	envID := inttest.FixtureName(t, "env")

	created, err := c.Environments.Create(ctx, projectID, environments.CreateInput{
		ID:          envID,
		Name:        "Integration test " + envID,
		Description: "Created by SDK integration test; safe to delete.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Environments.Delete(ctx, envID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", envID, err)
		}
	})
	if created.ID != envID {
		t.Errorf("Create returned ID %q, want %q", created.ID, envID)
	}

	got, err := c.Environments.Get(ctx, envID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Created by SDK integration test; safe to delete." {
		t.Errorf("Get description = %q, want the create-time value", got.Description)
	}

	updated, err := c.Environments.Update(ctx, envID, environments.UpdateInput{
		Name:        "Integration test (renamed) " + envID,
		Description: "Updated by SDK integration test.",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "Updated by SDK integration test." {
		t.Errorf("Update description = %q, want the updated value", updated.Description)
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
	envID := inttest.FixtureName(t, "env")

	if _, err := c.Environments.Create(ctx, projectID, environments.CreateInput{
		ID:   envID,
		Name: "List-test fixture " + envID,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Environments.Delete(ctx, envID)
	})

	all, err := c.Environments.List(ctx)
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
