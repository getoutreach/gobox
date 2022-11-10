// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This code contains cloud interface specific code
package region

import "context"

// CloudName is a cloud that's able to be discovered
type CloudName string

// Cloud is an interface that returns regions exposed by a cloud
type Cloud interface {
	Regions(ctx context.Context) Regions
}

// supportedClouds is a CloudName -> Cloud mapping
// of all known/supported clouds
var supportedClouds = map[CloudName]Cloud{
	CloudGCP:    &GCP{},
	CloudCustom: &CustomCloud{},
}

// CloudFromCloudName returns a cloud from a provided cloud name
func CloudFromCloudName(cloud CloudName) Cloud {
	return supportedClouds[cloud]
}
