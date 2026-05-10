// Package inttest provides shared helpers for integration tests that
// exercise the SDK against a live Massdriver server.
//
// Integration tests are gated by the `integration` build tag — the
// `go test ./...` default never runs them. To run them:
//
//	export MASSDRIVER_API_KEY=mds_...
//	export MASSDRIVER_ORGANIZATION_ID=<sandbox-org>
//	go test -tags=integration ./...
//
// Tests that import this package belong in `*_integration_test.go`
// files with the same build tag — that pattern keeps them invisible
// to the unit-test pipeline and only compiled when integration is
// explicitly requested.
//
// Conventions:
//
//   - All fixtures are named with the prefix returned by [FixturePrefix]
//     so orphans from a crashed test run can be swept later.
//   - Tests use [t.Cleanup] for inline teardown — fixtures are removed
//     even when assertions fail.
//   - Tests should assume nothing about the sandbox's existing state,
//     and should leave it in the same state on success.
package inttest

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
)

// FixturePrefix is the name prefix every integration-test fixture
// carries. Sweep orphans by listing-and-deleting anything matching
// `inttest-*` in the configured organization.
const FixturePrefix = "inttest-"

// Client returns a SDK client built from the standard env vars
// (MASSDRIVER_API_KEY, MASSDRIVER_ORGANIZATION_ID, etc.). If
// credentials are missing, the test fails — running with
// `-tags=integration` is an explicit opt-in, and there is no
// meaningful integration test we can run without a live API.
//
// The build tag (`//go:build integration`) is the gate: a normal
// `go test ./...` does not compile these files, so contributors
// without sandbox credentials are unaffected.
//
// Call from each integration test:
//
//	func TestIntegration_Projects_CRUD(t *testing.T) {
//	    c := inttest.Client(t)
//	    // ...
//	}
func Client(t *testing.T) *massdriver.Client {
	t.Helper()
	if os.Getenv("MASSDRIVER_API_KEY") == "" || os.Getenv("MASSDRIVER_ORGANIZATION_ID") == "" {
		t.Fatal("integration tests require MASSDRIVER_API_KEY and MASSDRIVER_ORGANIZATION_ID")
	}
	c, err := massdriver.NewClient()
	if err != nil {
		t.Fatalf("inttest.Client: %v", err)
	}
	return c
}

// FixtureName returns a unique, grep-able name for a test fixture.
// The format is `inttest-<kind>-<random>`. Use the same `kind` for
// every fixture of the same conceptual type within a test (e.g.
// "project") so the leftover-cleanup heuristic can group them.
//
// Example:
//
//	id := inttest.FixtureName(t, "project")
//	// → "inttest-project-2f9c1b3a"
func FixtureName(t *testing.T, kind string) string {
	t.Helper()
	return FixturePrefix + sanitizeKind(kind) + "-" + randomSuffix()
}

// IsFixture reports whether name was produced by [FixtureName] —
// i.e., it carries the [FixturePrefix]. Useful in cleanup sweeps:
//
//	for _, p := range projects {
//	    if inttest.IsFixture(p.Name) {
//	        _, _ = c.Projects.Delete(ctx, p.ID)
//	    }
//	}
func IsFixture(name string) bool {
	return strings.HasPrefix(name, FixturePrefix)
}

// sanitizeKind strips characters that aren't legal in Massdriver
// identifiers (id rules: lowercase alphanumeric and dashes).
func sanitizeKind(kind string) string {
	out := make([]byte, 0, len(kind))
	for i := 0; i < len(kind); i++ {
		c := kind[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+('a'-'A'))
		case c == '_', c == ' ':
			out = append(out, '-')
		}
	}
	return string(out)
}

// randomSuffix returns 8 hex characters of entropy. Crypto/rand so
// parallel tests can't collide.
func randomSuffix() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
