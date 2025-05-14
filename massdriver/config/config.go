package config

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/kelseyhightower/envconfig"
)

const defaultURL = "https://api.massdriver.cloud"

type Config struct {
	OrgID           string `json:"organization_id" envconfig:"ORG_ID"`
	APIKey          string `json:"api_key" envconfig:"API_KEY"`
	DeploymentID    string `json:"deployment_id" envconfig:"DEPLOYMENT_ID"`
	DeploymentToken string `json:"deployment_token" envconfig:"TOKEN"`
	Profile         string `json:"profile" envconfig:"PROFILE"`
	URL             string `json:"url" envconfig:"URL"`
}

func Get() (*Config, error) {
	cfg := Config{}
	err := envconfig.Process("massdriver", &cfg)
	if err != nil {
		return nil, fmt.Errorf("error initializing configuration: %w", err)
	}

	uuidErr := uuid.Validate(cfg.OrgID)
	if uuidErr == nil {
		fmt.Printf("environment variable MASSDRIVER_ORG_ID is a UUID. This is deprecated and will be removed in a future release. Please use the organization abbreviation instead.\n")
	}

	if cfg.URL == "" {
		cfg.URL = defaultURL
	}

	return &cfg, nil
}
