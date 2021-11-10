// Copyright 2021 Outreach Corporation. All Rights Reserved.

// Description: This code contains cloud interface specific code
package region

import "context"

// CloudName is a cloud that's able to be discovered
type CloudName string

// Cloud is an interface that returns regions exposed by a cloud
type Cloud interface {
	Regions(ctx context.Context) Regions
}
