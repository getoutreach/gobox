package region

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestCustomCloud_Regions_Nearest(t *testing.T) {
	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "regionpicker-cache-*")
	assert.NilError(t, err, "failed to create tempdir")
	RegionCachePath = tempDir

	regionName, err := NewCustomCloud([]*CustomRegion{
		{
			Name:     "my-region",
			Endpoint: "https://google.com",
		},
	}).Regions(ctx).Nearest(ctx, logrus.New())
	assert.NilError(t, err, "failed to get nearest region")
	assert.Equal(t, regionName, Name("my-region"), "failed to find custom cloud region")
}
