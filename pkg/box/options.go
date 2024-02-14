// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Provides options for loading box configs

package box

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/agent"
)

type LoadBoxOptions struct {
	// MinVersion of a box configuration that is required for this
	// LoadBox call.
	MinVersion *float32

	// DefaultBoxSources is a list of URLs to present to the user
	// as being the default locations of box configuration.
	// Deprecated: Configure before running an application instead.
	DefaultBoxSources []string

	// Agent is the SSH agent used when fetching the box git repository.
	Agent agent.Agent

	// log is the logger to use.
	log logrus.FieldLogger
}

type LoadBoxOption func(*LoadBoxOptions)

// WithMinVersion sets a minimum version of a box configuration being
// required. If this version is not currently downloaded it will be
// force a box re-download. This is useful for using new fields.
// Version in box.go should be bumped when this is required.
func WithMinVersion(version float32) LoadBoxOption {
	return func(opts *LoadBoxOptions) {
		opts.MinVersion = &version
	}
}

// WithDefaults sets the default URLs to provided to a user when
// a box configuration doesn't exist locally.
// Deprecated: Do not use. See field on LoadBoxOptions
func WithDefaults(defaults []string) LoadBoxOption {
	return func(opts *LoadBoxOptions) {
		opts.DefaultBoxSources = defaults
	}
}

// WithAgent sets the SSH agent for fetching the box repository.
// If not specified, it creates a new one and uses the existing SSH
// config to load the github.com SSH key specified.
func WithAgent(sshAgent agent.Agent) LoadBoxOption {
	return func(opts *LoadBoxOptions) {
		opts.Agent = sshAgent
	}
}

// WithLogger sets the logger to use when outputting to the user.
func WithLogger(log logrus.FieldLogger) LoadBoxOption {
	return func(opts *LoadBoxOptions) {
		opts.log = log
	}
}
