package config_test

import (
	"encoding/base64"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/stretchr/testify/require"
)

func TestResolveAuth(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.Config
		expectErr   bool
		expected    *config.Auth
		expectedEnc string
	}{
		{
			name: "Deployment Token Auth",
			cfg: config.Config{
				DeploymentID:    "deploy-123",
				DeploymentToken: "token-abc",
				OrgID:           "org789",
				APIKey:          "abc123",
			},
			expectErr: false,
			expected: &config.Auth{
				Method:    config.AuthDeployment,
				AccountID: "deploy-123",
			},
			expectedEnc: "Basic " + base64.StdEncoding.EncodeToString([]byte("deploy-123:token-abc")),
		},
		{
			name: "API Key Fallback Auth",
			cfg: config.Config{
				DeploymentID:    "",
				DeploymentToken: "",
				OrgID:           "org789",
				APIKey:          "abc123",
			},
			expectErr: false,
			expected: &config.Auth{
				Method:    config.AuthAPIKey,
				AccountID: "org789",
			},
			expectedEnc: "Basic " + base64.StdEncoding.EncodeToString([]byte("org789:abc123")),
		},
		{
			name: "Missing Credentials",
			cfg: config.Config{
				DeploymentID:    "",
				DeploymentToken: "",
				OrgID:           "",
				APIKey:          "",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := config.ResolveAuth(&tt.cfg)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, auth)
			} else {
				require.NoError(t, err)
				require.NotNil(t, auth)
				require.Equal(t, tt.expected.Method, auth.Method)
				require.Equal(t, tt.expected.AccountID, auth.AccountID)
				require.Equal(t, tt.expectedEnc, auth.Value)
			}
		})
	}
}
