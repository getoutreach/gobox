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

// config is the configuration for the updater
type config struct {
	Version int `yaml:"version"`

	// GlobalConfig is the global configuration for the updater
	GlobalConfig *updateConfiguration `yaml:"global"`

	// PerRepositoryConfiguration is configuration for each repository
	PerRepositoryConfiguration map[string]*updateConfiguration `yaml:"perRepository"`

	// UpdaterCache contains the cache for the updater
	UpdaterCache map[string]updateCache `yaml:"cache,omitempty"`
}

type updateConfiguration struct {
	// CheckEvery is the interval at which the updater will check for updates
	// for the provided tool.
	CheckEvery time.Duration `yaml:"checkEvery,omitempty"`

	// Channel is the channel to use for this tool
	Channel string `yaml:"channel,omitempty"`
}

type updateCache struct {
	// LastChecked is the time this tool checked for an update
	LastChecked time.Time `yaml:"lastChecked,omitempty"`

	// LastVersion is the last version used before being updated.
	LastVersion string `yaml:"lastVersion,omitempty"`
}

// readConfig returns the configuration for the updater
func readConfig() (*config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	confPath := filepath.Join(homeDir, ConfigFile)
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		return &config{
			Version:                    ConfigVersion,
			GlobalConfig:               &updateConfiguration{},
			PerRepositoryConfiguration: make(map[string]*updateConfiguration),
			UpdaterCache:               make(map[string]updateCache),
		}, nil
	}

	f, err := os.Open(confPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conf config
	if err := yaml.NewDecoder(f).Decode(&conf); err != nil {
		return nil, err
	}

	if conf.GlobalConfig == nil {
		conf.GlobalConfig = &updateConfiguration{}
	}

	if conf.PerRepositoryConfiguration == nil {
		conf.PerRepositoryConfiguration = make(map[string]*updateConfiguration)
	}

	if conf.UpdaterCache == nil {
		conf.UpdaterCache = make(map[string]updateCache)
	}

	return &conf, err
}

// Get returns a copy of a specific repository's configuration
func (c *config) Get(repoURL string) (updateConfiguration, bool) {
	if c.PerRepositoryConfiguration == nil {
		return updateConfiguration{}, false
	}

	v, ok := c.PerRepositoryConfiguration[repoURL]
	if !ok {
		return updateConfiguration{}, false
	}

	return *v, true
}

// Set updates a repository's configuration, call Save() to
// save the changes
func (c *config) Set(repoURL string, conf *updateConfiguration) {
	c.PerRepositoryConfiguration[repoURL] = conf
}

// Save saves the changes to the configuration
func (c *config) Save() error {
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
