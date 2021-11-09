package region

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestCloud_Regions_Nearest(t *testing.T) {
	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "regionpicker-cache-*")
	assert.NilError(t, err, "failed to create tempdir")
	RegionCachePath = tempDir

	regionName, err := (&GCP{}).Regions(ctx).Nearest(ctx, logrus.New())
	assert.NilError(t, err, "failed to get nearest region")

	if regionName == "" {
		t.Errorf("got an empty region trying to find nearest region")
		return
	}
}

func TestCloud_Regions_Filter_Nearest(t *testing.T) {
	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "regionpicker-cache-*")
	assert.NilError(t, err, "failed to create tempdir")
	RegionCachePath = tempDir

	regionName, err := (&GCP{}).Regions(ctx).Filter([]Name{RegionGCPUS}).Nearest(ctx, logrus.New())
	assert.NilError(t, err, "failed to get nearest region")
	assert.Equal(t, regionName, RegionGCPUS, "expected to get filtered region")
}
