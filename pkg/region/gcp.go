// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the GCP cloud code.
package region

import "context"

const (
	// CloudGCP is the Google Cloud
	CloudGCP CloudName = "gcp"

	// RegionGCPUS is a multi-region region for all of the US
	RegionGCPUS Name = "us"

	// RegionGCPUS is a multi-region region for all of Europe
	RegionGCPEurope Name = "europe"

	// RegionGCPUS is a multi-region region for all of Asia
	RegionGCPAsia Name = "asia"
)

// GCP is a Google Cloud implementation of the interface Cloud
type GCP struct{}

// Regions returns a list of all known-gcp regions. This should not be used
// over native GCP region fetching.
// IDEA: One day we could probably get this from GCP. That is authenticated though.
func (*GCP) Regions(_ context.Context) Regions {
	regions := []region{
		{Name: "us", Endpoint: "https://us-docker.pkg.dev"},
		{Name: "europe", Endpoint: "https://europe-docker.pkg.dev"},
		{Name: "asia", Endpoint: "https://asia-docker.pkg.dev"},
	}

	for i := range regions {
		regions[i].Cloud = CloudGCP
	}

	return regions
}
