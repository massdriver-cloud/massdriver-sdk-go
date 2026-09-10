package config

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
)

const defaultURL = "https://api.massdriver.cloud"

type configEnvs struct {
	OrganizationID  string `json:"organization_id" yaml:"organization_id" envconfig:"MASSDRIVER_ORGANIZATION_ID"`
	OrgId           string `json:"org_id" yaml:"org_id" envconfig:"MASSDRIVER_ORG_ID"`
	APIKey          string `json:"api_key" yaml:"api_key" envconfig:"MASSDRIVER_API_KEY"`
	DeploymentID    string `json:"deployment_id" yaml:"deployment_id" envconfig:"MASSDRIVER_DEPLOYMENT_ID"`
	DeploymentToken string `json:"deployment_token" yaml:"deployment_token" envconfig:"MASSDRIVER_TOKEN"`
	Profile         string `json:"profile" yaml:"profile" envconfig:"MASSDRIVER_PROFILE"`
	URL             string `json:"url" yaml:"url" envconfig:"MASSDRIVER_URL"`
	TemplatesPath   string `json:"templates_path" yaml:"templates_path" envconfig:"MASSDRIVER_TEMPLATES_PATH"`
}

type Config struct {
	Credentials    Credentials
	OrganizationID string
	// Profile is the config-file profile that supplied the values
	// below; empty if no profile was loaded.
	Profile string
	URL     string
	// TemplatesPath is the directory the Massdriver CLI uses to
	// scaffold new bundles. The SDK itself does not consume this
	// field — it is loaded for the benefit of CLI tools that share
	// this config-resolution code. Safe to ignore in non-CLI usage.
	TemplatesPath string
}

// Overrides are caller-supplied values that win over environment
// variables and the config file. Empty strings mean "no override —
// fall back to env/file."
type Overrides struct {
	APIKey         string
	OrganizationID string
	URL            string
	Profile        string
	// AuthMethod selects the credential kind: empty resolves
	// API-key/PAT auth; [AuthDeployment] opts into the deployment
	// token env vars.
	AuthMethod AuthMethod
}

// Get is shorthand for [Load](Overrides{}).
func Get() (Config, error) {
	return Load(Overrides{})
}

// Load resolves a [Config] from environment variables, the active
// profile in the config file at [FilePath], and the supplied
// [Overrides] (highest precedence).
//
// The active profile is [Overrides.Profile], else MASSDRIVER_PROFILE,
// else the file's current_profile, else [DefaultProfileName]. The
// first three name a profile explicitly and fail with
// [ErrProfileNotFound] if it doesn't exist, rather than falling back
// to whatever credentials the environment holds. The
// [DefaultProfileName] fallback may be absent — that is how a
// caller configured purely through environment variables resolves.
//
// [AuthDeployment] is exempt: its credentials come only from the
// environment, so a stale profile name never blocks a provisioner.
func Load(o Overrides) (Config, error) {
	cfg, initErr := initializeConfig(o)
	if initErr != nil {
		return Config{}, fmt.Errorf("error initializing configuration: %w", initErr)
	}

	validateErr := validateConfig(cfg)
	if validateErr != nil {
		return Config{}, fmt.Errorf("configuration is invalid: %w", validateErr)
	}

	return cfg, nil
}

func initializeConfig(o Overrides) (Config, error) {
	cfg := Config{}

	configEnvs, configEnvsErr := getConfigEnvs()
	if configEnvsErr != nil {
		return Config{}, fmt.Errorf("error reading environment configuration: %w", configEnvsErr)
	}

	// Apply overrides on top of env-sourced values. Overrides win.
	// Track whether the API key was supplied via an option so the
	// resolved Credentials carries the right Source.
	var apiKeyOrigin CredentialSource
	if o.OrganizationID != "" {
		configEnvs.OrganizationID = o.OrganizationID
	}
	if o.APIKey != "" {
		configEnvs.APIKey = o.APIKey
		apiKeyOrigin = SourceOption
	}
	if o.URL != "" {
		configEnvs.URL = o.URL
	}
	// Remember which layer named the profile so a miss can say so.
	envProfileOrigin := "MASSDRIVER_PROFILE"
	if o.Profile != "" {
		configEnvs.Profile = o.Profile
		envProfileOrigin = "the Profile option"
	}

	profile := Profile{}
	configFile, configFileErr := ReadFile()
	if configFileErr != nil {
		return Config{}, fmt.Errorf("error reading config file: %w", configFileErr)
	}

	selection := selectProfile(configEnvs.Profile, envProfileOrigin, configFile)
	if profileConfig, exists := configFile.lookup(selection.name); exists {
		profile = profileConfig
		cfg.Profile = selection.name
	} else if selection.explicit && o.AuthMethod != AuthDeployment {
		// Deployment tokens come from the environment and never from a
		// profile, so a stale profile name must not block a provisioner.
		return Config{}, selection.notFoundError(configFile)
	}

	cfg.OrganizationID = cmp.Or(configEnvs.OrganizationID, configEnvs.OrgId, profile.OrganizationID)
	cfg.URL = cmp.Or(configEnvs.URL, profile.URL, defaultURL)
	cfg.TemplatesPath = cmp.Or(configEnvs.TemplatesPath, profile.TemplatesPath)

	credentials, credErr := resolveCredentials(configEnvs, &profile, apiKeyOrigin, o.AuthMethod)
	if credErr != nil {
		return Config{}, fmt.Errorf("error resolving credentials: %w", credErr)
	}
	cfg.Credentials = credentials

	return cfg, nil
}

type profileSelection struct {
	name string
	// explicit is false only for the DefaultProfileName fallback,
	// which is the one selection allowed to miss.
	explicit bool
	origin   string // the layer that asked, for the not-found error
}

// selectProfile applies the precedence documented on [Load].
// envProfile holds the option and env layers already merged, with
// envOrigin naming whichever supplied it.
func selectProfile(envProfile, envOrigin string, file *File) profileSelection {
	if envProfile != "" {
		return profileSelection{name: envProfile, explicit: true, origin: envOrigin}
	}
	if file != nil && file.CurrentProfile != "" {
		return profileSelection{name: file.CurrentProfile, explicit: true, origin: "current_profile in the config file"}
	}
	return profileSelection{name: DefaultProfileName}
}

func (s profileSelection) notFoundError(file *File) error {
	path, pathErr := FilePath()
	if pathErr != nil {
		path = "the config file"
	}

	switch names := file.ProfileNames(); {
	case file == nil:
		return fmt.Errorf("%w: %q was requested by %s, but no config file exists at %s",
			ErrProfileNotFound, s.name, s.origin, path)
	case len(names) == 0:
		return fmt.Errorf("%w: %q was requested by %s, but %s defines no profiles",
			ErrProfileNotFound, s.name, s.origin, path)
	default:
		return fmt.Errorf("%w: %q was requested by %s, but %s defines only %s",
			ErrProfileNotFound, s.name, s.origin, path, strings.Join(names, ", "))
	}
}

func (f *File) lookup(name string) (Profile, bool) {
	if f == nil {
		return Profile{}, false
	}
	p, exists := f.Profiles[name]
	return p, exists
}

func getConfigEnvs() (*configEnvs, error) {
	envs := new(configEnvs)
	envErr := envconfig.Process("", envs)
	if envErr != nil {
		return nil, fmt.Errorf("error processing environment variables: %w", envErr)
	}
	return envs, nil
}

func validateConfig(cfg Config) error {
	if cfg.OrganizationID == "" {
		return ErrOrganizationIDRequired
	}

	if cfg.Credentials.ID == "" || cfg.Credentials.Secret == "" {
		return fmt.Errorf("credentials are required")
	}

	uuidErr := uuid.Validate(cfg.OrganizationID)
	if uuidErr == nil {
		return fmt.Errorf("organization ID is a UUID. This is deprecated and will be removed in a future release, please use the organization abbreviation instead")
	}

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("url must include scheme and host (e.g., https://api.massdriver.cloud)")
	}

	return nil
}
