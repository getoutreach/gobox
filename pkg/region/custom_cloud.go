// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the custom cloud code
package region

import "context"

const (
	// CloudCustom is a custom cloud created by NewCustomCloud
	CloudCustom CloudName = "custom"
)

// CustomCloud is a cloud that can take arbitrary regions
type CustomCloud struct {
	regions []*CustomRegion
}

// CustomRegion is a region that can be passed to NewCustomCloud to create
// a custom cloud with specific options.
type CustomRegion struct {
	// Name is a region name
	Name Name

	// Endpoint is an endpoint to HEAD to determine the latency of this region
	Endpoint string
}

// NewCustomCloud creates a new CustomCloud with the provided regions
func NewCustomCloud(regions []*CustomRegion) *CustomCloud {
	return &CustomCloud{regions}
}

// Regions converts the underlying region list into Regions
func (cc *CustomCloud) Regions(ctx context.Context) Regions {
	regions := make([]region, len(cc.regions))
	for i, r := range cc.regions {
		regions[i] = region{
			Name:     r.Name,
			Endpoint: r.Endpoint,
			Cloud:    CloudCustom,
		}
	}
	return regions
}
