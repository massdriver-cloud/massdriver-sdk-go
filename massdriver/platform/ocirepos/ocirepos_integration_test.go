//go:build integration

package ocirepos_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/ocirepos"
)

// TestIntegration_OciRepos_CRUD walks Create → Get → Update → Delete
// against a live API. Every fixture is created with the inttest-prefix
// and removed in t.Cleanup so a crash mid-test still leaves the sandbox
// grep-able for orphans.
//
// OCI repo IDs are constrained to lowercase letters, numbers, dashes,
// and underscores — FixtureName output is compatible.
func TestIntegration_OciRepos_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	id := inttest.FixtureName(t, "ocirepo")

	created, err := c.OciRepos.Create(ctx, ocirepos.CreateInput{
		ID:           id,
		ArtifactType: ocirepos.ArtifactTypeBundle,
		Attributes: map[string]any{
			"created-by": "sdk-integration-test",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.OciRepos.Delete(ctx, id); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", id, err)
		}
	})
	if created.ID != id {
		t.Errorf("Create returned ID %q, want %q", created.ID, id)
	}

	got, err := c.OciRepos.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != id {
		t.Errorf("Get returned ID %q, want %q", got.ID, id)
	}

	if _, err := c.OciRepos.Update(ctx, id, ocirepos.UpdateInput{
		Attributes: map[string]any{
			"created-by": "sdk-integration-test",
			"updated":    "true",
		},
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if _, err := c.OciRepos.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Subsequent Get should now return ErrNotFound — the SDK's
	// not-found classification working end-to-end.
	if _, err := c.OciRepos.Get(ctx, id); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_OciRepos_List confirms List returns without error
// and produces a slice. We don't assert exact counts because the sandbox
// state is unknown — the result may legitimately be empty.
func TestIntegration_OciRepos_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := c.OciRepos.List(ctx, ocirepos.ListInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_ = len(got)
}

// TestIntegration_OciRepos_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound. This is the live-API counterpart
// to the unit tests that mock the wire-level nil; here the actual server
// has to produce the not-found signal and the classifier has to match it.
func TestIntegration_OciRepos_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.OciRepos.Get(ctx, "definitely-not-a-real-ocirepo-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
