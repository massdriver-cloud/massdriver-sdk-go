//go:build integration

package groups_test

// Member-management tests (AddUser, AddServiceAccount, etc.) are not
// included here — they require knowing a real user email or service
// account ID that's safe to mutate. Run those manually if needed.

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/groups"
)

// TestIntegration_Groups_CRUD walks Create → Get → Update → Delete
// against a live API. Built-in groups (Admins, Viewers) are not
// touched — only a freshly-minted custom group, removed in
// t.Cleanup so a crash mid-test still leaves the sandbox grep-able
// for orphans.
func TestIntegration_Groups_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "group")

	created, err := c.Groups.Create(ctx, groups.CreateInput{
		Name:        name,
		Description: "Created by SDK integration test; safe to delete.",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Groups.Delete(ctx, created.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", created.ID, err)
		}
	})
	if created.Name != name {
		t.Errorf("Create returned Name %q, want %q", created.Name, name)
	}

	got, err := c.Groups.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Created by SDK integration test; safe to delete." {
		t.Errorf("Get description = %q, want the create-time value", got.Description)
	}

	renamed := name + "-renamed"
	updated, err := c.Groups.Update(ctx, created.ID, groups.UpdateInput{
		Name:        renamed,
		Description: "Updated by SDK integration test.",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != renamed {
		t.Errorf("Update name = %q, want %q", updated.Name, renamed)
	}
	if updated.Description != "Updated by SDK integration test." {
		t.Errorf("Update description = %q, want the updated value", updated.Description)
	}

	if _, err := c.Groups.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Subsequent Get should now return ErrNotFound — the SDK's
	// not-found classification working end-to-end.
	if _, err := c.Groups.Get(ctx, created.ID); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Groups_List confirms List returns at least the
// fixture we created. We don't assert exact counts because the
// sandbox always has the built-in Admins/Viewers groups plus
// whatever else lives there.
func TestIntegration_Groups_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "group")
	created, err := c.Groups.Create(ctx, groups.CreateInput{
		Name: name,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Groups.Delete(ctx, created.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", created.ID, err)
		}
	})

	all, err := c.Groups.List(ctx, groups.ListInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found bool
	for _, g := range all {
		if g.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("List did not contain freshly-created group %s; got %d groups total", created.ID, len(all))
	}
}

// TestIntegration_Groups_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
func TestIntegration_Groups_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Groups.Get(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
