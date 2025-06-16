package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

const defaultURL = "https://api.massdriver.cloud"

type configFileProfile struct {
	OrganizationID string `json:"organization_id" yaml:"organization_id"`
	APIKey         string `json:"api_key" yaml:"api_key"`
	URL            string `json:"url" yaml:"url"`
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
}

type Config struct {
	Credentials    *Credentials
	OrganizationID string
	Profile        string
	URL            string
}

func Get() (*Config, error) {

	cfg, initErr := initializeConfig()
	if initErr != nil {
		return nil, fmt.Errorf("error initializing configuration: %w", initErr)
	}

	validateErr := validateConfig(cfg)
	if validateErr != nil {
		return nil, fmt.Errorf("configuration is invalid: %w", validateErr)
	}

	return cfg, nil
}

func initializeConfig() (*Config, error) {
	cfg := Config{}

	envs, envsErr := getConfigEnvs()
	if envsErr != nil {
		return nil, envsErr
	}

	// Ignore error from getConfigFile since it isn't mandatory. Eventually we can issue warnings here if needed.
	profile := configFileProfile{}
	configFile, _ := getConfigFile()
	if configFile != nil && configFile.Profiles != nil {
		profileName := envs.Profile
		if profileName == "" {
			profileName = "default"
		}

		if profileConfig, exists := configFile.Profiles[profileName]; exists {
			profile = profileConfig
			cfg.Profile = profileName
		}
	}

	cfg.OrganizationID = coalesceString(envs.OrganizationID, envs.OrgId, profile.OrganizationID)
	cfg.URL = coalesceString(envs.URL, profile.URL, defaultURL)

	credentials, credErr := resolveCredentials(envs, &profile)
	if credErr != nil {
		return nil, fmt.Errorf("error resolving credentials: %w", credErr)
	}
	cfg.Credentials = credentials

	return &cfg, nil
}

func getConfigFile() (*configFile, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	configPath := homeDir + "/.massdriver/config.yaml"
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("could not read config file %s: %w", configPath, err)
	}

	var cfg configFile
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("could not unmarshal config file %s: %w", configPath, err)
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

func validateConfig(cfg *Config) error {
	if cfg.OrganizationID == "" {
		return fmt.Errorf("organization ID is required")
	}

	if cfg.Credentials == nil || cfg.Credentials.ID == "" || cfg.Credentials.Secret == "" {
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

func coalesceString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
