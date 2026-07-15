//go:build integration

package resources_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/resources"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// TestIntegration_Resources_CRUD_Imported walks Create → Get → Update →
// Delete on an imported resource. Provisioned resources are owned by
// their producing instance's lifecycle and can't be exercised this way;
// the imported path is the only direct CRUD surface.
//
// Assumes the sandbox has the resource type "aws-iam-role" available.
func TestIntegration_Resources_CRUD_Imported(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "resource")

	created, err := c.Resources.Create(ctx, "aws-iam-role", resources.CreateInput{
		Name: name,
		Payload: map[string]any{
			"arn":         "arn:aws:iam::123456789012:role/inttest",
			"external_id": "00000000-0000-0000-0000-000000000000",
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	id := created.ID
	t.Cleanup(func() {
		if _, err := c.Resources.Delete(ctx, id); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", id, err)
		}
	})

	got, err := c.Resources.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != id {
		t.Errorf("Get returned ID %q, want %q", got.ID, id)
	}

	updatedName := name + "-renamed"
	updated, err := c.Resources.Update(ctx, id, resources.UpdateInput{
		Name: updatedName,
		Payload: map[string]any{
			"arn":         "arn:aws:iam::123456789012:role/inttest-renamed",
			"external_id": "11111111-1111-1111-1111-111111111111",
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != updatedName {
		t.Errorf("Update name = %q, want %q", updated.Name, updatedName)
	}

	if _, err := c.Resources.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Subsequent Get should now return ErrNotFound — the SDK's
	// not-found classification working end-to-end.
	if _, err := c.Resources.Get(ctx, id); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Resources_List confirms List returns without error
// and produces a slice. We don't assert exact counts because the sandbox
// state is unknown — the result may legitimately be empty.
func TestIntegration_Resources_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := types.Collect(c.Resources.Iter(ctx, resources.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_ = len(got)
}

// TestIntegration_Resources_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound. This is the live-API counterpart
// to the unit tests that mock the wire-level nil; here the actual server
// has to produce the not-found signal and the classifier has to match it.
func TestIntegration_Resources_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Resources.Get(ctx, "definitely-not-a-real-resource-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_ResourceGrants walks the grant lifecycle on an imported
// resource fixture: create → list → delete → list empty. Grants are
// immutable so there is no update leg.
//
// Assumes the sandbox has the resource type "aws-iam-role" available.
func TestIntegration_ResourceGrants(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "resource")

	created, err := c.Resources.Create(ctx, "aws-iam-role", resources.CreateInput{
		Name: name,
		Payload: map[string]any{
			"arn":         "arn:aws:iam::123456789012:role/inttest",
			"external_id": "00000000-0000-0000-0000-000000000000",
		},
	})
	if err != nil {
		t.Fatalf("Create resource: %v", err)
	}
	id := created.ID
	t.Cleanup(func() {
		if _, err := c.Resources.Delete(ctx, id); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", id, err)
		}
	})

	grant, err := c.Resources.CreateGrant(ctx, id, resources.CreateGrantInput{
		Action:              "resource:export",
		RecipientConditions: nil, // wildcard
	})
	if err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Resources.DeleteGrant(ctx, grant.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete grant %s: %v", grant.ID, err)
		}
	})

	got, err := types.Collect(c.Resources.IterGrants(ctx, id, resources.ListGrantsInput{}))
	if err != nil {
		t.Fatalf("IterGrants: %v", err)
	}
	if len(got) != 1 || got[0].ID != grant.ID {
		t.Fatalf("IterGrants = %v, want exactly the created grant %s", got, grant.ID)
	}
	if got[0].RecipientConditions != nil {
		t.Errorf("grant RecipientConditions = %v, want nil (wildcard)", got[0].RecipientConditions)
	}

	if err := c.Resources.DeleteGrant(ctx, grant.ID); err != nil {
		t.Fatalf("DeleteGrant: %v", err)
	}

	got, err = types.Collect(c.Resources.IterGrants(ctx, id, resources.ListGrantsInput{}))
	if err != nil {
		t.Fatalf("IterGrants after delete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d grants after delete, want 0", len(got))
	}
}
