//go:build integration

package resourcetypes_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
)

// TestIntegration_ResourceTypes_Get fetches a public Massdriver-provided
// resource type, which every organization can see.
func TestIntegration_ResourceTypes_Get(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	rt, err := c.ResourceTypes.Get(ctx, "aws-iam-role")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rt.ID != "aws-iam-role" {
		t.Errorf("ID = %q, want aws-iam-role", rt.ID)
	}
	if rt.Name == "" {
		t.Errorf("Name is empty; want a display name")
	}
	if len(rt.Schema) == 0 {
		t.Errorf("Schema is empty; want the type's JSON Schema")
	}
}

// TestIntegration_ResourceTypes_NotFoundClassification confirms Get for a
// non-existent ID returns ErrNotFound — the live-API counterpart to the
// unit test that mocks the wire-level nil.
func TestIntegration_ResourceTypes_NotFoundClassification(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	_, err := c.ResourceTypes.Get(ctx, "definitely-not-a-real-resource-type-12345")
	if !errors.Is(err, gql.ErrNotFound) {
		t.Errorf("Get nonexistent: got %v, want errors.Is(err, gql.ErrNotFound)", err)
	}
}
