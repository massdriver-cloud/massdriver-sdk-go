//go:build integration

package policies_test

// Evaluator/Explainer tests are not included — they require a real
// authenticated identity (user/SA) plus a target entity to evaluate
// against, which is too domain-specific to script generically.

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/groups"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/policies"
)

// TestIntegration_Policies_CRUD walks Create → Update → Delete on a
// fresh group fixture. The action "project:view" is assumed to
// exist in the sandbox — it's a built-in Massdriver action present
// on every organization. If a future server release renames or
// removes it, swap to any id from c.Policies.ListActions(ctx).
func TestIntegration_Policies_CRUD(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	// Owning group for the policy.
	groupName := inttest.FixtureName(t, "group")
	group, err := c.Groups.Create(ctx, groups.CreateInput{
		Name:        groupName,
		Description: "Group fixture for policy CRUD test.",
	})
	if err != nil {
		t.Fatalf("Create group: %v", err)
	}
	t.Cleanup(func() {
		if _, err := c.Groups.Delete(ctx, group.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete group fixture %s: %v", group.ID, err)
		}
	})

	// Closed-set conditions exercise the non-nil PolicyConditions
	// path. "md-environment" is a system attribute set on every
	// environment-scoped resource.
	created, err := c.Policies.Create(ctx, group.ID, policies.CreatePolicyInput{
		Effect:  policies.EffectAllow,
		Actions: []string{"project:view"},
		Conditions: policies.PolicyConditions{
			"md-environment": {"production"},
		},
	})
	if err != nil {
		t.Fatalf("Create policy: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Policies.Delete(ctx, created.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to delete policy fixture %s: %v", created.ID, err)
		}
	})
	if created.Effect != string(policies.EffectAllow) {
		t.Errorf("Create effect = %q, want %q", created.Effect, policies.EffectAllow)
	}
	if got, want := created.Conditions["md-environment"], []string{"production"}; len(got) != len(want) || got[0] != want[0] {
		t.Errorf("Create conditions[md-environment] = %v, want %v", got, want)
	}

	// Flip Effect and broaden the closed set; this exercises both
	// the Effect-update branch and the Conditions-replace branch.
	newConditions := policies.PolicyConditions{
		"md-environment": {"production", "staging"},
	}
	updated, err := c.Policies.Update(ctx, created.ID, policies.UpdatePolicyInput{
		Effect:     policies.EffectDeny,
		Actions:    []string{"project:view"},
		Conditions: &newConditions,
	})
	if err != nil {
		t.Fatalf("Update policy: %v", err)
	}
	if updated.Effect != string(policies.EffectDeny) {
		t.Errorf("Update effect = %q, want %q", updated.Effect, policies.EffectDeny)
	}
	if got := updated.Conditions["md-environment"]; len(got) != 2 {
		t.Errorf("Update conditions[md-environment] = %v, want 2 entries", got)
	}

	if err := c.Policies.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete policy: %v", err)
	}
}

// TestIntegration_Policies_ListActions confirms the action catalog
// returns at least one entry. The catalog is server-driven, so we
// don't assert specific ids — only that the call succeeds and
// returns something.
func TestIntegration_Policies_ListActions(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	actions, err := c.Policies.ListActions(ctx)
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if len(actions) == 0 {
		t.Errorf("ListActions returned 0 entries; expected a non-empty catalog")
	}
}

// TestIntegration_Policies_ListEntities confirms the entity catalog
// returns at least one entry.
func TestIntegration_Policies_ListEntities(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	entities, err := c.Policies.ListEntities(ctx)
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(entities) == 0 {
		t.Errorf("ListEntities returned 0 entries; expected a non-empty catalog")
	}
}
