package bundle_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/bundle"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote"
)

func TestGetBundleRepository(t *testing.T) {
	tests := []struct {
		name         string
		cfg          config.Config
		bundleName   string
		expectErr    bool
		expectedHost string
		expectedPath string
		expectedHTTP bool
	}{
		{
			name: "valid config with HTTPS",
			cfg: config.Config{
				Credentials: &config.Credentials{
					ID:     "test-org",
					Secret: "test-secret",
				},
				OrganizationID: "test-org",
				URL:            "https://registry.example.com",
			},
			bundleName:   "test-bundle",
			expectErr:    false,
			expectedHost: "registry.example.com",
			expectedPath: "test-org/test-bundle",
			expectedHTTP: false,
		},
		{
			name: "valid config with HTTP",
			cfg: config.Config{
				Credentials: &config.Credentials{
					ID:              "test-org",
					Secret:          "test-secret",
					AuthHeaderValue: "Basic dGVzdC1vcmc6dGVzdC1zZWNyZXQ=",
				},
				OrganizationID: "test-org",
				URL:            "http://registry.example.com",
			},
			bundleName:   "test-bundle",
			expectErr:    false,
			expectedHost: "registry.example.com",
			expectedPath: "test-org/test-bundle",
			expectedHTTP: true,
		},
		{
			name: "invalid URL",
			cfg: config.Config{
				Credentials: &config.Credentials{
					ID:     "test-org",
					Secret: "test-secret",
				},
				OrganizationID: "test-org",
				URL:            "://bad-url",
			},
			bundleName: "test-bundle",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mdClient := &client.Client{
				Config: tt.cfg,
			}
			repo, err := bundle.GetBundleRepository(mdClient, tt.bundleName)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			remoteRepo, ok := repo.(*remote.Repository)
			if !ok {
				t.Fatalf("expected *remote.Repository, got %T", repo)
			}

			require.Equal(t, tt.expectedHost, remoteRepo.Reference.Registry)
			require.Equal(t, tt.expectedPath, remoteRepo.Reference.Repository)
			require.NotNil(t, remoteRepo.Client, "expected auth client to be set")
			require.Equal(t, tt.expectedHTTP, remoteRepo.PlainHTTP)
		})
	}
}
