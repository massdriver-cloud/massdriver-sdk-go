package config_test

import (
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
  custom:
    organization_id: "custom-org"
    api_key: "custom-key"
    url: "https://custom.massdriver.cloud"
`
	xdgProfileYAML := `
version: 1
profiles:
  default:
    organization_id: "xdg-org"
    api_key: "xdg-key"
    url: "https://xdg.massdriver.cloud"
`

	tests := []struct {
		name            string
		env             map[string]string
		writeProfile    bool
		writeXDGProfile bool
		expectErr       string
		expectConfig    *config.Config
	}{
		{
			name: "loads full config with URL",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_API_KEY":         "key-abc",
				"MASSDRIVER_DEPLOYMENT_ID":   "deploy-123",
				"MASSDRIVER_TOKEN":           "token-abc",
				"MASSDRIVER_URL":             "https://custom.massdriver.cloud",
				"MASSDRIVER_PROFILE":         "dev",
			},
			expectConfig: &config.Config{
				OrganizationID: "org-id",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "",
				Credentials: &config.Credentials{
					Method:          config.AuthDeployment,
					ID:              "deploy-123",
					Secret:          "token-abc",
					AuthHeaderValue: "Basic ZGVwbG95LTEyMzp0b2tlbi1hYmM=",
				},
			},
		},
		{
			name: "defaults URL to standard URL",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-id",
				"MASSDRIVER_API_KEY":         "abc123",
			},
			expectConfig: &config.Config{
				OrganizationID: "org-id",
				URL:            "https://api.massdriver.cloud",
				Credentials: &config.Credentials{
					Method:          config.AuthAPIKey,
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
			expectConfig: &config.Config{
				OrganizationID: "org-id",
				URL:            "https://api.massdriver.cloud",
				Credentials: &config.Credentials{
					Method:          config.AuthAPIKey,
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
			expectConfig: &config.Config{
				OrganizationID: "org-id",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				Credentials: &config.Credentials{
					Method:          config.AuthAPIKey,
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
			expectConfig: &config.Config{
				OrganizationID: "custom-org",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				Credentials: &config.Credentials{
					Method:          config.AuthAPIKey,
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
			expectConfig: &config.Config{
				OrganizationID: "profile-org",
				URL:            "https://api.massdriver.cloud",
				Profile:        "default",
				Credentials: &config.Credentials{
					Method:          config.AuthAPIKey,
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
			expectConfig: &config.Config{
				OrganizationID: "env-org",
				URL:            "https://custom.massdriver.cloud",
				Profile:        "custom",
				Credentials: &config.Credentials{
					Method:          config.AuthAPIKey,
					ID:              "env-org",
					Secret:          "env-key",
					AuthHeaderValue: "Basic ZW52LW9yZzplbnYta2V5",
				},
			},
		},
		{
			name: "loads config from XDG_CONFIG_HOME if present",
			env: map[string]string{
				"HOME": "/nonexistent",
			},
			writeXDGProfile: true,
			expectConfig: &config.Config{
				OrganizationID: "xdg-org",
				URL:            "https://xdg.massdriver.cloud",
				Profile:        "default",
				Credentials: &config.Credentials{
					Method:          config.AuthAPIKey,
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for k, v := range test.env {
				t.Setenv(k, v)
			}
			xdgDir := t.TempDir()
			homeDir := t.TempDir()

			if test.writeXDGProfile {
				writeTempConfigFileAt(t, xdgDir, "massdriver/config.yaml", xdgProfileYAML)
				t.Setenv("XDG_CONFIG_HOME", xdgDir)
			}

			if test.writeProfile {
				writeTempConfigFileAt(t, homeDir, ".config/massdriver/config.yaml", profileYAML)
				t.Setenv("HOME", homeDir)
			}

			cfg, err := config.Get()

			if test.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				require.Equal(t, *test.expectConfig, *cfg)
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
