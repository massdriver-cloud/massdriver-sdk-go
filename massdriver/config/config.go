package config

import (
	"fmt"
	"net/url"
	"os"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
)

const defaultURL = "https://api.massdriver.cloud"

type Config struct {
	Credentials     *Credentials
	OrganizationID  string `json:"organization_id" envconfig:"ORGANIZATION_ID"`
	APIKey          string `json:"api_key" envconfig:"API_KEY"`
	DeploymentID    string `json:"deployment_id" envconfig:"DEPLOYMENT_ID"`
	DeploymentToken string `json:"deployment_token" envconfig:"TOKEN"`
	Profile         string `json:"profile" envconfig:"PROFILE"`
	URL             string `json:"url" envconfig:"URL"`
}

func Get() (*Config, error) {
	cfg := Config{}
	envErr := envconfig.Process("massdriver", &cfg)
	if envErr != nil {
		return nil, fmt.Errorf("error initializing configuration: %w", envErr)
	}

	auth, authErr := resolveAuth(&cfg)
	if authErr != nil {
		return nil, fmt.Errorf("error resolving credentials: %w", authErr)
	}
	cfg.Credentials = auth

	if cfg.URL == "" {
		cfg.URL = defaultURL
	}

	cfg.initializeOrganizationID()

	validateErr := validateConfig(&cfg)
	if validateErr != nil {
		return nil, fmt.Errorf("configuration is invalid: %w", validateErr)
	}

	return &cfg, nil
}

func (c *Config) initializeOrganizationID() {
	if c.OrganizationID == "" {
		os.Getenv("MASSDRIVER_ORG_ID")
	}
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
