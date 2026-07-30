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
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/policies"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// TestIntegration_Groups_ListAdminGroupMembers confirms the membership
// listing resolves. The built-in Organization Admin group is used because it
// exists on every organization and always has at least one member (someone
// must administer the org).
func TestIntegration_Groups_ListAdminGroupMembers(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	var admin *groups.Group
	for g, err := range c.Groups.Iter(ctx, groups.ListInput{}) {
		if err != nil {
			t.Fatalf("Iter: %v", err)
		}
		if g.Role == string(groups.RoleOrganizationAdmin) {
			admin = &g
			break
		}
	}
	if admin == nil {
		t.Fatal("no ORGANIZATION_ADMIN group found; every organization should have one")
	}

	page, err := c.Groups.ListMembersPage(ctx, admin.ID, groups.ListMembersInput{})
	if err != nil {
		t.Fatalf("ListMembersPage: %v", err)
	}
	if len(page.Items) == 0 {
		t.Errorf("ListMembersPage on the admin group is empty; want at least one member")
	}
}

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

	// A fresh custom group has no policies; attach one and confirm the
	// policies listing returns it.
	pol, err := c.Policies.Create(ctx, created.ID, policies.CreatePolicyInput{
		Effect:  policies.EffectAllow,
		Actions: []string{"project:view"},
	})
	if err != nil {
		t.Fatalf("Policies.Create: %v", err)
	}
	polPage, err := c.Groups.ListPoliciesPage(ctx, created.ID, groups.ListPoliciesInput{})
	if err != nil {
		t.Fatalf("ListPoliciesPage: %v", err)
	}
	foundPolicy := false
	for _, p := range polPage.Items {
		if p.ID == pol.ID {
			foundPolicy = true
		}
	}
	if !foundPolicy {
		t.Errorf("ListPoliciesPage = %+v, want it to include policy %s", polPage.Items, pol.ID)
	}

	// Invitations listing is admin-gated; the sandbox token is an admin, so
	// this must succeed (a fresh group simply has none pending).
	invs, err := c.Groups.ListInvitationsPage(ctx, created.ID, groups.ListInvitationsInput{})
	if err != nil {
		t.Fatalf("ListInvitationsPage: %v", err)
	}
	if len(invs.Items) != 0 {
		t.Errorf("ListInvitationsPage on a fresh group = %+v, want no pending invitations", invs.Items)
	}

	if _, err := c.Policies.Delete(ctx, pol.ID); err != nil {
		t.Fatalf("Policies.Delete: %v", err)
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

	all, err := types.Collect(c.Groups.Iter(ctx, groups.ListInput{}))
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
