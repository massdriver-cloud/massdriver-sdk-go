//go:build integration

package ocirepos_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/ocirepos"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
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

	got, err := types.Collect(c.OciRepos.Iter(ctx, ocirepos.ListInput{}))
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

// TestIntegration_OciRepoGrants walks the full grant lifecycle on a
// fixture repository: create (wildcard + attribute-conditioned) → list →
// paginate → delete → list empty. Grants are immutable so there is no
// update leg.
func TestIntegration_OciRepoGrants(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	id := inttest.FixtureName(t, "ocirepo")

	if _, err := c.OciRepos.Create(ctx, ocirepos.CreateInput{
		ID:           id,
		ArtifactType: ocirepos.ArtifactTypeBundle,
		Attributes: map[string]any{
			"created-by": "sdk-integration-test",
		},
	}); err != nil {
		t.Fatalf("Create repo: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.OciRepos.Delete(ctx, id); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", id, err)
		}
	})

	wildcard, err := c.OciRepos.CreateGrant(ctx, id, ocirepos.CreateGrantInput{
		Action:              "repo:pull",
		RecipientConditions: nil, // wildcard
	})
	if err != nil {
		t.Fatalf("CreateGrant (wildcard): %v", err)
	}
	t.Cleanup(func() {
		if err := c.OciRepos.DeleteGrant(ctx, wildcard.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete grant %s: %v", wildcard.ID, err)
		}
	})
	if wildcard.RecipientConditions != nil {
		t.Errorf("wildcard grant RecipientConditions = %v, want nil", wildcard.RecipientConditions)
	}

	conditioned, err := c.OciRepos.CreateGrant(ctx, id, ocirepos.CreateGrantInput{
		Action: "repo:pull",
		RecipientConditions: types.PolicyConditions{
			"created-by": []string{"sdk-integration-test"},
		},
	})
	if err != nil {
		t.Fatalf("CreateGrant (conditioned): %v", err)
	}
	t.Cleanup(func() {
		if err := c.OciRepos.DeleteGrant(ctx, conditioned.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete grant %s: %v", conditioned.ID, err)
		}
	})

	got, err := types.Collect(c.OciRepos.IterGrants(ctx, id, ocirepos.ListGrantsInput{}))
	if err != nil {
		t.Fatalf("IterGrants: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d grants, want 2", len(got))
	}
	found := map[string]bool{}
	for _, g := range got {
		found[g.ID] = true
	}
	if !found[wildcard.ID] || !found[conditioned.ID] {
		t.Errorf("IterGrants returned %v, want both %s and %s", found, wildcard.ID, conditioned.ID)
	}

	// A one-item page over two grants must hand back a next cursor.
	page, err := c.OciRepos.ListGrantsPage(ctx, id, ocirepos.ListGrantsInput{PageSize: 1})
	if err != nil {
		t.Fatalf("ListGrantsPage: %v", err)
	}
	if len(page.Items) != 1 {
		t.Errorf("page has %d items, want 1", len(page.Items))
	}
	if page.Next == "" {
		t.Error("page.Next is empty, want a cursor to the second page")
	}

	for _, grantID := range []string{wildcard.ID, conditioned.ID} {
		if err := c.OciRepos.DeleteGrant(ctx, grantID); err != nil {
			t.Fatalf("DeleteGrant %s: %v", grantID, err)
		}
	}

	got, err = types.Collect(c.OciRepos.IterGrants(ctx, id, ocirepos.ListGrantsInput{}))
	if err != nil {
		t.Fatalf("IterGrants after delete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d grants after delete, want 0", len(got))
	}

	// Listing grants on a nonexistent repo classifies as not-found.
	_, err = types.Collect(c.OciRepos.IterGrants(ctx, "definitely-not-a-real-ocirepo-12345", ocirepos.ListGrantsInput{}))
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("IterGrants nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
