// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file defines the user configuration for the updater that
// is stored on the user's machine. This is not configuration that the updater
// takes in. This is, however, loaded into the updater's configuration.

package updater

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// This block contains constants for the updater's configuration and cache
// files.
var (
	// ConfigVersion is the current version of the configuration schema.
	ConfigVersion = 1

	// ConfigFile is the non-HOME containing path to the config file for the updater
	ConfigFile = filepath.Join(".outreach", ".config", "updater", "config.yaml")
)

// Config is the configuration for the updater.
type Config struct {
	Version int `yaml:"version"`

	// GlobalConfig is the global configuration for the updater
	GlobalConfig *UpdateConfiguration `yaml:"global"`

	// PerRepositoryConfiguration is configuration for each repository
	PerRepositoryConfiguration map[string]*UpdateConfiguration `yaml:"perRepository"`

	// UpdaterCache contains the cache for the updater
	UpdaterCache map[string]updateCache `yaml:"cache,omitempty"`
}

// UpdateConfiguration is the configuration for a specific tool, or the
// global configuration.
type UpdateConfiguration struct {
	// CheckEvery is the interval at which the updater will check for updates
	// for the provided tool.
	CheckEvery time.Duration `yaml:"checkEvery,omitempty"`

	// Channel is the channel to use for this tool
	Channel string `yaml:"channel,omitempty"`

	// SkipPaths is a list of path substrings to skip when checking for updates.
	// For example, if you want to skip all installations from Homebrew, add the
	// path "/usr/local/Cellar/" to this list.
	// This is useful for tools that are installed in multiple ways, such as
	// Homebrew and mise. This specifically checks the absolute executable path,
	// as determined by `pkg/exec.ResolveExecutable()`.
	SkipPaths []string `yaml:"skipPaths,omitempty"`
}

type updateCache struct {
	// LastChecked is the time this tool checked for an update
	LastChecked time.Time `yaml:"lastChecked,omitempty"`

	// LastVersion is the last version used before being updated.
	LastVersion string `yaml:"lastVersion,omitempty"`
}

// ReadConfig loads the configuration for the updater.
func ReadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	confPath := filepath.Join(homeDir, ConfigFile)
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return &Config{
			Version:                    ConfigVersion,
			GlobalConfig:               &UpdateConfiguration{},
			PerRepositoryConfiguration: make(map[string]*UpdateConfiguration),
			UpdaterCache:               make(map[string]updateCache),
		}, nil
	}

	f, err := os.Open(confPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conf Config
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, err
	}

	if conf.GlobalConfig == nil {
		conf.GlobalConfig = &UpdateConfiguration{}
	}

	if conf.PerRepositoryConfiguration == nil {
		conf.PerRepositoryConfiguration = make(map[string]*UpdateConfiguration)
	}

	if conf.UpdaterCache == nil {
		conf.UpdaterCache = make(map[string]updateCache)
	}

	return &conf, err
}

// Get returns a copy of a specific repository's configuration.
func (c *Config) Get(repoURL string) (UpdateConfiguration, bool) {
	if c.PerRepositoryConfiguration == nil {
		return UpdateConfiguration{}, false
	}

	v, ok := c.PerRepositoryConfiguration[repoURL]
	if !ok {
		return UpdateConfiguration{}, false
	}

	return *v, true
}

// / GetGlobal returns a copy of the global configuration.
func (c *Config) GetGlobal() UpdateConfiguration {
	if c.GlobalConfig == nil {
		return UpdateConfiguration{}
	}

	return *c.GlobalConfig
}

// Set updates a repository's configuration. Call Save() to
// save the changes.
func (c *Config) Set(repoURL string, conf *UpdateConfiguration) {
	c.PerRepositoryConfiguration[repoURL] = conf
}

// SetGlobal updates the global configuration. Call Save() to
// save the changes.
func (c *Config) SetGlobal(conf *UpdateConfiguration) {
	c.GlobalConfig = conf
}

// Save saves the changes to the configuration.
func (c *Config) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	confPath := filepath.Join(homeDir, ConfigFile)
	if _, err := os.Stat(confPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(confPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(confPath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	defer enc.Close()

	return enc.Encode(c)
}
