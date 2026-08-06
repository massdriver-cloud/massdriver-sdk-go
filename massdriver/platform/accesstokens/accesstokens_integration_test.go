//go:build integration

package accesstokens_test

import (
	"context"
	"errors"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/inttest"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/accesstokens"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/viewer"
)

// TestIntegration_AccessTokens_CreateAndRevoke creates a fresh token and
// revokes it. The create mutation is identity-gated, so the test asks the
// viewer which identity the sandbox credential is and picks the matching
// create method. Access tokens are an auth-sensitive surface: revoking the
// wrong token could lock the test runner out, so this test only revokes
// the token it just created — never anything from List.
func TestIntegration_AccessTokens_CreateAndRevoke(t *testing.T) {
	c := inttest.Client(t)
	ctx := context.Background()

	name := inttest.FixtureName(t, "pat")

	v, err := c.Viewer.Get(ctx)
	if err != nil {
		t.Fatalf("Viewer.Get: %v", err)
	}
	create := c.AccessTokens.CreatePersonal
	if v.Kind == viewer.KindServiceAccount {
		create = c.AccessTokens.CreateServiceAccountToken
	}

	created, err := create(ctx, accesstokens.CreateInput{
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

	got, err := types.Collect(c.AccessTokens.Iter(ctx, accesstokens.ListInput{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_ = len(got)
}
