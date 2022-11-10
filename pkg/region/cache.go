// Copyright 2022 Outreach Corporation. All Rights Reserved.

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

	// ExpiresAt is when this cache entry expires. UTC.
	ExpiresAt time.Time `json:"expires_at"`
}

type cacheStore struct {
	// Clouds is a map of cloudName to cache entries
	// cloud -> region -> cacheEntry
	Clouds map[CloudName]map[Name]cacheEntry `json:"clouds"`

	mu sync.RWMutex

	once sync.Once
}

// expireKeyIfRequired expires a key if it's ready to be expired
func (c *cacheStore) expireKeyIfRequired(cloud CloudName, r Name, v *cacheEntry) bool {
	if time.Now().UTC().After(v.ExpiresAt) {
		c.mu.Lock()
		delete(c.Clouds[cloud], r)
		c.mu.Unlock()

		// save the expiration to disk
		c.save() //nolint:errcheck // Why: best effort
		return true
	}

	return false
}

// get returns a cache entry for a given cloud/region pairing
func (c *cacheStore) get(cloud CloudName, r Name) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	v, ok := c.Clouds[cloud][r]
	if !ok {
		return nil, ok
	}

	return &v, ok
}

// Get returns the duration of a cloud/region pairing, if it exists.
// Otherwise ok is returned as false and time.Duration is it's zero-value.
func (c *cacheStore) Get(cloud CloudName, r Name) (time.Duration, bool) {
	c.once.Do(c.load) // read the cache from disk at most once

	v, ok := c.get(cloud, r)
	if !ok {
		// not ok, so just return it's not there
		return time.Duration(0), false
	}

	// expire the key if required to do so
	if c.expireKeyIfRequired(cloud, r, v) {
		return time.Duration(0), false
	}

	return v.Duration, ok
}

func (c *cacheStore) set(cloud CloudName, r Name, dur time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.Clouds[cloud]; !ok {
		c.Clouds[cloud] = make(map[Name]cacheEntry)
	}

	c.Clouds[cloud][r] = cacheEntry{
		Duration: dur,
		// Expire in 8 hours
		ExpiresAt: time.Now().UTC().Add(time.Hour * 8),
	}
}

// Set sets the duration of a cloud/region pairing
func (c *cacheStore) Set(cloud CloudName, r Name, dur time.Duration) error {
	c.once.Do(c.load) // read the cache from disk at most once

	c.set(cloud, r, dur) // set the key into our datastore

	// save the result to disk
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

	// ensure that the directory exists
	//nolint:errcheck // Why: best effort
	os.MkdirAll(storageDir, 0o755)

	return filepath.Join(storageDir, RegionCacheFile), nil
}

// load retrieves the cache from disk, if it exists, otherwise
// it is returned uninitialized
func (c *cacheStore) load() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// ensure that we always have a cache datastructure configured
	c.Clouds = make(map[CloudName]map[Name]cacheEntry)

	cacheFilePath, err := c.getCacheFilePath()
	if err != nil {
		return
	}

	f, err := os.Open(cacheFilePath)
	if err != nil {
		return
	}
	defer f.Close()

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
	defer f.Close()

	return json.NewEncoder(f).Encode(c)
}
