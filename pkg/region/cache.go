// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains the read through cache implementation for
// this package.
package region

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var (
	// RegionCachePath is the $HOME/<RegionCachePath> location of the regionpicker cache
	RegionCachePath = filepath.Join(".outreach", ".cache", "box")
	// RegionCacheFile is the name of the regionpicker cache file
	RegionCacheFile = "regions.json"
)

// cache is the package specific cache instance used. This is a global variable to
// ensure that this is go-routine safe.
var cache = &cacheStore{}

// cacheEntry is information on a cloud's cache entry
type cacheEntry struct {
	// Duration is how long it took to talk to this region at LastUpdatedAt
	Duration time.Duration `json:"duration"`

	// LastUpdatedAt is the last time this file was updated. UTC.
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

type cacheStore struct {
	// Clouds is a map of cloudName to cache entries
	// cloud -> region -> cacheEntry
	Clouds map[CloudName]map[Name]cacheEntry `json:"clouds"`

	mu sync.Mutex
}

// Get returns the duration of a cloud/region pairing, if it exists.
// Otherwise ok is returned as false and time.Duration is it's zero-value.
func (c *cacheStore) Get(cloud CloudName, r Name) (time.Duration, bool) {
	if c.Clouds == nil {
		c.load()
	}
	c.ensureKey(cloud, r)

	v, ok := c.Clouds[cloud][r]
	if !ok {
		return time.Duration(0), ok
	}
	return v.Duration, ok
}

// ensureKey ensures that a key can be properly accessed in the underlying cache
func (c *cacheStore) ensureKey(cloud CloudName, _ Name) {
	if _, ok := c.Clouds[cloud]; !ok {
		c.mu.Lock()
		c.Clouds[cloud] = make(map[Name]cacheEntry)
		c.mu.Unlock()
	}
}

// Set sets the duration of a cloud/region pairing
func (c *cacheStore) Set(cloud CloudName, r Name, dur time.Duration) error {
	if c.Clouds == nil {
		c.load()
	}

	c.ensureKey(cloud, r)
	c.Clouds[cloud][r] = cacheEntry{
		Duration:      dur,
		LastUpdatedAt: time.Now().UTC(),
	}
	return c.save()
}

func (c *cacheStore) getCacheFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	storageDir := RegionCachePath
	if !filepath.IsAbs(storageDir) {
		storageDir = filepath.Join(homeDir, storageDir)
	}

	return filepath.Join(storageDir, RegionCacheFile), nil
}

// load retrieves the cache from disk, if it exists, otherwise
// it is returned uninitialized
func (c *cacheStore) load() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// on first init this will be empty, fix that
	if c.Clouds == nil {
		c.Clouds = make(map[CloudName]map[Name]cacheEntry)
	}

	cacheFilePath, err := c.getCacheFilePath()
	if err != nil {
		return
	}

	f, err := os.Open(cacheFilePath)
	if err != nil {
		return
	}

	_ = json.NewDecoder(f).Decode(&c) //nolint:errcheck // Why: function signature/acceptable
}

// save saves the cache to disk
func (c *cacheStore) save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheFilePath, err := c.getCacheFilePath()
	if err != nil {
		return errors.Wrap(err, "failed to get cache file path")
	}

	f, err := os.Create(cacheFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to create cache file")
	}

	return json.NewEncoder(f).Encode(c)
}
