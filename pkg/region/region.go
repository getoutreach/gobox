// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains region specific type definitions
// and region logic.
package region

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Name is the name of region
type Name string

// Region is a logical cloud region
type region struct {
	// Name is the name of this region
	Name Name

	// Cloud is the cloud this region is apart of
	Cloud CloudName

	// Endpoint is the endpoint to test against
	Endpoint string
}

// Duration hits the attached region's endpoint and returns how long it took
// to do a HEAD request.
func (r *region) Duration(ctx context.Context) (time.Duration, error) {
	dur, ok := cache.Get(r.Cloud, r.Name)
	if ok {
		return dur, nil
	}

	startTime := time.Now()
	resp, err := http.Head(r.Endpoint) //nolint:gosec // Why: not really variable
	// we don't care about HTTP status here, we're just determining network latency
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	dur = time.Since(startTime)

	cache.Set(r.Cloud, r.Name, dur) //nolint:errcheck // Why: don't need to handle errors

	return dur, nil
}

// Regions is a list of regions
type Regions []region

func (regions Regions) Filter(allowed []Name) Regions {
	allowedHM := make(map[Name]struct{})
	for _, ar := range allowed {
		allowedHM[ar] = struct{}{}
	}

	newRegions := make([]region, 0)
	for _, r := range regions {
		if _, ok := allowedHM[r.Name]; ok {
			newRegions = append(newRegions, r)
		}
	}

	return newRegions
}

// Nearest returns the nearest region to the current caller based on latency
// of a HEAD request to all region's endpoints.
func (regions Regions) Nearest(ctx context.Context, logger logrus.FieldLogger) (Name, error) {
	var bestTime *time.Duration
	var bestRegion Name

	for _, r := range regions {
		dur, err := r.Duration(ctx)
		if err != nil {
			if logger != nil {
				logger.WithError(err).WithField("cloud", r.Cloud).
					WithField("region", r.Name).Warn("failed to check region")
			}
			continue
		}

		if bestTime == nil || dur < *bestTime {
			bestTime = &dur
			bestRegion = r.Name
		}
	}

	if bestRegion == "" {
		return "", fmt.Errorf("failed to find the nearest region")
	}

	return bestRegion, nil
}
