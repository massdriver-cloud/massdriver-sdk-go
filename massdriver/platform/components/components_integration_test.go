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
// from. The sandbox must have a published bundle in this repo.
const sandboxOCIRepoName = "sdk-integration-test"

// projectFixtureID is the parent project for component fixtures.
// Fixed short name (project IDs are capped at 20 chars); different
// per package so parallel package runs don't collide.
const projectFixtureID = "inttestcomp"

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

// TestIntegration_Components_CRUD walks Add → Get → Update → Remove
// for a component scoped to a freshly-created parent project.
func TestIntegration_Components_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	projectID := newProjectFixture(t, ctx)

	added, err := c.Components.Add(ctx, projectID, components.AddInput{
		OciRepoName: sandboxOCIRepoName,
		ID:          "inttestcomp",
		Name:        "Integration test component",
		Description: "Created by SDK integration test; safe to delete.",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	compID := added.ID
	t.Cleanup(func() {
		_, _ = c.Components.Remove(ctx, compID)
	})

	got, err := c.Components.Get(ctx, compID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Created by SDK integration test; safe to delete." {
		t.Errorf("Get description = %q, want the create-time value", got.Description)
	}

	updated, err := c.Components.Update(ctx, compID, components.UpdateInput{
		Name:        "Integration test (renamed)",
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

	if _, err := c.Components.Get(ctx, compID); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Remove: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Components_NotFoundClassification confirms Get for a
// nonexistent ID returns ErrNotFound.
func TestIntegration_Components_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Components.Get(ctx, "definitely-not-a-real-component-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
