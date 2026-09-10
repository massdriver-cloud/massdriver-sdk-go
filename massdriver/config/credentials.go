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
	// id:token) issued by the platform to deployment workers; resolved
	// only on explicit opt-in via [Overrides.AuthMethod].
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

// String implements [fmt.Stringer] with Secret and AuthHeaderValue
// redacted, so a printed Credentials (or Config) can't leak a live
// credential. Read the fields directly for the real values.
func (c Credentials) String() string {
	return fmt.Sprintf("{Method:%s Source:%s ID:%s Secret:%s AuthHeaderValue:%s}",
		c.Method, c.Source, c.ID, redact(c.Secret), redact(c.AuthHeaderValue))
}

// GoString redacts %#v the same way.
func (c Credentials) GoString() string {
	return fmt.Sprintf("config.Credentials{Method:%q, Source:%q, ID:%q, Secret:%q, AuthHeaderValue:%q}",
		string(c.Method), string(c.Source), c.ID, redact(c.Secret), redact(c.AuthHeaderValue))
}

// redact keeps "unset" distinguishable from "set" without exposing
// the value.
func redact(s string) string {
	if s == "" {
		return ""
	}
	return "REDACTED"
}

// resolveCredentials resolves the credential for the requested
// [AuthMethod]. The default is API-key/PAT auth; deployment tokens
// (MASSDRIVER_DEPLOYMENT_ID + MASSDRIVER_TOKEN) resolve only when
// explicitly requested, never as an ambient fallback — they
// authenticate just the provisioning API subset.
//
// origin identifies which layer supplied an explicit API-key
// override; empty means "infer from envs vs profile."
func resolveCredentials(envs *configEnvs, profile *Profile, origin CredentialSource, method AuthMethod) (Credentials, error) {
	switch method {
	case AuthDeployment:
		return resolveDeploymentCredentials(envs)
	case "", AuthAPIKey, AuthPAT:
		return resolveAPIKeyCredentials(envs, profile, origin)
	default:
		return Credentials{}, fmt.Errorf("unsupported auth method: %q", method)
	}
}

func resolveDeploymentCredentials(envs *configEnvs) (Credentials, error) {
	if envs.DeploymentID == "" || envs.DeploymentToken == "" {
		return Credentials{}, ErrDeploymentCredentialsMissing
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(envs.DeploymentID + ":" + envs.DeploymentToken))
	return Credentials{
		Method:          AuthDeployment,
		Source:          SourceEnv,
		ID:              envs.DeploymentID,
		Secret:          envs.DeploymentToken,
		AuthHeaderValue: "Basic " + encoded,
	}, nil
}

func resolveAPIKeyCredentials(envs *configEnvs, profile *Profile, origin CredentialSource) (Credentials, error) {
	organizationID := cmp.Or(envs.OrganizationID, envs.OrgId, profile.OrganizationID)
	apiKey := cmp.Or(envs.APIKey, profile.APIKey)

	if apiKey == "" {
		if envs.DeploymentID != "" && envs.DeploymentToken != "" {
			return Credentials{}, fmt.Errorf("%w; MASSDRIVER_DEPLOYMENT_ID and MASSDRIVER_TOKEN are set, but deployment token authentication requires explicit opt-in (massdriver.WithDeploymentTokenAuth)", ErrNoCredentials)
		}
		return Credentials{}, ErrNoCredentials
	}
	if organizationID == "" {
		return Credentials{}, fmt.Errorf("%w for API key authentication", ErrOrganizationIDRequired)
	}

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
