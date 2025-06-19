package bundle_test

import (
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/bundle"
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
	}{
		{
			name: "valid config",
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

			if remoteRepo.Reference.Registry != tt.expectedHost {
				t.Errorf("expected registry %q, got %q", tt.expectedHost, remoteRepo.Reference.Registry)
			}
			if remoteRepo.Reference.Repository != tt.expectedPath {
				t.Errorf("expected repository path %q, got %q", tt.expectedPath, remoteRepo.Reference.Repository)
			}
			if remoteRepo.Client == nil {
				t.Error("expected auth client to be set")
			}
		})
	}
}
