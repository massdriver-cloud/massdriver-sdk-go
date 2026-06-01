//go:build integration

package auditlogs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/auditlogs"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// TestIntegration_AuditLogs_List confirms List returns without error
// and produces a slice. Audit logs are server-generated, so no fixture
// setup is needed; the result may legitimately be empty for a fresh
// sandbox.
func TestIntegration_AuditLogs_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := types.Collect(c.AuditLogs.Iter(ctx, auditlogs.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_ = len(got)
}

// TestIntegration_AuditLogs_Iter exercises the streaming iterator.
// We bound the iteration to the first 10 items so the test is fast
// even on long audit trails. Any yielded error fails the test.
func TestIntegration_AuditLogs_Iter(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	count := 0
	for _, err := range c.AuditLogs.Iter(ctx, auditlogs.ListInput{}) {
		if err != nil {
			t.Fatalf("Iter yielded error: %v", err)
		}
		count++
		if count >= 10 {
			break
		}
	}
}

// TestIntegration_AuditLogs_ListEventTypes confirms the static
// catalog of event types is non-empty.
func TestIntegration_AuditLogs_ListEventTypes(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := c.AuditLogs.ListEventTypes(ctx)
	if err != nil {
		t.Fatalf("ListEventTypes: %v", err)
	}
	if len(got) == 0 {
		t.Errorf("ListEventTypes returned empty slice; want at least one event type in the catalog")
	}
}

// TestIntegration_AuditLogs_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound. This is the live-API
// counterpart to the unit tests that mock the wire-level nil; here
// the actual server has to produce the not-found signal and the
// classifier has to match it.
func TestIntegration_AuditLogs_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.AuditLogs.Get(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
