//go:build integration

package serviceaccounts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/serviceaccounts"
)

// TestIntegration_ServiceAccounts_CRUD walks Create → Get → Update →
// Delete against a live API. Service accounts are an auth-sensitive
// surface: a careless Delete on an existing identity could break the
// test runner. Every fixture is created with the inttest- prefix and
// removed in t.Cleanup so a crash mid-test still leaves the sandbox
// grep-able for orphans.
func TestIntegration_ServiceAccounts_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "sa")

	created, err := c.ServiceAccounts.Create(ctx, serviceaccounts.CreateInput{
		Name:                                  name,
		Description:                           "Created by SDK integration test; safe to delete.",
		DefaultAccessTokenExpirationInMinutes: 60,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	id := created.ID
	t.Cleanup(func() {
		if _, err := c.ServiceAccounts.Delete(ctx, id); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete fixture %s: %v", id, err)
		}
	})
	if created.Name != name {
		t.Errorf("Create returned Name %q, want %q", created.Name, name)
	}
	if created.DefaultToken == "" {
		t.Errorf("Create returned empty DefaultToken; want a non-empty bearer string")
	}

	got, err := c.ServiceAccounts.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Description != "Created by SDK integration test; safe to delete." {
		t.Errorf("Get description = %q, want the create-time value", got.Description)
	}

	updated, err := c.ServiceAccounts.Update(ctx, id, serviceaccounts.UpdateInput{
		Name:        name + "-renamed",
		Description: "Updated by SDK integration test.",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != name+"-renamed" {
		t.Errorf("Update name = %q, want %q", updated.Name, name+"-renamed")
	}

	if _, err := c.ServiceAccounts.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Subsequent Get should now return ErrNotFound — the SDK's
	// not-found classification working end-to-end.
	if _, err := c.ServiceAccounts.Get(ctx, id); !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get after Delete: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_ServiceAccounts_List confirms List returns without
// error and produces a slice. We don't assert exact counts because the
// sandbox state is unknown.
func TestIntegration_ServiceAccounts_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := c.ServiceAccounts.List(ctx, serviceaccounts.ListInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_ = len(got)
}

// TestIntegration_ServiceAccounts_NotFoundClassification confirms Get
// for a non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
func TestIntegration_ServiceAccounts_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.ServiceAccounts.Get(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
