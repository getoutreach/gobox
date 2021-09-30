package box

import "github.com/sirupsen/logrus"

type LoadBoxOptions struct {
	// MinVersion of a box configuration that is required for this
	// LoadBox call.
	MinVersion *float32

	// Deprecated: Configure before running an application instead.
	// DefaultBoxSources is a list of URLs to present to the user
	// as being the default locations of box configuration.
	DefaultBoxSources []string

	// log is the logger to use
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

// Deprecated: Do not use. See field on LoadBoxOptions
// WithDefaults sets the default URLs to provided to a user when
// a box configuration doesn't exist locally.
func WithDefaults(defaults []string) LoadBoxOption {
	return func(opts *LoadBoxOptions) {
		opts.DefaultBoxSources = defaults
	}
}

// WithLogger sets the logger to use when outputting to the user.
func WithLogger(log logrus.FieldLogger) LoadBoxOption {
	return func(opts *LoadBoxOptions) {
		opts.log = log
	}
}
