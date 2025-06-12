package config

import (
	"encoding/base64"
	"fmt"
)

type AuthMethod string

const (
	AuthDeployment AuthMethod = "deployment"
	AuthAPIKey     AuthMethod = "api_key"
)

type Credentials struct {
	Method          AuthMethod
	ID              string
	Secret          string
	AuthHeaderValue string
}

// ResolveAuth determines which authentication method to use, in priority order:
// 1. Environment variables MASSDRIVER_DEPLOYMENT_ID + MASSDRIVER_TOKEN
// 2. Environment variables MASSDRIVER_ORG_ID + MASSDRIVER_API_KEY
// 3. MASSDRIVER_PROFILE from ~/.massdriver/config (stubbed)
func resolveAuth(cfg *Config) (*Credentials, error) {
	if cfg.DeploymentID != "" && cfg.DeploymentToken != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(cfg.DeploymentID + ":" + cfg.DeploymentToken))
		return &Credentials{
			Method:          AuthDeployment,
			ID:              cfg.DeploymentID,
			Secret:          cfg.DeploymentToken,
			AuthHeaderValue: "Basic " + encoded,
		}, nil
	}

	if cfg.OrganizationID != "" && cfg.APIKey != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(cfg.OrganizationID + ":" + cfg.APIKey))
		return &Credentials{
			Method:          AuthAPIKey,
			ID:              cfg.OrganizationID,
			Secret:          cfg.APIKey,
			AuthHeaderValue: "Basic " + encoded,
		}, nil
	}

	return resolveProfileAuth()
}

// ResolveProfileAuth is stubbed for now. Later, it will read ~/.massdriver/config
// and return an API key + org ID based on MASSDRIVER_PROFILE or default.
func resolveProfileAuth() (*Credentials, error) {
	return nil, fmt.Errorf("no valid credentials found in environment; profile auth not yet implemented")
}
