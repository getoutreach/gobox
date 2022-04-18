// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file defines the ServiceActivity type
// to be implemented by services.

// Package serviceactivity provides service activities for services
// to implement and a framework to run them.
package serviceactivity

import (
	"context"
)

// ServiceActivity is the interface that all service activities must implement.
type ServiceActivity interface {
	Run(ctx context.Context) error
	Close(ctx context.Context) error
}
