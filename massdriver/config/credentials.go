package config

import (
	"encoding/base64"
	"fmt"
	"strings"
)

type AuthMethod string
type CredentialSource string

const (
	AuthDeployment AuthMethod = "deployment"
	AuthAPIKey     AuthMethod = "api_key"
	AuthPAT        AuthMethod = "personal_access_token"
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
func resolveCredentials(envs *configEnvs, profile *configFileProfile) (*Credentials, error) {
	if envs.DeploymentID != "" && envs.DeploymentToken != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(envs.DeploymentID + ":" + envs.DeploymentToken))
		return &Credentials{
			Method:          AuthDeployment,
			ID:              envs.DeploymentID,
			Secret:          envs.DeploymentToken,
			AuthHeaderValue: "Basic " + encoded,
		}, nil
	}

	organizationID := coalesceString(envs.OrganizationID, envs.OrgId, profile.OrganizationID)
	apiKey := coalesceString(envs.APIKey, profile.APIKey)
	if organizationID != "" && apiKey != "" {
		if strings.HasPrefix(apiKey, "mds_") || strings.HasPrefix(apiKey, "md_") {
			return &Credentials{
				Method:          AuthPAT,
				ID:              organizationID,
				Secret:          apiKey,
				AuthHeaderValue: "Bearer " + apiKey,
			}, nil
		}

		encoded := base64.StdEncoding.EncodeToString([]byte(organizationID + ":" + apiKey))
		return &Credentials{
			Method:          AuthAPIKey,
			ID:              organizationID,
			Secret:          apiKey,
			AuthHeaderValue: "Basic " + encoded,
		}, nil
	}

	return nil, fmt.Errorf("no credentials found")
}
