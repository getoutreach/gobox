// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This file contains development registry configuration
package box

import "github.com/getoutreach/gobox/pkg/region"

// DevelopmentRegistries contains a slice of DevelopmentRegistrys
type DevelopmentRegistries struct {
	// Path is a go-template string of the path to append to the end of the endpoint
	// for the docker image registry to use. This is useful for namespacing images.
	Path string `yaml:"path"`

	// Clouds is a CloudName -> DevelopmentRegistriesSlice
	Clouds map[region.CloudName]DevelopmentRegistriesSlice
}

// DevelopmentRegistriesSlice is a slice of DevelopmentRegistry
type DevelopmentRegistriesSlice []DevelopmentRegistry

// Regions returns all of the regions of the development registries
func (dr DevelopmentRegistriesSlice) Regions() []region.Name {
	regions := make([]region.Name, 0)
	for _, e := range dr {
		regions = append(regions, e.Region)
	}
	return regions
}

// DevelopmentRegistry is a docker image registry used for development
type DevelopmentRegistry struct {
	// Endpoint is the endpoint of this registry, e.g.
	// gcr.io/outreach-docker or docker.io/getoutreach
	Endpoint string `yaml:"endpoint"`

	// Region that this registry should be used in. If not set will be randomly selected.
	Region region.Name `yaml:"region"`
}
