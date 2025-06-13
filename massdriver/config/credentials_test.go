package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveAuth(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		expectErr bool
		expected  *Credentials
	}{
		{
			name: "Deployment Token Auth",
			cfg: Config{
				DeploymentID:    "deploy-123",
				DeploymentToken: "token-abc",
				OrganizationID:  "org789",
				APIKey:          "abc123",
			},
			expectErr: false,
			expected: &Credentials{
				Method:          AuthDeployment,
				ID:              "deploy-123",
				Secret:          "token-abc",
				AuthHeaderValue: "Basic ZGVwbG95LTEyMzp0b2tlbi1hYmM=",
			},
		},
		{
			name: "API Key Fallback Auth",
			cfg: Config{
				DeploymentID:    "",
				DeploymentToken: "",
				OrganizationID:  "org789",
				APIKey:          "abc123",
			},
			expectErr: false,
			expected: &Credentials{
				Method:          AuthAPIKey,
				ID:              "org789",
				Secret:          "abc123",
				AuthHeaderValue: "Basic b3JnNzg5OmFiYzEyMw==",
			},
		},
		{
			name: "Missing Credentials",
			cfg: Config{
				DeploymentID:    "",
				DeploymentToken: "",
				OrganizationID:  "",
				APIKey:          "",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := resolveAuth(&tt.cfg)

			if tt.expectErr {
				require.Error(t, err)
				require.Nil(t, auth)
			} else {
				require.NoError(t, err)
				require.NotNil(t, auth)
				require.Equal(t, tt.expected, auth)
			}
		})
	}
}
