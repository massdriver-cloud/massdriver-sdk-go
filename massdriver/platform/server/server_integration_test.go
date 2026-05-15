//go:build integration

package server_test

import (
	"context"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
)

// TestIntegration_Server_Get confirms the server-metadata endpoint
// returns a populated record. The endpoint does not require
// authentication on the server side, but our SDK clients always go
// through the auth path — Client(t) supplies real credentials so the
// transport behaves identically to authenticated traffic.
func TestIntegration_Server_Get(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := c.Server.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Version == "" {
		t.Errorf("Server.Version is empty; want a non-empty semver string")
	}
	// ServerMode enum values per schema: SELF_HOSTED, MANAGED.
	switch got.Mode {
	case "SELF_HOSTED", "MANAGED":
		// ok
	default:
		t.Errorf("Server.Mode = %q, want one of %q or %q", got.Mode, "SELF_HOSTED", "MANAGED")
	}
}
