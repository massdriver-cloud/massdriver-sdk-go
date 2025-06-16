package config_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name         string
		env          map[string]string
		expectErr    string
		expectConfig *config.Config
	}{
		{
			name: "loads full config with URL",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-slug",
				"MASSDRIVER_API_KEY":         "key-abc",
				"MASSDRIVER_DEPLOYMENT_ID":   "deploy-123",
				"MASSDRIVER_TOKEN":           "token-xyz",
				"MASSDRIVER_URL":             "https://custom.massdriver.cloud",
				"MASSDRIVER_PROFILE":         "dev",
			},
			expectConfig: &config.Config{
				OrganizationID:  "org-slug",
				APIKey:          "key-abc",
				DeploymentID:    "deploy-123",
				DeploymentToken: "token-xyz",
				URL:             "https://custom.massdriver.cloud",
				Profile:         "dev",
			},
		},
		{
			name: "defaults URL to standard URL",
			env: map[string]string{
				"MASSDRIVER_ORGANIZATION_ID": "org-slug",
				"MASSDRIVER_API_KEY":         "abc123",
			},
			expectConfig: &config.Config{
				OrganizationID: "org-slug",
				APIKey:         "abc123",
				URL:            "https://api.massdriver.cloud",
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
				APIKey:         "abc123",
				URL:            "https://api.massdriver.cloud",
			},
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
			expectErr: "no valid credentials found in environment",
		},
	}

	for _, test := range tests {
		for k, v := range test.env {
			t.Setenv(k, v)
		}
		t.Run(test.name, func(t *testing.T) {
			cfg, err := config.Get()

			if test.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				require.Equal(t, test.expectConfig.OrganizationID, cfg.OrganizationID)
				require.Equal(t, test.expectConfig.APIKey, cfg.APIKey)
				require.Equal(t, test.expectConfig.DeploymentID, cfg.DeploymentID)
				require.Equal(t, test.expectConfig.DeploymentToken, cfg.DeploymentToken)
				require.Equal(t, test.expectConfig.Profile, cfg.Profile)
				require.Equal(t, test.expectConfig.URL, cfg.URL)
			}
		})
		for k, _ := range test.env {
			t.Setenv(k, "")
		}
	}
}
