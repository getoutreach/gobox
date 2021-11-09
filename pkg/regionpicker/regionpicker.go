package regionpicker

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// clouds is the global map of all supported clouds
var clouds = map[CloudName]Cloud{
	CloudGCP: &GCP{},
}

// getPing hits an endpoint and returns the ping time
func getPing(url string) (time.Duration, error) {
	startTime := time.Now().UTC()
	resp, err := http.Head(url) //nolint:gosec // Why: not really variable
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	endTime := time.Now().UTC()

	return endTime.Sub(startTime), nil
}

// best returns the best region out of a list of regions
func best(regions []region, logger logrus.FieldLogger) RegionName {
	var bestTime *time.Duration
	var bestRegion RegionName

	for _, r := range regions {
		ping, err := getPing(r.endpoint)
		if err != nil {
			if logger != nil {
				logger.WithError(err).WithField("region", r.name).Warn("failed to check region")
			}
			continue
		}

		if bestTime == nil || ping < *bestTime {
			bestTime = &ping
			bestRegion = r.name
		}
	}

	return bestRegion
}

// FindBest returns the best region based on ping.
// error is only returned when improperly called, or when no region could
// be found.
// allowedRegions and logger are optional.
func FindBest(cloud CloudName, allowedRegions []RegionName, logger logrus.FieldLogger) (RegionName, error) {
	c, ok := clouds[cloud]
	if !ok {
		return "", fmt.Errorf("unsupported cloud '%s'", cloud)
	}

	allowedRegionsHM := make(map[RegionName]struct{})
	for _, r := range allowedRegions {
		allowedRegionsHM[r] = struct{}{}
	}

	allRegions := c.Regions()

	// filter out all the regions we found based on what's in allowedRegions
	// if it's set, otherwise we allow all
	var regions []region
	if allowedRegions != nil {
		regions = make([]region, 0)
		for _, r := range allRegions {
			if _, ok := allowedRegionsHM[r.name]; !ok {
				continue
			}

			regions = append(regions, r)
		}
	} else {
		regions = allRegions
	}

	bestRegion := best(regions, logger)
	if bestRegion == "" {
		return "", fmt.Errorf("failed to find best region")
	}

	return bestRegion, nil
}
