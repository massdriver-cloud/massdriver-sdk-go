package config

import (
	"cmp"
	"encoding/base64"
	"fmt"
	"strings"
)

// AuthMethod identifies the kind of credential a [Credentials] value
// represents. It answers "what kind of secret is this?" — distinct
// from [CredentialSource], which answers "where did the secret come
// from?"
type AuthMethod string

const (
	// AuthDeployment is a Massdriver deployment token (Basic auth,
	// id:token) issued by the platform to deployment workers.
	AuthDeployment AuthMethod = "deployment"
	// AuthAPIKey is a legacy API key paired with an organization id
	// (Basic auth, orgID:key).
	AuthAPIKey AuthMethod = "api_key"
	// AuthPAT is a personal access token, identified by the "mds_" or
	// "md_" prefix and authenticated via Bearer auth.
	AuthPAT AuthMethod = "personal_access_token"
)

// CredentialSource identifies which configuration layer supplied the
// credential the SDK resolved. Useful for diagnostics — e.g., when a
// user reports "wrong credentials," AuthSource tells you whether the
// SDK picked them up from an option, an environment variable, or the
// config file.
//
// Mirrors AWS SDK's `aws.Credentials.Source` field. Sources are
// reported in priority order — the first non-empty layer wins.
type CredentialSource string

const (
	// SourceUnknown means no credentials have been resolved (typically
	// the test path, where [WithGQLClient] bypasses auth entirely).
	SourceUnknown CredentialSource = ""
	// SourceOption indicates the credential came from a functional
	// option passed to NewClient (e.g., [massdriver.WithAPIKey]).
	SourceOption CredentialSource = "option"
	// SourceEnv indicates the credential came from an environment
	// variable (MASSDRIVER_API_KEY, MASSDRIVER_TOKEN, etc.).
	SourceEnv CredentialSource = "env"
	// SourceProfile indicates the credential came from the active
	// profile in ~/.config/massdriver/config.yaml.
	SourceProfile CredentialSource = "profile"
)

type Credentials struct {
	Method          AuthMethod
	Source          CredentialSource
	ID              string
	Secret          string
	AuthHeaderValue string
}

// resolveCredentials picks an authentication method, in priority order:
//
//  1. MASSDRIVER_DEPLOYMENT_ID + MASSDRIVER_TOKEN env vars (Basic auth,
//     used by deployment workers).
//  2. An organization id (env var, ORG_ID alias, or active profile) plus
//     an API key (option override, env var, or profile). Keys prefixed
//     "mds_" or "md_" are personal access tokens (Bearer auth); other
//     values are legacy API keys (Basic auth).
//
// origin identifies which layer the API key came from when the
// option-or-env-or-profile path is taken; for the deployment path it
// is always [SourceEnv]. Empty origin means "infer from envs vs
// profile" — used when the caller didn't pass an explicit override.
//
// Returns ("no credentials found") if neither path supplies a usable
// pair.
func resolveCredentials(envs *configEnvs, profile *configFileProfile, origin CredentialSource) (Credentials, error) {
	if envs.DeploymentID != "" && envs.DeploymentToken != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(envs.DeploymentID + ":" + envs.DeploymentToken))
		return Credentials{
			Method:          AuthDeployment,
			Source:          SourceEnv,
			ID:              envs.DeploymentID,
			Secret:          envs.DeploymentToken,
			AuthHeaderValue: "Basic " + encoded,
		}, nil
	}

	organizationID := cmp.Or(envs.OrganizationID, envs.OrgId, profile.OrganizationID)
	apiKey := cmp.Or(envs.APIKey, profile.APIKey)
	if organizationID != "" && apiKey != "" {
		source := origin
		if source == SourceUnknown {
			// No option set the API key. Infer from the layer that had
			// a non-empty value — apiKey came from cmp.Or(envs, profile),
			// so exactly one of these branches must hit.
			if envs.APIKey != "" {
				source = SourceEnv
			} else {
				source = SourceProfile
			}
		}
		if strings.HasPrefix(apiKey, "mds_") || strings.HasPrefix(apiKey, "md_") {
			return Credentials{
				Method:          AuthPAT,
				Source:          source,
				ID:              organizationID,
				Secret:          apiKey,
				AuthHeaderValue: "Bearer " + apiKey,
			}, nil
		}

		encoded := base64.StdEncoding.EncodeToString([]byte(organizationID + ":" + apiKey))
		return Credentials{
			Method:          AuthAPIKey,
			Source:          source,
			ID:              organizationID,
			Secret:          apiKey,
			AuthHeaderValue: "Basic " + encoded,
		}, nil
	}

	return Credentials{}, fmt.Errorf("no credentials found")
}
