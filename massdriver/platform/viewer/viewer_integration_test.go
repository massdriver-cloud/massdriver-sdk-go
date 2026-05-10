//go:build integration

package viewer_test

import (
	"context"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/viewer"
)

// TestIntegration_Viewer_Get confirms the "who am I?" endpoint returns
// a populated viewer for the configured credentials. We don't assert
// exact identity (the test runner could be a user or a service
// account), only that the discriminator is one of the two valid
// kinds and the ID is non-empty.
func TestIntegration_Viewer_Get(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := c.Viewer.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID == "" {
		t.Errorf("Viewer.ID is empty; want a non-empty identifier")
	}
	switch got.Kind {
	case viewer.KindAccount, viewer.KindServiceAccount:
		// ok
	default:
		t.Errorf("Viewer.Kind = %q, want one of %q or %q", got.Kind, viewer.KindAccount, viewer.KindServiceAccount)
	}
}
