//go:build integration

package organizations_test

// Organization-level mutations (Update name, RemoveMember, Create)
// are intentionally not exercised here — they affect organization-wide
// state that can't be safely sandboxed. RemoveMember in particular
// could revoke a real user's access. Run those manually when needed.

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/organizations"
)

// TestIntegration_Organizations_Get confirms the configured
// organization's metadata round-trips. The returned ID must match
// the one the client is configured against — anything else means
// the auth/org-scoping plumbing is wrong.
func TestIntegration_Organizations_Get(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	org, err := c.Organizations.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if org.ID != c.Config.OrganizationID {
		t.Errorf("Get returned ID %q, want configured org id %q", org.ID, c.Config.OrganizationID)
	}
}

// TestIntegration_Organizations_CustomAttributes_CRUD walks Create →
// Update → Delete on a custom attribute. The Key is constructed
// manually instead of via inttest.FixtureName because attribute
// keys don't permit dashes — only letters/digits/underscores per
// the server's identifier rules.
func TestIntegration_Organizations_CustomAttributes_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	// FixtureName uses dashes (e.g. "inttest-attr-2f9c1b3a"), which
	// aren't legal in custom-attribute keys. Construct manually with
	// underscore separators.
	key := fmt.Sprintf("inttest_attr_%08x", rand.Uint32())

	created, err := c.Organizations.CreateCustomAttribute(ctx, organizations.CreateCustomAttributeInput{
		Key:    key,
		Scope:  organizations.AttributeScopeProject,
		Values: []string{"alpha", "beta"},
	})
	if err != nil {
		t.Fatalf("CreateCustomAttribute: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Organizations.DeleteCustomAttribute(ctx, created.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete custom attribute fixture %s: %v", created.ID, err)
		}
	})
	if created.Key != key {
		t.Errorf("Create key = %q, want %q", created.Key, key)
	}
	if created.Scope != string(organizations.AttributeScopeProject) {
		t.Errorf("Create scope = %q, want %q", created.Scope, organizations.AttributeScopeProject)
	}

	updated, err := c.Organizations.UpdateCustomAttribute(ctx, created.ID, organizations.UpdateCustomAttributeInput{
		Values: []string{"alpha", "beta", "gamma"},
	})
	if err != nil {
		t.Fatalf("UpdateCustomAttribute: %v", err)
	}
	if len(updated.Values) != 3 {
		t.Errorf("Update values = %v, want 3 entries", updated.Values)
	}

	if _, err := c.Organizations.DeleteCustomAttribute(ctx, created.ID); err != nil {
		t.Fatalf("DeleteCustomAttribute: %v", err)
	}
}
