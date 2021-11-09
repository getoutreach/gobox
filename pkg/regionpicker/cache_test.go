package regionpicker

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestFindCachedBest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "regionpicker-cache-*")
	if err != nil {
		t.Errorf("failed to create temp dir: MkdirTemp(): %v", err)
	}
	RegionCachePath = tempDir

	regionName, err := FindCachedBest(CloudGCP, nil, logrus.New())
	if err != nil {
		t.Errorf("got an error trying to find best cached region: FindCachedBest(): %v", err)
		return
	}

	if regionName == "" {
		t.Errorf("got an empty region trying to find best cached region")
		return
	}

	cacheFile, err := loadCache()
	if err != nil {
		t.Errorf("failed to read cache file: loadCache(): %v", err)
		return
	}

	if cacheFile.Clouds[CloudGCP].LastBest != regionName {
		t.Errorf("expected cached region to equal found region: %v = %v", cacheFile.Clouds[CloudGCP].LastBest, regionName)
		return
	}

	oldLastUpdated := cacheFile.Clouds[CloudGCP].LastUpdatedAt

	regionName, err = FindCachedBest(CloudGCP, nil, logrus.New())
	if err != nil {
		t.Errorf("got an error trying to find best cached region: FindCachedBest(): %v", err)
		return
	}

	cacheFile, err = loadCache()
	if err != nil {
		t.Errorf("failed to read cache file: loadCache(): %v", err)
		return
	}

	if cacheFile.Clouds[CloudGCP].LastUpdatedAt != oldLastUpdated {
		t.Errorf("cache file was update on second consecutive run")
		return
	}
}
