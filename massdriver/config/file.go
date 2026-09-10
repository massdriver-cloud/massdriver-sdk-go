package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// Version is the config file schema version this package understands;
// [ReadFile] rejects any other value.
//
// Add new optional fields within version 1 rather than bumping this:
// an older binary rejects a file declaring a version it doesn't know,
// so a bump breaks every released SDK and CLI against a new file.
const Version = 1

// DefaultProfileName is the profile [Load] falls back to when nothing
// selects one.
const DefaultProfileName = "default"

const configPathFromConfigDir = "massdriver/config.yaml"

// Profile is one named set of credentials and settings in the config
// file.
type Profile struct {
	OrganizationID string `json:"organization_id" yaml:"organization_id"`
	APIKey         string `json:"api_key" yaml:"api_key"`
	URL            string `json:"url" yaml:"url"`
	TemplatesPath  string `json:"templates_path" yaml:"templates_path"`
}

// File is the on-disk config file at [FilePath]. It is exported so
// tools that write it — principally the Massdriver CLI — share this
// schema rather than redeclaring and drifting from it. This package
// only reads; writing it (and preserving the user's comments and key
// order while doing so) is the caller's job.
type File struct {
	Version        int                `json:"version" yaml:"version"`
	CurrentProfile string             `json:"current_profile" yaml:"current_profile"`
	Profiles       map[string]Profile `json:"profiles" yaml:"profiles"`
}

// ProfileNames returns the file's profile names, sorted.
func (f *File) ProfileNames() []string {
	if f == nil {
		return nil
	}
	names := make([]string, 0, len(f.Profiles))
	for name := range f.Profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// FilePath returns the path to the Massdriver config file, which need
// not exist: $XDG_CONFIG_HOME/massdriver/config.yaml when
// XDG_CONFIG_HOME is set, otherwise ~/.config/massdriver/config.yaml.
// Callers that write the file should locate it through this function
// so they agree with the SDK about which file is in play.
func FilePath() (string, error) {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, configPathFromConfigDir), nil
	}

	homeDir, homeDirErr := os.UserHomeDir()
	if homeDirErr != nil {
		return "", fmt.Errorf("could not determine home directory: %w", homeDirErr)
	}
	return filepath.Join(homeDir, ".config", configPathFromConfigDir), nil
}

// ReadFile reads and parses the config file at [FilePath], returning
// (nil, nil) if it does not exist — credentials may come entirely from
// the environment.
func ReadFile() (*File, error) {
	configFilePath, pathErr := FilePath()
	if pathErr != nil {
		return nil, pathErr
	}

	contents, readErr := os.ReadFile(configFilePath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			// quietly return nil if the config file does not exist
			return nil, nil
		}
		return nil, fmt.Errorf("could not read config file %s: %w", configFilePath, readErr)
	}

	var cfg File
	if yamlErr := yaml.Unmarshal(contents, &cfg); yamlErr != nil {
		return nil, fmt.Errorf("could not unmarshal config file %s: %w", configFilePath, yamlErr)
	}

	if cfg.Version != Version {
		return nil, fmt.Errorf("unsupported config file version: %d  expected version %d", cfg.Version, Version)
	}

	return &cfg, nil
}
