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
		expectErr    bool
		expectConfig *config.Config
	}{
		{
			name: "loads full config with URL",
			env: map[string]string{
				"MASSDRIVER_ORG_ID":        "org-slug",
				"MASSDRIVER_API_KEY":       "key-abc",
				"MASSDRIVER_DEPLOYMENT_ID": "deploy-123",
				"MASSDRIVER_TOKEN":         "token-xyz",
				"MASSDRIVER_API_URL":       "https://custom.massdriver.cloud",
				"MASSDRIVER_PROFILE":       "dev",
			},
			expectErr: false,
			expectConfig: &config.Config{
				OrgID:           "org-slug",
				APIKey:          "key-abc",
				DeploymentID:    "deploy-123",
				DeploymentToken: "token-xyz",
				URL:             "https://custom.massdriver.cloud",
				Profile:         "dev",
			},
		},
		{
			name: "defaults API_URL to standard URL",
			env: map[string]string{
				"MASSDRIVER_ORG_ID":  "org-slug",
				"MASSDRIVER_API_KEY": "abc123",
			},
			expectErr: false,
			expectConfig: &config.Config{
				OrgID:  "org-slug",
				APIKey: "abc123",
				URL:    "https://api.massdriver.cloud",
			},
		},
		{
			name: "prints UUID warning if OrgID is a UUID",
			env: map[string]string{
				"MASSDRIVER_ORG_ID": "00000000-1111-2222-3333-444444444444",
			},
			expectErr: false,
			expectConfig: &config.Config{
				OrgID: "00000000-1111-2222-3333-444444444444",
				URL:   "https://api.massdriver.cloud",
			},
		},
		{
			name:      "empty config should not error",
			env:       map[string]string{},
			expectErr: false,
			expectConfig: &config.Config{
				URL: "https://api.massdriver.cloud",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all possible env vars to avoid bleed
			clearEnv := []string{
				"MASSDRIVER_ORG_ID", "MASSDRIVER_API_KEY",
				"MASSDRIVER_DEPLOYMENT_ID", "MASSDRIVER_TOKEN",
				"MASSDRIVER_API_URL", "MASSDRIVER_PROFILE",
			}
			for _, key := range clearEnv {
				t.Setenv(key, "")
			}

			// Set test-specific env
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := config.Get()

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				require.Equal(t, tt.expectConfig.OrgID, cfg.OrgID)
				require.Equal(t, tt.expectConfig.APIKey, cfg.APIKey)
				require.Equal(t, tt.expectConfig.DeploymentID, cfg.DeploymentID)
				require.Equal(t, tt.expectConfig.DeploymentToken, cfg.DeploymentToken)
				require.Equal(t, tt.expectConfig.Profile, cfg.Profile)
				require.Equal(t, tt.expectConfig.URL, cfg.URL)
			}
		})
	}
}
