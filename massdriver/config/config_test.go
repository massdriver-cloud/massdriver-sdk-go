package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	profileYAML := `
version: 1
profiles:
  default:
    organization_id: "profile-org"
    api_key: "profile-key"
    templates_path: "/default/templates"
  custom:
    organization_id: "custom-org"
    api_key: "custom-key"
    url: "https://custom.massdriver.cloud"
    templates_path: "/custom/templates"
`
	xdgProfileYAML := `
version: 1
profiles:
  default:
    organization_id: "xdg-org"
    api_key: "xdg-key"
    url: "https://xdg.massdriver.cloud"
`

	noDefaultProfileYAML := `
version: 1
profiles:
  custom:
    organization_id: "custom-org"
    api_key: "custom-key"
`

	tests := []struct {
		name            string
		env             map[string]string
		writeProfile    bool
		writeXDGProfile bool
		// currentProf is prepended to the written file as current_profile.
		currentProf           string
		writeNoDefaultProfile bool
		expectErr             string
		expectConfig          config.Config
	}{
		{
			name: "API key wins over deployment env vars without explicit opt-in",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_API_KEY":         "key-abc",
				"MASSDRIVER_DEPLOYMENT_ID":   "deploy-123",
				"MASSDRIVER_TOKEN":           "token-abc",
				"MASSDRIVER_URL":             "https://custom.massdriver.cloud",
			},
			expectConfig: config.Config{
				OrganizationID: "org-id",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceEnv,
					ID:              "org-id",
					Secret:          "key-abc",
					AuthHeaderValue: "Basic b3JnLWlkOmtleS1hYmM=",
				},
			},
		},
		{
			name: "deployment env vars alone are not used without explicit opt-in",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_DEPLOYMENT_ID":   "deploy-123",
				"MASSDRIVER_TOKEN":           "token-abc",
			},
			expectErr: "deployment token authentication requires explicit opt-in",
		},
		{
			name: "defaults URL to standard URL",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_API_KEY":         "abc123",
			},
			expectConfig: config.Config{
				OrganizationID: "org-id",
				URL:            "https://api.massdriver.cloud",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceEnv,
					ID:              "org-id",
					Secret:          "abc123",
					AuthHeaderValue: "Basic b3JnLWlkOmFiYzEyMw==",
				},
			},
		},
		{
			name: "falls back to MASSDRIVER_ORG_ID if MASSDRIVER_ORGANIZATION_ID is not set",
			env: map[string]string{
				"MASSDRIVER_ORG_ID":  "org-id",
				"MASSDRIVER_API_KEY": "abc123",
			},
			expectConfig: config.Config{
				OrganizationID: "org-id",
				URL:            "https://api.massdriver.cloud",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceEnv,
					ID:              "org-id",
					Secret:          "abc123",
					AuthHeaderValue: "Basic b3JnLWlkOmFiYzEyMw==",
				},
			},
		},
		{
			name: "env vars take precedence over profile",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_API_KEY":         "key-abc",
				"MASSDRIVER_URL":             "https://custom.massdriver.cloud",
				"MASSDRIVER_PROFILE":         "custom",
			},
			writeProfile: true,
			expectConfig: config.Config{
				OrganizationID: "org-id",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				TemplatesPath:  "/custom/templates",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceEnv,
					ID:              "org-id",
					Secret:          "key-abc",
					AuthHeaderValue: "Basic b3JnLWlkOmtleS1hYmM=",
				},
			},
		},
		{
			name: "profile used if env vars missing",
			env: map[string]string{
				"MASSDRIVER_PROFILE": "custom",
			},
			writeProfile: true,
			expectConfig: config.Config{
				OrganizationID: "custom-org",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				TemplatesPath:  "/custom/templates",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceProfile,
					ID:              "custom-org",
					Secret:          "custom-key",
					AuthHeaderValue: "Basic Y3VzdG9tLW9yZzpjdXN0b20ta2V5",
				},
			},
		},
		{
			name:         "default profile used if MASSDRIVER_PROFILE not set",
			env:          map[string]string{},
			writeProfile: true,
			expectConfig: config.Config{
				OrganizationID: "profile-org",
				URL:            "https://api.massdriver.cloud",
				Profile:        "default",
				TemplatesPath:  "/default/templates",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceProfile,
					ID:              "profile-org",
					Secret:          "profile-key",
					AuthHeaderValue: "Basic cHJvZmlsZS1vcmc6cHJvZmlsZS1rZXk=",
				},
			},
		},
		{
			name: "env vars take precedence over profile for org id and api key",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "env-org",
				"MASSDRIVER_API_KEY":         "env-key",
				"MASSDRIVER_PROFILE":         "custom",
			},
			writeProfile: true,
			expectConfig: config.Config{
				OrganizationID: "env-org",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				TemplatesPath:  "/custom/templates",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceEnv,
					ID:              "env-org",
					Secret:          "env-key",
					AuthHeaderValue: "Basic ZW52LW9yZzplbnYta2V5",
				},
			},
		},
		{
			name: "templates_path env var takes precedence over profile",
			env: map[string]string{
				"MASSDRIVER_PROFILE":        "custom",
				"MASSDRIVER_TEMPLATES_PATH": "/env/templates",
			},
			writeProfile: true,
			expectConfig: config.Config{
				OrganizationID: "custom-org",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				TemplatesPath:  "/env/templates",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceProfile,
					ID:              "custom-org",
					Secret:          "custom-key",
					AuthHeaderValue: "Basic Y3VzdG9tLW9yZzpjdXN0b20ta2V5",
				},
			},
		},
		{
			name: "loads config from XDG_CONFIG_HOME if present",
			env: map[string]string{
				"HOME": "/nonexistent",
			},
			writeXDGProfile: true,
			expectConfig: config.Config{
				OrganizationID: "xdg-org",
				URL:            "https://xdg.massdriver.cloud",
				Profile:        "default",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceProfile,
					ID:              "xdg-org",
					Secret:          "xdg-key",
					AuthHeaderValue: "Basic eGRnLW9yZzp4ZGcta2V5",
				},
			},
		},
		{
			name: "errors if OrgID is a UUID",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "00000000-1111-2222-3333-444444444444",
				"MASSDRIVER_API_KEY":         "abc123",
			},
			expectErr: "organization ID is a UUID. This is deprecated and will be removed in a future release, please use the organization abbreviation instead",
		},
		{
			name: "errors if niether orgId or organizationId is set",
			env: map[string]string{
				"MASSDRIVER_DEPLOYMENT_ID": "deploy-123",
				"MASSDRIVER_TOKEN":         "token-xyz",
				"MASSDRIVER_API_KEY":       "abc123",
			},
			expectErr: "organization ID is required",
		},
		{
			name: "errors if URL doesn't include protocol",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-slug",
				"MASSDRIVER_API_KEY":         "key-abc",
				"MASSDRIVER_URL":             "custom.domain.com",
			},
			expectErr: "url must include scheme and host (e.g., https://api.massdriver.cloud)",
		},
		{
			name:      "empty config should error",
			env:       map[string]string{},
			expectErr: "no credentials found",
		},
		{
			name: "unknown MASSDRIVER_PROFILE errors instead of using env credentials",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_API_KEY":         "key-abc",
				"MASSDRIVER_PROFILE":         "dev",
			},
			writeProfile: true,
			expectErr:    `profile not found: "dev" was requested by MASSDRIVER_PROFILE, but`,
		},
		{
			name: "unknown MASSDRIVER_PROFILE errors when no config file exists",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_API_KEY":         "key-abc",
				"MASSDRIVER_PROFILE":         "dev",
			},
			expectErr: "but no config file exists at",
		},
		{
			name: "not-found error lists the available profiles",
			env: map[string]string{
				"MASSDRIVER_PROFILE": "dev",
			},
			writeProfile: true,
			expectErr:    "defines only custom, default",
		},
		{
			name:         "current_profile selects the profile when MASSDRIVER_PROFILE is unset",
			env:          map[string]string{},
			writeProfile: true,
			currentProf:  "custom",
			expectConfig: config.Config{
				OrganizationID: "custom-org",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				TemplatesPath:  "/custom/templates",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceProfile,
					ID:              "custom-org",
					Secret:          "custom-key",
					AuthHeaderValue: "Basic Y3VzdG9tLW9yZzpjdXN0b20ta2V5",
				},
			},
		},
		{
			name: "MASSDRIVER_PROFILE outranks current_profile",
			env: map[string]string{
				"MASSDRIVER_PROFILE": "default",
			},
			writeProfile: true,
			currentProf:  "custom",
			expectConfig: config.Config{
				OrganizationID: "profile-org",
				URL:            "https://api.massdriver.cloud",
				Profile:        "default",
				TemplatesPath:  "/default/templates",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceProfile,
					ID:              "profile-org",
					Secret:          "profile-key",
					AuthHeaderValue: "Basic cHJvZmlsZS1vcmc6cHJvZmlsZS1rZXk=",
				},
			},
		},
		{
			name:         "unknown current_profile errors",
			env:          map[string]string{},
			writeProfile: true,
			currentProf:  "deleted",
			expectErr:    `profile not found: "deleted" was requested by current_profile in the config file`,
		},
		{
			name: "missing default profile is not an error when nothing requested it",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "env-org",
				"MASSDRIVER_API_KEY":         "env-key",
			},
			writeNoDefaultProfile: true,
			expectConfig: config.Config{
				OrganizationID: "env-org",
				URL:            "https://api.massdriver.cloud",
				Profile:        "",
				Credentials: config.Credentials{
					Method:          config.AuthAPIKey,
					Source:          config.SourceEnv,
					ID:              "env-org",
					Secret:          "env-key",
					AuthHeaderValue: "Basic ZW52LW9yZzplbnYta2V5",
				},
			},
		},
	}

	// Every MASSDRIVER_* env var the config package reads. We clear all
	// of them at the start of each subtest so values from the developer's
	// shell (e.g. integration-test credentials) don't leak in and shadow
	// the per-case fixtures.
	massdriverEnv := []string{
		"MASSDRIVER_ORGANIZATION_ID",
		"MASSDRIVER_ORG_ID",
		"MASSDRIVER_API_KEY",
		"MASSDRIVER_DEPLOYMENT_ID",
		"MASSDRIVER_TOKEN",
		"MASSDRIVER_PROFILE",
		"MASSDRIVER_URL",
		"MASSDRIVER_TEMPLATES_PATH",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, k := range massdriverEnv {
				t.Setenv(k, "")
			}
			for k, v := range test.env {
				t.Setenv(k, v)
			}

			xdgDir := t.TempDir()
			homeDir := t.TempDir()
			t.Setenv("HOME", homeDir)

			if test.writeXDGProfile {
				writeTempConfigFileAt(t, xdgDir, "massdriver/config.yaml", xdgProfileYAML)
				t.Setenv("XDG_CONFIG_HOME", xdgDir)
			} else {
				t.Setenv("XDG_CONFIG_HOME", "")
			}

			if test.writeProfile {
				contents := profileYAML
				if test.currentProf != "" {
					contents = fmt.Sprintf("current_profile: %q\n%s", test.currentProf, contents)
				}
				writeTempConfigFileAt(t, homeDir, ".config/massdriver/config.yaml", contents)
			}

			if test.writeNoDefaultProfile {
				writeTempConfigFileAt(t, homeDir, ".config/massdriver/config.yaml", noDefaultProfileYAML)
			}

			cfg, err := config.Get()

			if test.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				require.Equal(t, test.expectConfig, cfg)
			}
		})
	}
}

func writeTempConfigFileAt(t *testing.T, dir, relPath, content string) string {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
	return fullPath
}

// TestLoad_OverrideSetsSourceOption confirms that an explicit override
// (the path the top-level [massdriver.WithAPIKey] takes) tags the
// resolved Credentials with [config.SourceOption], not the env or
// profile source the override would otherwise mask.
func TestLoad_OverrideSetsSourceOption(t *testing.T) {
	// Env vars supply a fallback the override should preempt.
	t.Setenv("MASSDRIVER_ORGANIZATION_ID", "env-org")
	t.Setenv("MASSDRIVER_API_KEY", "env-key")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	cfg, err := config.Load(config.Overrides{
		APIKey:         "explicit-key",
		OrganizationID: "explicit-org",
	})
	require.NoError(t, err)

	require.Equal(t, config.SourceOption, cfg.Credentials.Source,
		"override path must mark the credential as Source=option")
	require.Equal(t, "explicit-key", cfg.Credentials.Secret)
	require.Equal(t, "explicit-org", cfg.OrganizationID)
}

// TestLoad_PATSourceTracking confirms a PAT-prefixed key resolves to
// AuthPAT regardless of which layer supplied it, and the Source
// reflects that layer.
func TestLoad_PATSourceTracking(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	t.Run("PAT from option", func(t *testing.T) {
		cfg, err := config.Load(config.Overrides{
			APIKey:         "mds_abc123",
			OrganizationID: "ecomm",
		})
		require.NoError(t, err)
		require.Equal(t, config.AuthPAT, cfg.Credentials.Method)
		require.Equal(t, config.SourceOption, cfg.Credentials.Source)
		require.Equal(t, "Bearer mds_abc123", cfg.Credentials.AuthHeaderValue)
	})

	t.Run("PAT from env", func(t *testing.T) {
		t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")
		t.Setenv("MASSDRIVER_API_KEY", "md_xyz789")
		cfg, err := config.Load(config.Overrides{})
		require.NoError(t, err)
		require.Equal(t, config.AuthPAT, cfg.Credentials.Method)
		require.Equal(t, config.SourceEnv, cfg.Credentials.Source)
	})
}

// TestLoad_AuthMethod covers the explicit auth-method request:
// deployment tokens resolve only when asked for, and asking for them
// makes the deployment env vars mandatory.
func TestLoad_AuthMethod(t *testing.T) {
	for _, k := range []string{
		"MASSDRIVER_ORGANIZATION_ID",
		"MASSDRIVER_ORG_ID",
		"MASSDRIVER_API_KEY",
		"MASSDRIVER_DEPLOYMENT_ID",
		"MASSDRIVER_TOKEN",
		"MASSDRIVER_PROFILE",
		"MASSDRIVER_URL",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	t.Run("deployment opt-in resolves deployment envs", func(t *testing.T) {
		t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")
		t.Setenv("MASSDRIVER_DEPLOYMENT_ID", "deploy-123")
		t.Setenv("MASSDRIVER_TOKEN", "token-abc")

		cfg, err := config.Load(config.Overrides{AuthMethod: config.AuthDeployment})
		require.NoError(t, err)
		require.Equal(t, config.Credentials{
			Method:          config.AuthDeployment,
			Source:          config.SourceEnv,
			ID:              "deploy-123",
			Secret:          "token-abc",
			AuthHeaderValue: "Basic ZGVwbG95LTEyMzp0b2tlbi1hYmM=",
		}, cfg.Credentials)
	})

	t.Run("deployment opt-in beats an ambient API key", func(t *testing.T) {
		t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")
		t.Setenv("MASSDRIVER_API_KEY", "key-abc")
		t.Setenv("MASSDRIVER_DEPLOYMENT_ID", "deploy-123")
		t.Setenv("MASSDRIVER_TOKEN", "token-abc")

		cfg, err := config.Load(config.Overrides{AuthMethod: config.AuthDeployment})
		require.NoError(t, err)
		require.Equal(t, config.AuthDeployment, cfg.Credentials.Method)
	})

	t.Run("deployment opt-in errors when envs are missing", func(t *testing.T) {
		t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")
		t.Setenv("MASSDRIVER_API_KEY", "key-abc")
		t.Setenv("MASSDRIVER_DEPLOYMENT_ID", "")
		t.Setenv("MASSDRIVER_TOKEN", "")

		_, err := config.Load(config.Overrides{AuthMethod: config.AuthDeployment})
		require.Error(t, err)
		require.Contains(t, err.Error(), "MASSDRIVER_DEPLOYMENT_ID and MASSDRIVER_TOKEN")
	})

	t.Run("unsupported method errors", func(t *testing.T) {
		t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")
		t.Setenv("MASSDRIVER_API_KEY", "key-abc")

		_, err := config.Load(config.Overrides{AuthMethod: "bogus"})
		require.Error(t, err)
		require.Contains(t, err.Error(), `unsupported auth method: "bogus"`)
	})
}

// TestLoad_SentinelErrors confirms credential failures classify with
// errors.Is through Load's wrapping, so callers (e.g. the Terraform
// provider treating an API key as optional) don't have to match
// message text.
func TestLoad_SentinelErrors(t *testing.T) {
	for _, k := range []string{
		"MASSDRIVER_ORGANIZATION_ID",
		"MASSDRIVER_ORG_ID",
		"MASSDRIVER_API_KEY",
		"MASSDRIVER_DEPLOYMENT_ID",
		"MASSDRIVER_TOKEN",
		"MASSDRIVER_PROFILE",
		"MASSDRIVER_URL",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	t.Run("empty config is ErrNoCredentials", func(t *testing.T) {
		_, err := config.Load(config.Overrides{})
		require.ErrorIs(t, err, config.ErrNoCredentials)
	})

	t.Run("deployment envs without opt-in is still ErrNoCredentials", func(t *testing.T) {
		t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")
		t.Setenv("MASSDRIVER_DEPLOYMENT_ID", "deploy-123")
		t.Setenv("MASSDRIVER_TOKEN", "token-abc")

		_, err := config.Load(config.Overrides{})
		require.ErrorIs(t, err, config.ErrNoCredentials)
	})

	t.Run("deployment opt-in without envs is ErrDeploymentCredentialsMissing", func(t *testing.T) {
		t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")

		_, err := config.Load(config.Overrides{AuthMethod: config.AuthDeployment})
		require.ErrorIs(t, err, config.ErrDeploymentCredentialsMissing)
	})

	t.Run("API key without org is ErrOrganizationIDRequired", func(t *testing.T) {
		t.Setenv("MASSDRIVER_API_KEY", "key-abc")

		_, err := config.Load(config.Overrides{})
		require.ErrorIs(t, err, config.ErrOrganizationIDRequired)
	})

	t.Run("deployment auth without org is ErrOrganizationIDRequired", func(t *testing.T) {
		t.Setenv("MASSDRIVER_DEPLOYMENT_ID", "deploy-123")
		t.Setenv("MASSDRIVER_TOKEN", "token-abc")

		_, err := config.Load(config.Overrides{AuthMethod: config.AuthDeployment})
		require.ErrorIs(t, err, config.ErrOrganizationIDRequired)
	})
}

// TestLoad_IgnoresBareEnvVars guards against envconfig's bare-tag
// fallback: unprefixed TOKEN, URL, API_KEY, etc. must never be honored.
func TestLoad_IgnoresBareEnvVars(t *testing.T) {
	for _, k := range []string{
		"MASSDRIVER_ORGANIZATION_ID",
		"MASSDRIVER_ORG_ID",
		"MASSDRIVER_API_KEY",
		"MASSDRIVER_DEPLOYMENT_ID",
		"MASSDRIVER_TOKEN",
		"MASSDRIVER_PROFILE",
		"MASSDRIVER_URL",
		"MASSDRIVER_TEMPLATES_PATH",
	} {
		t.Setenv(k, "") // register restore-on-cleanup
		os.Unsetenv(k)  // the fallback only triggers when truly unset
	}
	for k, v := range map[string]string{
		"ORGANIZATION_ID": "bare-org",
		"ORG_ID":          "bare-org",
		"API_KEY":         "bare-key",
		"DEPLOYMENT_ID":   "bare-deploy",
		"TOKEN":           "bare-token",
		"PROFILE":         "bare-profile",
		"URL":             "https://bare.example.com",
		"TEMPLATES_PATH":  "/bare/templates",
	} {
		t.Setenv(k, v)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	_, err := config.Load(config.Overrides{})
	require.ErrorIs(t, err, config.ErrNoCredentials, "bare API_KEY/ORGANIZATION_ID must not resolve credentials")

	_, err = config.Load(config.Overrides{AuthMethod: config.AuthDeployment})
	require.ErrorIs(t, err, config.ErrDeploymentCredentialsMissing, "bare DEPLOYMENT_ID/TOKEN must not resolve deployment credentials")

	// With real prefixed credentials, a stray bare URL must not
	// redirect the client.
	t.Setenv("MASSDRIVER_ORGANIZATION_ID", "org")
	t.Setenv("MASSDRIVER_API_KEY", "key")
	cfg, err := config.Load(config.Overrides{})
	require.NoError(t, err)
	require.Equal(t, "https://api.massdriver.cloud", cfg.URL)
	require.Empty(t, cfg.TemplatesPath)
	require.Empty(t, cfg.Profile)
}

// TestLoad_ProfileNotFoundSentinel confirms a missing explicitly-named
// profile classifies with errors.Is through Load's wrapping.
func TestLoad_ProfileNotFoundSentinel(t *testing.T) {
	for _, k := range []string{
		"MASSDRIVER_ORGANIZATION_ID",
		"MASSDRIVER_ORG_ID",
		"MASSDRIVER_API_KEY",
		"MASSDRIVER_DEPLOYMENT_ID",
		"MASSDRIVER_TOKEN",
		"MASSDRIVER_PROFILE",
		"MASSDRIVER_URL",
	} {
		t.Setenv(k, "")
	}
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", "")
	writeTempConfigFileAt(t, homeDir, ".config/massdriver/config.yaml", `
version: 1
profiles:
  default:
    organization_id: "profile-org"
    api_key: "profile-key"
`)

	t.Run("Profile override that misses is ErrProfileNotFound", func(t *testing.T) {
		_, err := config.Load(config.Overrides{Profile: "nope"})
		require.ErrorIs(t, err, config.ErrProfileNotFound)
		require.Contains(t, err.Error(), "the Profile option")
	})

	t.Run("override outranks env for the requested name", func(t *testing.T) {
		t.Setenv("MASSDRIVER_PROFILE", "default")
		_, err := config.Load(config.Overrides{Profile: "nope"})
		require.ErrorIs(t, err, config.ErrProfileNotFound)
		require.Contains(t, err.Error(), `"nope"`)
	})

	t.Run("implicit default that hits is not an error", func(t *testing.T) {
		cfg, err := config.Load(config.Overrides{})
		require.NoError(t, err)
		require.Equal(t, "default", cfg.Profile)
	})
}

// TestLoad_ProfileNotFoundMatrix is the canonical statement of when a
// missing profile is an error: only when a layer named it explicitly,
// and never for deployment-token auth, which cannot use a profile.
func TestLoad_ProfileNotFoundMatrix(t *testing.T) {
	const fileWithDefault = `
version: 1
profiles:
  default:
    organization_id: "profile-org"
    api_key: "profile-key"
`
	const fileDangling = `
version: 1
current_profile: "deleted"
profiles:
  default:
    organization_id: "profile-org"
    api_key: "profile-key"
`
	const fileNoDefault = `
version: 1
profiles:
  custom:
    organization_id: "custom-org"
    api_key: "custom-key"
`

	cases := []struct {
		name      string
		file      string
		env       map[string]string
		overrides config.Overrides
		wantErr   bool
	}{
		// --- explicitly named, missing -> MUST error ---
		{"WithProfile missing, file present", fileWithDefault, nil,
			config.Overrides{Profile: "nope"}, true},
		{"WithProfile missing, no file", "", nil,
			config.Overrides{Profile: "nope"}, true},
		{"WithProfile missing, valid option creds", fileWithDefault, nil,
			config.Overrides{Profile: "nope", APIKey: "mds_a", OrganizationID: "ecomm"}, true},
		{"MASSDRIVER_PROFILE missing", fileWithDefault,
			map[string]string{"MASSDRIVER_PROFILE": "nope"}, config.Overrides{}, true},
		{"MASSDRIVER_PROFILE missing, valid env creds", fileWithDefault,
			map[string]string{"MASSDRIVER_PROFILE": "nope", "MASSDRIVER_ORGANIZATION_ID": "e", "MASSDRIVER_API_KEY": "k"},
			config.Overrides{}, true},
		{"current_profile missing", fileDangling, nil, config.Overrides{}, true},
		{"current_profile missing, valid env creds", fileDangling,
			map[string]string{"MASSDRIVER_ORGANIZATION_ID": "e", "MASSDRIVER_API_KEY": "k"},
			config.Overrides{}, true},
		{"current_profile missing, valid option creds", fileDangling, nil,
			config.Overrides{APIKey: "mds_a", OrganizationID: "ecomm"}, true},

		// --- deployment auth: profiles are irrelevant -> MUST NOT error ---
		{"deployment + WithProfile missing", fileWithDefault,
			map[string]string{"MASSDRIVER_ORGANIZATION_ID": "e", "MASSDRIVER_DEPLOYMENT_ID": "d", "MASSDRIVER_TOKEN": "t"},
			config.Overrides{Profile: "nope", AuthMethod: config.AuthDeployment}, false},
		{"deployment + MASSDRIVER_PROFILE missing", fileWithDefault,
			map[string]string{"MASSDRIVER_PROFILE": "nope", "MASSDRIVER_ORGANIZATION_ID": "e", "MASSDRIVER_DEPLOYMENT_ID": "d", "MASSDRIVER_TOKEN": "t"},
			config.Overrides{AuthMethod: config.AuthDeployment}, false},
		{"deployment + current_profile missing", fileDangling,
			map[string]string{"MASSDRIVER_ORGANIZATION_ID": "e", "MASSDRIVER_DEPLOYMENT_ID": "d", "MASSDRIVER_TOKEN": "t"},
			config.Overrides{AuthMethod: config.AuthDeployment}, false},

		// --- nothing named a profile -> MUST NOT error ---
		{"implicit default, no file, env creds", "",
			map[string]string{"MASSDRIVER_ORGANIZATION_ID": "e", "MASSDRIVER_API_KEY": "k"},
			config.Overrides{}, false},
		{"implicit default absent from file, env creds", fileNoDefault,
			map[string]string{"MASSDRIVER_ORGANIZATION_ID": "e", "MASSDRIVER_API_KEY": "k"},
			config.Overrides{}, false},
		{"implicit default present", fileWithDefault, nil, config.Overrides{}, false},

		// --- named profile exists -> MUST NOT error ---
		{"WithProfile hits", fileWithDefault, nil, config.Overrides{Profile: "default"}, false},
		{"current_profile hits", "\nversion: 1\ncurrent_profile: \"custom\"\nprofiles:\n  custom:\n    organization_id: \"c\"\n    api_key: \"k\"\n",
			nil, config.Overrides{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{
				"MASSDRIVER_ORGANIZATION_ID", "MASSDRIVER_ORG_ID", "MASSDRIVER_API_KEY",
				"MASSDRIVER_DEPLOYMENT_ID", "MASSDRIVER_TOKEN", "MASSDRIVER_PROFILE", "MASSDRIVER_URL",
			} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			xdg := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdg)
			t.Setenv("HOME", t.TempDir())
			if tc.file != "" {
				writeTempConfigFileAt(t, xdg, "massdriver/config.yaml", tc.file)
			}

			_, err := config.Load(tc.overrides)
			isProfileErr := errors.Is(err, config.ErrProfileNotFound)
			if tc.wantErr {
				require.True(t, isProfileErr, "expected ErrProfileNotFound, got: %v", err)
			} else {
				require.False(t, isProfileErr, "expected no profile error, got: %v", err)
			}
		})
	}
}

// TestFileAPI covers the surface the Massdriver CLI writes against, so
// the two can't drift on schema or file location.
func TestFileAPI(t *testing.T) {
	t.Run("FilePath prefers XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/xdg")
		path, err := config.FilePath()
		require.NoError(t, err)
		require.Equal(t, "/xdg/massdriver/config.yaml", path)
	})

	t.Run("FilePath falls back to home", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/home/someone")
		path, err := config.FilePath()
		require.NoError(t, err)
		require.Equal(t, "/home/someone/.config/massdriver/config.yaml", path)
	})

	t.Run("ReadFile returns nil for an absent file", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		file, err := config.ReadFile()
		require.NoError(t, err)
		require.Nil(t, file)
	})

	t.Run("ReadFile parses current_profile and profiles", func(t *testing.T) {
		xdgDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdgDir)
		writeTempConfigFileAt(t, xdgDir, "massdriver/config.yaml", `
version: 1
current_profile: "custom"
profiles:
  default:
    organization_id: "profile-org"
  custom:
    organization_id: "custom-org"
    templates_path: "/custom/templates"
`)
		file, err := config.ReadFile()
		require.NoError(t, err)
		require.Equal(t, config.Version, file.Version)
		require.Equal(t, "custom", file.CurrentProfile)
		require.Equal(t, []string{"custom", "default"}, file.ProfileNames())
		require.Equal(t, "/custom/templates", file.Profiles["custom"].TemplatesPath)
	})

	t.Run("ReadFile rejects an unsupported version", func(t *testing.T) {
		xdgDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdgDir)
		writeTempConfigFileAt(t, xdgDir, "massdriver/config.yaml", "version: 99\n")
		_, err := config.ReadFile()
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported config file version: 99")
	})

	t.Run("ProfileNames on a nil file is empty", func(t *testing.T) {
		var file *config.File
		require.Empty(t, file.ProfileNames())
	})
}

// TestCredentials_Redaction confirms Secret and AuthHeaderValue never
// appear when a Credentials — bare or nested inside a Config — is
// printed with any of fmt's default verbs.
func TestCredentials_Redaction(t *testing.T) {
	creds := config.Credentials{
		Method:          config.AuthAPIKey,
		Source:          config.SourceEnv,
		ID:              "ecomm",
		Secret:          "super-secret-key",
		AuthHeaderValue: "Basic c3VwZXItc2VjcmV0",
	}

	for _, verb := range []string{"%v", "%+v", "%s", "%#v"} {
		out := fmt.Sprintf(verb, creds)
		require.NotContains(t, out, "super-secret-key", "verb %s leaked Secret", verb)
		require.NotContains(t, out, "c3VwZXItc2VjcmV0", "verb %s leaked AuthHeaderValue", verb)
		require.Contains(t, out, "REDACTED", "verb %s should mark set fields as REDACTED", verb)
		require.Contains(t, out, "ecomm", "verb %s should keep non-secret fields visible", verb)
	}

	cfg := config.Config{OrganizationID: "ecomm", URL: "https://api.massdriver.cloud", Credentials: creds}
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		out := fmt.Sprintf(verb, cfg)
		require.NotContains(t, out, "super-secret-key", "verb %s leaked Secret via Config", verb)
		require.NotContains(t, out, "c3VwZXItc2VjcmV0", "verb %s leaked AuthHeaderValue via Config", verb)
	}

	// Unset fields print empty, not REDACTED, so "no credential
	// resolved" stays distinguishable in debug output.
	require.NotContains(t, fmt.Sprintf("%v", config.Credentials{}), "REDACTED")
}
