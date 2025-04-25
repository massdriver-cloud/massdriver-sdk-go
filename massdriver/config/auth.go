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

type Auth struct {
	Method    AuthMethod
	Value     string
	AccountID string // Deployment ID or Org ID
	BaseURL   string
}

// ResolveAuth determines which authentication method to use, in priority order:
// 1. Environment variables MASSDRIVER_DEPLOYMENT_ID + MASSDRIVER_TOKEN
// 2. Environment variables MASSDRIVER_ORG_ID + MASSDRIVER_API_KEY
// 3. MASSDRIVER_PROFILE from ~/.massdriver/config (stubbed)
func ResolveAuth(cfg *Config) (*Auth, error) {
	if cfg.DeploymentID != "" && cfg.DeploymentToken != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(cfg.DeploymentID + ":" + cfg.DeploymentToken))
		return &Auth{
			Method:    AuthDeployment,
			Value:     "Basic " + encoded,
			AccountID: cfg.DeploymentID,
			BaseURL:   defaultMassdriverURL,
		}, nil
	}

	if cfg.OrgID != "" && cfg.APIKey != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(cfg.OrgID + ":" + cfg.APIKey))
		return &Auth{
			Method:    AuthAPIKey,
			Value:     "Basic " + encoded,
			AccountID: cfg.OrgID,
			BaseURL:   defaultMassdriverURL,
		}, nil
	}

	return ResolveProfileAuth()
}

// ResolveProfileAuth is stubbed for now. Later, it will read ~/.massdriver/config
// and return an API key + org ID based on MASSDRIVER_PROFILE or default.
func ResolveProfileAuth() (*Auth, error) {
	return nil, fmt.Errorf("no valid credentials found in environment; profile auth not yet implemented")
}
