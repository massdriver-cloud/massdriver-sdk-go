//go:build integration

package accesstokens_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/accesstokens"
)

// TestIntegration_AccessTokens_CreateAndRevoke creates a fresh PAT and
// revokes it. Personal access tokens are an auth-sensitive surface:
// revoking the wrong token could lock the test runner out, so this
// test only revokes the token it just created — never anything from
// List.
func TestIntegration_AccessTokens_CreateAndRevoke(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "pat")

	created, err := c.AccessTokens.Create(ctx, accesstokens.CreateInput{
		Name:             name,
		Scopes:           []string{"*"},
		ExpiresInMinutes: 60,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Cleanup is the revoke; if the test body's revoke succeeds this
	// is a no-op (already revoked). Tolerate ErrNotFound just in case.
	t.Cleanup(func() {
		if _, err := c.AccessTokens.Revoke(ctx, created.ID); err != nil && !errors.Is(err, gql.ErrNotFound) {
			t.Logf("cleanup: failed to revoke fixture %s: %v", created.ID, err)
		}
	})

	if created.Token == "" {
		t.Errorf("Create returned empty Token; want a non-empty bearer string")
	}
	if created.ID == "" {
		t.Errorf("Create returned empty ID")
	}

	if _, err := c.AccessTokens.Revoke(ctx, created.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
}

// TestIntegration_AccessTokens_List confirms List returns without error
// and produces a slice. We don't assert exact counts because the
// sandbox state is unknown — the result may legitimately be empty
// (a brand-new account with no PATs).
func TestIntegration_AccessTokens_List(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	got, err := c.AccessTokens.List(ctx, accesstokens.ListInput{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_ = len(got)
}
