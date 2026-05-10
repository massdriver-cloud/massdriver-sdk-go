//go:build integration

package components_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/components"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
)

// sandboxOCIRepoName is the OCI repo Add tests source their component
// from. Components require an existing bundle in the configured org's
// OCI registry; we can't easily push one inside the test, so we
// assume a well-known bundle is already present — convention is to
// publish a minimal bundle to "sdk-integration-test" specifically
// for SDK integration testing.
const sandboxOCIRepoName = "sdk-integration-test"

// newProjectFixture creates a parent project for a component fixture.
// Components are part of a project's blueprint, so every CRUD-style
// test must own a project to add the component to. The returned
// project ID is registered for cleanup; callers register their
// component cleanup separately so a crash mid-test still tears
// everything down in the right order (component first, then project).
func newProjectFixture(t *testing.T, ctx context.Context) string {
	t.Helper()
	c := inttest.Client(t)
	id := inttest.FixtureName(t, "project")
	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:          id,
		Name:        "Integration test parent " + id,
		Description: "Parent project for components integration test; safe to delete.",
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

// TestIntegration_Components_CRUD walks Add → Get → Update → Remove
// for a component scoped to a freshly-created parent project.
// Each fixture is registered with t.Cleanup so a mid-test crash still
// leaves the sandbox grep-able for orphans.
//
// Assumes the sandbox has an OCI repo named "sdk-integration-test"
// — see [sandboxOCIRepoName].
func TestIntegration_Components_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	projectID := newProjectFixture(t, ctx)
	compID := inttest.FixtureName(t, "comp")

	added, err := c.Components.Add(ctx, projectID, sandboxOCIRepoName, components.AddInput{
		ID:          compID,
		Name:        "Integration test " + compID,
		Description: "Created by SDK integration test; safe to delete.",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Components.Remove(ctx, compID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to remove fixture %s: %v", compID, err)
		}
	})
	if added.ID != compID {
		t.Errorf("Add returned ID %q, want %q", added.ID, compID)
	}

	got, err := c.Components.Get(ctx, compID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Created by SDK integration test; safe to delete." {
		t.Errorf("Get description = %q, want the create-time value", got.Description)
	}

	updated, err := c.Components.Update(ctx, compID, components.UpdateInput{
		Name:        "Integration test (renamed) " + compID,
		Description: "Updated by SDK integration test.",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "Updated by SDK integration test." {
		t.Errorf("Update description = %q, want the updated value", updated.Description)
	}

	if _, err := c.Components.Remove(ctx, compID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Subsequent Get should now return ErrNotFound — the SDK's
	// not-found classification working end-to-end.
	if _, err := c.Components.Get(ctx, compID); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Remove: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Components_NotFoundClassification confirms Get for
// a non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
func TestIntegration_Components_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Components.Get(ctx, "definitely-not-a-real-component-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
