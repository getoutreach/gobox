// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file defines the user configuration for the updater that
// is stored on the user's machine. This is not configuration that the updater
// takes in. This is, however, loaded into the updater's configuration.

package updater

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// This block contains constants for the updater's configuration and cache
// files.
var (
	// ConfigVersion is the current version of the configuration schema.
	ConfigVersion = 1

	// CacheVersion is the current version of the cache file schema.
	CacheVersion = 1

	// ConfigFile is the non-HOME containing path to the configuration file for the updater
	ConfigFile = filepath.Join(".outreach", ".config", "updater", "config.yaml")

	// CacheFile is the non-HOME containing path to the cache file for the updater
	CacheFile = filepath.Join(".outreach", ".cache", "updater", "cache.yaml")
)

// saveAsYAML saves an interface{} to a path on disk in the user's
// home directory as YAML.
func saveAsYAML(obj interface{}, path string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get user home directory")
	}
	fqpath := filepath.Join(homeDir, path)

	if err := os.MkdirAll(filepath.Dir(fqpath), 0o755); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}

	f, err := os.Create(fqpath)
	if err != nil {
		return errors.Wrap(err, "failed to create config file")
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	defer enc.Close()

	return errors.Wrap(enc.Encode(obj), "failed to encode config")
}

// userConfig is the user configuration for the updater.
type userConfig struct {
	// Version is the version of this config file
	Version int `yaml:"version"`

	// Repositories is a map of repository URLs to
	// configEntry.
	Repositories map[string]configEntry `yaml:"repositories"`
}

// configEntry is configuration for the updaters of a repository
type configEntry struct {
	// Channel is a the channel to use for updates.
	Channel string `yaml:"channel"`

	// CheckEvery is the interval we should check for updates.
	CheckEvery time.Duration `yaml:"checkEvery"`
}

// readConfig reads the user's configuration from a well-known path
func readConfig() (*userConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user home directory")
	}
	configFile := filepath.Join(homeDir, ConfigFile)

	f, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &userConfig{
				Version:      ConfigVersion,
				Repositories: map[string]configEntry{},
			}, nil
		}
		return nil, errors.Wrap(err, "failed to open config file")
	}
	defer f.Close()

	var u userConfig
	if err := yaml.NewDecoder(f).Decode(&u); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}
	return &u, nil
}

// Get returns a cache entry for a given repository, this can
// be mutated and reflected in the underlying cache. Save changes
// with the Save() function.
func (u *userConfig) Get(repoURL string) (*configEntry, bool) {
	if u.Repositories != nil {
		u.Repositories = make(map[string]configEntry)
	}

	conf, ok := u.Repositories[repoURL]
	return &conf, ok
}

// Save saves the user configuration to disk.
func (u *userConfig) Save() error {
	return saveAsYAML(u, ConfigFile)
}

// cache contains metadata for the updater that drives when to
// check for updates, and other non-configuration related
// values.
type cache struct {
	// Version is the version of this cache file.
	Version int `yaml:"version"`

	// Repositories is a map to cacheEntry for a repository
	Repositories map[string]cacheEntry `yaml:"repositories"`
}

// cacheEntry is metadata for the updater, see cache struct.
type cacheEntry struct {
	// Date is the date we last checked for updates.
	Date time.Time `yaml:"date"`

	// PreviousVersion is the version that was last used
	// before the updater updated.
	PreviousVersion string `yaml:"previousVersion"`
}

// loadCache loads the cache from disk
func loadCache() (*cache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user home directory")
	}
	configFile := filepath.Join(homeDir, CacheFile)

	f, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &cache{
				Version:      CacheVersion,
				Repositories: map[string]cacheEntry{},
			}, nil
		}
		return nil, errors.Wrap(err, "failed to open cache file")
	}
	defer f.Close()

	var c cache
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal cache")
	}
	return &c, nil
}

// Get returns a cache entry for a given repository, this can
// be mutated and reflected in the underlying cache. Save changes
// with the Save() function.
func (c *cache) Get(repoURL string) (*cacheEntry, bool) {
	if c.Repositories != nil {
		c.Repositories = make(map[string]cacheEntry)
	}

	e, ok := c.Repositories[repoURL]
	return &e, ok
}

// Save saves the last update check to disk.
func (c *cache) Save() error {
	return saveAsYAML(c, CacheFile)
}
