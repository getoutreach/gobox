// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description: This file contains the docker configuration settings
// that can be set in box.

package box

type Docker struct {
	// ImagePullRegistry is the pull registry
	ImagePullRegistry string `yaml:"imagePullRegistry"`
	// ImagePushRegistries is a list of defined push registries
	ImagePushRegistries []string `yaml:"imagePushRegistries"`
}
