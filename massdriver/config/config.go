package config

import (
	"cmp"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

const defaultURL = "https://api.massdriver.cloud"
const configPathFromConfigDir = "massdriver/config.yaml"

type configFileProfile struct {
	OrganizationID string `json:"organization_id" yaml:"organization_id"`
	APIKey         string `json:"api_key" yaml:"api_key"`
	URL            string `json:"url" yaml:"url"`
	TemplatesPath  string `json:"templates_path" yaml:"templates_path"`
}
type configFile struct {
	Version  int                          `json:"version" yaml:"version"`
	Profiles map[string]configFileProfile `json:"profiles" yaml:"profiles"`
}

type configEnvs struct {
	OrganizationID  string `json:"organization_id" yaml:"organization_id" envconfig:"ORGANIZATION_ID"`
	OrgId           string `json:"org_id" yaml:"org_id" envconfig:"ORG_ID"`
	APIKey          string `json:"api_key" yaml:"api_key" envconfig:"API_KEY"`
	DeploymentID    string `json:"deployment_id" yaml:"deployment_id" envconfig:"DEPLOYMENT_ID"`
	DeploymentToken string `json:"deployment_token" yaml:"deployment_token" envconfig:"TOKEN"`
	Profile         string `json:"profile" yaml:"profile" envconfig:"PROFILE"`
	URL             string `json:"url" yaml:"url" envconfig:"URL"`
	TemplatesPath   string `json:"templates_path" yaml:"templates_path" envconfig:"TEMPLATES_PATH"`
}

type Config struct {
	Credentials    Credentials
	OrganizationID string
	Profile        string
	URL            string
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
}

// Get is shorthand for [Load](Overrides{}).
func Get() (Config, error) {
	return Load(Overrides{})
}

// Load resolves a [Config] from environment variables, the active
// profile in ~/.config/massdriver/config.yaml, and the supplied
// [Overrides] (highest precedence).
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
	if o.Profile != "" {
		configEnvs.Profile = o.Profile
	}

	profile := configFileProfile{}
	configFile, configFileErr := getConfigFile()
	if configFileErr != nil {
		return Config{}, fmt.Errorf("error reading config file: %w", configFileErr)
	}
	if configFile != nil && configFile.Profiles != nil {
		profileName := configEnvs.Profile
		if profileName == "" {
			profileName = "default"
		}

		if profileConfig, exists := configFile.Profiles[profileName]; exists {
			profile = profileConfig
			cfg.Profile = profileName
		}
	}

	cfg.OrganizationID = cmp.Or(configEnvs.OrganizationID, configEnvs.OrgId, profile.OrganizationID)
	cfg.URL = cmp.Or(configEnvs.URL, profile.URL, defaultURL)
	cfg.TemplatesPath = cmp.Or(configEnvs.TemplatesPath, profile.TemplatesPath)

	credentials, credErr := resolveCredentials(configEnvs, &profile, apiKeyOrigin)
	if credErr != nil {
		return Config{}, fmt.Errorf("error resolving credentials: %w", credErr)
	}
	cfg.Credentials = credentials

	return cfg, nil
}

func getConfigFile() (*configFile, error) {
	var configFilePath string

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		configFilePath = filepath.Join(xdgConfigHome, configPathFromConfigDir)
	} else {

		homeDir, homeDirErr := os.UserHomeDir()
		if homeDirErr != nil {
			return nil, fmt.Errorf("could not determine home directory: %w", homeDirErr)
		}
		configFilePath = filepath.Join(homeDir, ".config", configPathFromConfigDir)
	}

	file, readErr := os.ReadFile(configFilePath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			// quietly return nil if the config file does not exist
			return nil, nil
		}
		return nil, fmt.Errorf("could not read config file %s: %w", configFilePath, readErr)
	}

	var cfg configFile
	if yamlErr := yaml.Unmarshal(file, &cfg); yamlErr != nil {
		return nil, fmt.Errorf("could not unmarshal config file %s: %w", configFilePath, yamlErr)
	}

	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported config file version: %d  expected version 1", cfg.Version)
	}

	return &cfg, nil
}

func getConfigEnvs() (*configEnvs, error) {
	envs := new(configEnvs)
	envErr := envconfig.Process("massdriver", envs)
	if envErr != nil {
		return nil, fmt.Errorf("error processing environment variables: %w", envErr)
	}
	return envs, nil
}

func validateConfig(cfg Config) error {
	if cfg.OrganizationID == "" {
		return fmt.Errorf("organization ID is required")
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

