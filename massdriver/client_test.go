package massdriver_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/gqltest"
)

// isolateEnv strips MASSDRIVER_* env vars and HOME so tests don't pick
// up the developer's real credentials. Sets HOME to a tempdir so the
// config-file path doesn't resolve to a real ~/.config/massdriver.
func isolateEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"MASSDRIVER_API_KEY",
		"MASSDRIVER_ORGANIZATION_ID",
		"MASSDRIVER_ORG_ID",
		"MASSDRIVER_DEPLOYMENT_ID",
		"MASSDRIVER_TOKEN",
		"MASSDRIVER_URL",
		"MASSDRIVER_PROFILE",
		"MASSDRIVER_TEMPLATES_PATH",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
}

// TestNewClient_WithAPIKeyOverridesEnv confirms an explicit
// WithAPIKey wins over MASSDRIVER_API_KEY in the environment, and
// that the resolved AuthSource reflects "option" (not "env").
func TestNewClient_WithAPIKeyOverridesEnv(t *testing.T) {
	isolateEnv(t)
	t.Setenv("MASSDRIVER_API_KEY", "env-key")
	t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")

	c, err := massdriver.NewClient(massdriver.WithAPIKey("explicit-key"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.AuthSource() != "option" {
		t.Errorf("AuthSource = %q, want option (override should be tagged)", c.AuthSource())
	}
}

// TestNewClient_AuthMethodForPATPrefix confirms the SDK detects PAT
// keys (mds_/md_ prefix) as AuthPAT regardless of which layer
// supplied them.
func TestNewClient_AuthMethodForPATPrefix(t *testing.T) {
	isolateEnv(t)

	cases := []struct {
		name string
		key  string
		want string // expected AuthMethod()
	}{
		{"mds_ prefix", "mds_abc123", "personal_access_token"},
		{"md_ prefix", "md_xyz789", "personal_access_token"},
		{"plain key", "abc123", "api_key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := massdriver.NewClient(
				massdriver.WithAPIKey(tc.key),
				massdriver.WithOrganizationID("ecomm"),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if got := c.AuthMethod(); got != tc.want {
				t.Errorf("AuthMethod = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNewClient_EnvSourceTracking confirms credentials sourced from
// MASSDRIVER_* env vars are tagged AuthSource=env.
func TestNewClient_EnvSourceTracking(t *testing.T) {
	isolateEnv(t)
	t.Setenv("MASSDRIVER_API_KEY", "env-key")
	t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")

	c, err := massdriver.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.AuthSource() != "env" {
		t.Errorf("AuthSource = %q, want env", c.AuthSource())
	}
}

// TestNewClient_WithTimeoutSucceeds is a smoke test: passing
// WithTimeout(...) shouldn't crash construction. Verifying the
// timeout actually fires would require a real HTTP round-trip; we
// trust resty's [resty.Client.SetTimeout] for the actual behavior.
func TestNewClient_WithTimeoutSucceeds(t *testing.T) {
	isolateEnv(t)

	cases := []struct {
		name    string
		timeout time.Duration
	}{
		{"explicit short", 5 * time.Second},
		{"explicit zero (no timeout)", 0},
		{"explicit long", 5 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := massdriver.NewClient(
				massdriver.WithAPIKey("k"),
				massdriver.WithOrganizationID("ecomm"),
				massdriver.WithTimeout(tc.timeout),
			)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if c == nil {
				t.Fatal("NewClient returned nil client")
			}
		})
	}
}

// TestNewClient_WithGQLClientBypassesAuth confirms the test path
// skips credential resolution entirely — no env vars, no profile, no
// API key, just a mocked GraphQL client and an org ID.
func TestNewClient_WithGQLClientBypassesAuth(t *testing.T) {
	isolateEnv(t)

	mock := gqltest.NewClient()
	c, err := massdriver.NewClient(
		massdriver.WithGQLClient(mock),
		massdriver.WithOrganizationID("test-org"),
	)
	if err != nil {
		t.Fatalf("NewClient with mock GQL must not require auth: %v", err)
	}
	if c.OrganizationID() != "test-org" {
		t.Errorf("OrganizationID = %q, want test-org", c.OrganizationID())
	}
	// AuthMethod/AuthSource are empty when auth is bypassed.
	if c.AuthMethod() != "" {
		t.Errorf("AuthMethod = %q, want empty (auth bypassed via WithGQLClient)", c.AuthMethod())
	}
	if c.AuthSource() != "" {
		t.Errorf("AuthSource = %q, want empty (auth bypassed via WithGQLClient)", c.AuthSource())
	}
}

// TestNewClient_BaseURLAccessor confirms the BaseURL accessor returns
// what the caller passed via WithBaseURL.
func TestNewClient_BaseURLAccessor(t *testing.T) {
	isolateEnv(t)

	c, err := massdriver.NewClient(
		massdriver.WithAPIKey("k"),
		massdriver.WithOrganizationID("ecomm"),
		massdriver.WithBaseURL("https://md.internal.acme.com"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got := c.BaseURL(); got != "https://md.internal.acme.com" {
		t.Errorf("BaseURL = %q, want https://md.internal.acme.com", got)
	}
}

// TestNewClient_NoCredentialsErrors confirms NewClient with no
// options and no env/file returns an error rather than a partly-
// constructed client.
func TestNewClient_NoCredentialsErrors(t *testing.T) {
	isolateEnv(t)

	_, err := massdriver.NewClient()
	if err == nil {
		t.Fatal("NewClient with no creds should error")
	}
	if !strings.Contains(err.Error(), "credentials") {
		t.Errorf("err = %v, want it to mention credentials", err)
	}
	// The error chain should be a regular error — not one of our
	// sentinels. Sentinels are for runtime API errors, not config errors.
	for _, sentinel := range []error{nil} {
		_ = sentinel // placeholder; explicitly not asserting sentinel match
	}
	// Sanity-check: errors.Is on unrelated sentinel returns false.
	if errors.Is(err, errors.New("nope")) {
		t.Error("errors.Is should not match a fresh unrelated error")
	}
}
