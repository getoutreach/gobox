// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the options for the github client.
package github

import (
	"io"

	"github.com/sirupsen/logrus"
)

// Options contains the options for a Github client
type Options struct {
	// AllowUnauthenticated allows the client to be created without
	// a token.
	AllowUnauthenticated bool

	// Logger is an optional logger to use for logging
	Logger logrus.FieldLogger
}

// apply applies functional options to the options struct
func (o *Options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}

	// set a default discard logger if none
	// was provided
	if o.Logger == nil {
		nullLog := logrus.New()
		nullLog.Out = io.Discard
		o.Logger = nullLog
	}
}

// Option is a functional option for configuring a client
type Option func(*Options)

// WithAllowUnauthenticated allows the client to be created without
// a token.
func WithAllowUnauthenticated() Option {
	return func(o *Options) {
		o.AllowUnauthenticated = true
	}
}

// WithLogger provides a logger for the client
func WithLogger(log logrus.FieldLogger) Option {
	return func(o *Options) {
		o.Logger = log
	}
}
