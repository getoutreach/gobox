// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file defines the user configuration for the updater.

package updater

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// userConfig is the user configuration for the updater.
type userConfig struct {
	// path is the path to this user configuration
	path string

	// AlwaysUsePrereleases instructs the updater to always consider prereleases
	AlwaysUsePrereleases bool `yaml:"alwaysUsePrereleases"`
}

// readConfig reads the user's configuration from a well-known path
func readConfig(repo string) (userConfig, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return userConfig{}, errors.Wrap(err, "failed to get the user's home directory")
	}

	configPath := filepath.Join(homedir, configDir, repo, "updater.yaml")

	f, err := os.Open(configPath)
	if err != nil {
		defaultConfig := userConfig{path: configPath}
		if errors.Is(err, os.ErrNotExist) {
			return defaultConfig, nil
		}
		return defaultConfig, err
	}

	var config userConfig
	if err := yaml.NewDecoder(f).Decode(&config); err != nil {
		return userConfig{}, errors.Wrap(err, "failed to decode user config")
	}
	config.path = configPath
	return config, nil
}

// Save saves the user configuration to disk.
func (u *userConfig) Save() error {
	if err := os.MkdirAll(filepath.Dir(u.path), 0o755); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}

	f, err := os.Create(u.path)
	if err != nil {
		return errors.Wrap(err, "failed to create config file")
	}
	defer f.Close()

	return errors.Wrap(yaml.NewEncoder(f).Encode(u), "failed to encode config")
}

// lastUpdateCheck is information about the last time we checked for updates.
type lastUpdateCheck struct {
	// path is the path to this last update check
	path string

	// Date is the date we last checked for updates.
	Date time.Time `yaml:"date"`

	// CheckEvery is the interval we should check for updates.
	CheckEvery time.Duration `yaml:"checkEvery"`

	// PreviousVersion is the version that was used before this.
	PreviousVersion string `yaml:"previousVersion"`
}

// loadLastUpdateCheck loads the last update check from disk.
func loadLastUpdateCheck(repo string) (*lastUpdateCheck, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the user's home directory")
	}

	updatePath := filepath.Join(homedir, cacheDir, repo, "updater.yaml")

	if err := os.MkdirAll(filepath.Dir(updatePath), 0o755); err != nil {
		return nil, errors.Wrap(err, "failed to create update metadata storage directory")
	}

	// check the last time we updated
	b, err := os.ReadFile(updatePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.Wrap(err, "failed to read the last update check")
		}
		return &lastUpdateCheck{path: updatePath}, nil
	}

	var last lastUpdateCheck
	if err := yaml.Unmarshal(b, &last); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the last update check")
	}
	last.path = updatePath

	return &last, nil
}

// Save saves the last update check to disk.
func (u *lastUpdateCheck) Save() error {
	if err := os.MkdirAll(filepath.Dir(u.path), 0o755); err != nil {
		return errors.Wrap(err, "failed to create update cache storage directory")
	}

	f, err := os.Create(u.path)
	if err != nil {
		return errors.Wrap(err, "failed to create update cache file")
	}
	defer f.Close()

	return errors.Wrap(yaml.NewEncoder(f).Encode(u), "failed to encode update cache")
}
