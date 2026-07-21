//go:build integration

package deployments_test

// Mutation tests (Create/Propose/Approve/Reject/Abort, and the
// Plan/Rollback pair, which additionally need an existing deployment
// history) require a live instance and are exercised by the
// cross-package scenario test in massdriver/test/scenario_integration_test.go.

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/deployments"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// TestIntegration_Deployments_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound. This is the live-API counterpart
// to the unit tests that mock the wire-level nil; here the actual server
// has to produce the not-found signal and the classifier has to match it.
func TestIntegration_Deployments_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	// Deployment IDs are UUIDs server-side; passing a non-UUID string
	// trips the GraphQL input validator before the not-found check
	// runs, masking the signal we're trying to test. Use the all-zero
	// UUID — well-formed, structurally valid, and never a real id.
	_, err := c.Deployments.Get(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Deployments_List confirms List returns without error
// and produces a slice. We don't assert exact counts because the sandbox
// state is unknown — the result may legitimately be empty.
func TestIntegration_Deployments_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := types.Collect(c.Deployments.Iter(ctx, deployments.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// got is []Deployment; nil and empty are both acceptable. Touch len
	// so the variable isn't unused if the compiler ever gets stricter.
	_ = len(got)
}

// TestIntegration_Deployments_Iter confirms Iter produces deployments
// without error. We break after 5 items to keep the test bounded — the
// sandbox may have a large history.
func TestIntegration_Deployments_Iter(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	count := 0
	for _, err := range c.Deployments.Iter(ctx, deployments.ListInput{}) {
		if err != nil {
			t.Fatalf("Iter: %v", err)
		}
		count++
		if count >= 5 {
			break
		}
	}
}

// TestIntegration_Deployments_GetLogs_NotFound confirms GetLogs on a
// non-existent deployment returns ErrNotFound. The not-found classifier
// has to match the server's signal here, same as Get.
func TestIntegration_Deployments_GetLogs_NotFound(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Deployments.GetLogs(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("GetLogs nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}

// TestIntegration_Deployments_Compare_NotFound confirms Compare with
// unknown deployment IDs surfaces ErrNotFound. Comparing real deployments
// requires a live deployment history and is covered by the unit test.
func TestIntegration_Deployments_Compare_NotFound(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Deployments.Compare(ctx,
		"00000000-0000-0000-0000-000000000000",
		"00000000-0000-0000-0000-000000000001",
	)
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Compare nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
