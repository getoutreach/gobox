// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains box configuration for loft
package box

import "github.com/getoutreach/gobox/pkg/region"

// LoftRuntimeConfig is configuration for loft runtimes
type LoftRuntimeConfig struct {
	// Clusters is a list of clusters provided by this loft instance
	Clusters LoftClusters `yaml:"clusters"`

	// DefaultCloud is the default cloud to use. Currently the only way to specify
	// which cloud.
	DefaultCloud region.CloudName `yaml:"defaultCloud"`

	// DefaultRegion is the default region to use when a nearest one couldn't
	// be calculated
	DefaultRegion region.Name `yaml:"regionName"`

	// URL is the URL of a loft instance.
	URL string `yaml:"URL"`
}

// LoftCluster is a loft cluster
type LoftCluster struct {
	// Name is the name of the cluster in loft
	Name string `yaml:"name"`

	// Region is the region that this cluster is in
	Region region.Name `yaml:"region"`

	// Cloud is the cloud that this loft cluster is in. Not currently used anywhere.
	Cloud region.CloudName `yaml:"cloud"`
}

// LoftClusters is a container for a slice of LoftClusters
type LoftClusters []LoftCluster

// Regions returns all of the regions of the regions for the loft clusters in a []LoftCluster
func (lc LoftClusters) Regions() []region.Name {
	regions := make([]region.Name, 0)
	for _, e := range lc {
		regions = append(regions, e.Region)
	}
	return regions
}
