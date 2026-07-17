//go:build integration

package bundles_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
)

// TestIntegration_Bundles_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
func TestIntegration_Bundles_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.Bundles.Get(ctx, "definitely-not-a-real-bundle-12345@1.0.0")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
