package regionpicker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// RegionCachePath is the $HOME/<RegionCachePath> location of the regionpicker cache
	RegionCachePath = filepath.Join(".outreach", ".cache", "box")
	// RegionCacheFile is the name of the regionpicker cache file
	RegionCacheFile = "regionpicker.json"
)

// cacheEntry is information on a cloud's cache entry
type cacheEntry struct {
	// LastBest was the last best region that was determined.
	LastBest RegionName `json:"last_best"`

	// LastUpdatedAt is the last time this file was updated. UTC.
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

type cacheFile struct {
	// Clouds is a map of cloudName to cache entries
	Clouds map[CloudName]*cacheEntry
}

// loadCache reads the cache from disk
func loadCache() (*cacheFile, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user homedir")
	}

	f, err := os.Open(filepath.Join(homeDir, RegionCacheFile, RegionCacheFile))
	if err != nil {
		return nil, err
	}

	var cache cacheFile
	if err := json.NewDecoder(f).Decode(&cache); err != nil {
		return nil, errors.Wrap(err, "failed to decode cache")
	}

	return &cache, nil
}

// saveCache saves the provided cacheFile to disk
func saveCache(c *cacheFile) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get user homedir")
	}

	cacheDir := filepath.Join(homeDir, RegionCacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create cache dir")
	}

	f, err := os.Create(filepath.Join(cacheDir, RegionCacheFile))
	if err != nil {
		return errors.Wrap(err, "failed to create cache file")
	}

	return json.NewEncoder(f).Encode(c)
}

// FindCachedBest returns the best region but uses a cache first.
func FindCachedBest(cloud CloudName, allowedRegions []RegionName, logger logrus.FieldLogger) (RegionName, error) {
	reason := ""

	existingC, err := loadCache()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			reason = errors.Wrap(err, "Failed to parse region cache").Error()
		}
		existingC = &cacheFile{
			Clouds: map[CloudName]*cacheEntry{
				cloud: {},
			},
		}
	}

	existingCE := existingC.Clouds[cloud]

	// only refresh every 8 hours, or if it's not set already
	if !existingCE.LastUpdatedAt.IsZero() && time.Now().UTC().Sub(existingCE.LastUpdatedAt) < time.Hour*8 {
		return existingCE.LastBest, nil
	}
	if reason == "" {
		reason = "Periodic refresh hit"
	}

	if logger != nil {
		logger.WithField("reason", reason).Info("Refreshing lowest latency (best) available region")
	}

	best, err := FindBest(cloud, allowedRegions, logger)
	if err != nil {
		return best, err
	}

	existingCE.LastBest = best
	existingCE.LastUpdatedAt = time.Now().UTC()

	if err := saveCache(existingC); err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to persist best region to disk, will refetch on next command invocation")
		}
	}

	return best, nil
}
