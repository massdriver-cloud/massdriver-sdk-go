//go:build integration

// Alarm CRUD requires a live instance with a registered alarm; that
// path is exercised end-to-end by the cross-package scenario test
// (massdriver/scenario_integration_test.go) which creates an instance
// via deployment first.

package instances_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/instances"
)

// TestIntegration_Instances_NotFoundClassification confirms Get for
// a non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
//
// Note: instances are not directly creatable by SDK callers — they
// appear when a deployment runs against a project blueprint. So the
// CRUD shape doesn't apply here; we exercise only the read paths.
func TestIntegration_Instances_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Instances.Get(ctx, "definitely-not-a-real-instance-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Instances_List exercises the unfiltered list path.
// Because we can't deterministically create an instance from inside
// the SDK (they're created by the deployment system, not by callers),
// we don't assert specific contents — only that the call returns
// without error and produces a slice (possibly empty).
func TestIntegration_Instances_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := c.Instances.List(ctx, instances.ListInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// We deliberately don't assert len(got) > 0 — a fresh sandbox may
	// have zero deployed instances. A non-nil error and a (possibly
	// empty) slice is the correct shape.
	_ = got
}

// TestIntegration_Instances_Iter exercises the streaming iterator
// against the live API. Same caveat as List: we can't guarantee any
// instances exist, so we only assert that no errors are yielded.
// We break early after 5 items to keep the test fast on sandboxes
// with many deployed instances.
func TestIntegration_Instances_Iter(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	const maxItems = 5
	seen := 0
	for _, err := range c.Instances.Iter(ctx, instances.ListInput{}) {
		if err != nil {
			t.Fatalf("Iter yielded error: %v", err)
		}
		seen++
		if seen >= maxItems {
			break
		}
	}
	// seen may be zero on an empty sandbox — that's fine.
}
