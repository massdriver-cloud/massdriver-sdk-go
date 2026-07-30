//go:build integration

package projects_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/projects"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// projectFixtureID is the project ID used by every integration test
// in this package. Project IDs are capped at 20 characters, which the
// default inttest.FixtureName format exceeds — using a fixed name
// keeps the tests within that limit while remaining grep-able as a
// fixture (`inttest-` prefix). Pre-test cleanup removes any orphan
// from a previous crashed run.
const projectFixtureID = "inttest"

// TestIntegration_Projects_CRUD walks Create → Get → Update → Delete
// against a live API. The fixture is removed in t.Cleanup so a crash
// mid-test still leaves the sandbox in a recoverable state.
func TestIntegration_Projects_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	// Pre-clean: previous crashed run may have left an orphan.
	// Pre-clean is best-effort. The platform surfaces "not found" on a
	// Delete mutation as *gql.MutationFailedError (not gql.ErrNotFound), so
	// we don't try to classify and simply proceed — Create will fail
	// loudly if there's an actual problem.
	_, _ = c.Projects.Delete(ctx, projectFixtureID)

	created, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:          projectFixtureID,
		Name:        "Integration test",
		Description: "Created by SDK integration test; safe to delete.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Projects.Delete(ctx, projectFixtureID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", projectFixtureID, err)
		}
	})
	if created.ID != projectFixtureID {
		t.Errorf("Create returned ID %q, want %q", created.ID, projectFixtureID)
	}

	got, err := c.Projects.Get(ctx, projectFixtureID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Created by SDK integration test; safe to delete." {
		t.Errorf("Get description = %q, want the create-time value", got.Description)
	}

	// NameIn-only filter: the request must omit the empty `eq` (the server
	// ANDs `eq` with `in`, so sending `eq: ""` would match nothing).
	page, err := c.Projects.ListPage(ctx, projects.ListInput{NameIn: []string{got.Name}})
	if err != nil {
		t.Fatalf("ListPage(NameIn): %v", err)
	}
	foundByNameIn := false
	for _, p := range page.Items {
		if p.ID == projectFixtureID {
			foundByNameIn = true
		}
	}
	if !foundByNameIn {
		t.Errorf("ListPage(NameIn: [%q]) did not return the fixture project", got.Name)
	}

	// Partial update: only Description is set, so Name must be left
	// unchanged by the server (nil fields are omitted from the request).
	updated, err := c.Projects.Update(ctx, projectFixtureID, projects.UpdateInput{
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

	if _, err := c.Projects.Delete(ctx, projectFixtureID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Subsequent Get should now return ErrNotFound — the SDK's
	// not-found classification working end-to-end.
	if _, err := c.Projects.Get(ctx, projectFixtureID); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Projects_List confirms List returns at least the
// fixture we created. We don't assert exact counts because the
// sandbox may have other projects.
func TestIntegration_Projects_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	// Pre-clean is best-effort. The platform surfaces "not found" on a
	// Delete mutation as *gql.MutationFailedError (not gql.ErrNotFound), so
	// we don't try to classify and simply proceed — Create will fail
	// loudly if there's an actual problem.
	_, _ = c.Projects.Delete(ctx, projectFixtureID)

	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:   projectFixtureID,
		Name: "List-test fixture",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Projects.Delete(ctx, projectFixtureID)
	})

	all, err := types.Collect(c.Projects.Iter(ctx, projects.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found bool
	for _, p := range all {
		if p.ID == projectFixtureID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List did not contain freshly-created project %s; got %d projects total", projectFixtureID, len(all))
	}
}

// TestIntegration_Projects_NameAndSearchFilters confirms the server-side
// name/search filters narrow the list to the fixture. The fixture gets a
// unique random name so the assertions hold regardless of what else is in
// the sandbox.
func TestIntegration_Projects_NameAndSearchFilters(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, _ = c.Projects.Delete(ctx, projectFixtureID) // best-effort pre-clean

	uniqueName := inttest.FixtureName(t, "search")
	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:   projectFixtureID,
		Name: uniqueName,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Projects.Delete(ctx, projectFixtureID)
	})

	byName, err := types.Collect(c.Projects.Iter(ctx, projects.ListInput{Name: uniqueName}))
	if err != nil {
		t.Fatalf("List by name: %v", err)
	}
	if len(byName) != 1 || byName[0].ID != projectFixtureID {
		t.Errorf("List by name = %+v, want exactly the fixture %s", byName, projectFixtureID)
	}

	bySearch, err := types.Collect(c.Projects.Iter(ctx, projects.ListInput{Search: uniqueName}))
	if err != nil {
		t.Fatalf("List by search: %v", err)
	}
	var found bool
	for _, p := range bySearch {
		if p.ID == projectFixtureID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List by search %q did not contain the fixture; got %d projects", uniqueName, len(bySearch))
	}
}

// TestIntegration_Projects_Clone clones the fixture into a second project
// and confirms the new project exists independently. The source has an
// empty blueprint, so this exercises the mutation wiring rather than
// component copying (covered by the unit test).
func TestIntegration_Projects_Clone(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	const cloneID = "inttestclone"
	_, _ = c.Projects.Delete(ctx, projectFixtureID) // best-effort pre-clean
	_, _ = c.Projects.Delete(ctx, cloneID)

	if _, err := c.Projects.Create(ctx, projects.CreateInput{
		ID:   projectFixtureID,
		Name: "Clone source",
	}); err != nil {
		t.Fatalf("Create source: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Projects.Delete(ctx, projectFixtureID)
	})

	cloned, err := c.Projects.Clone(ctx, projectFixtureID, projects.CloneInput{
		ID:          cloneID,
		Name:        "Clone target",
		Description: "Created by SDK integration test; safe to delete.",
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Projects.Delete(ctx, cloneID)
	})
	if cloned.ID != cloneID {
		t.Errorf("Clone returned ID %q, want %q", cloned.ID, cloneID)
	}

	got, err := c.Projects.Get(ctx, cloneID)
	if err != nil {
		t.Fatalf("Get clone: %v", err)
	}
	if got.Name != "Clone target" {
		t.Errorf("clone Name = %q, want Clone target", got.Name)
	}
}

// TestIntegration_Projects_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
func TestIntegration_Projects_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Projects.Get(ctx, "definitely-not-a-real-project-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
