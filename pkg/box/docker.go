// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description: This file contains the docker configuration settings
// that can be set in box.

package box

type Docker struct {
	// ImagePullRegistry is the container image registry used to pull images from during deployment.
	ImagePullRegistry string `yaml:"imagePullRegistry"`
	// ImagePushRegistries is a list of container image registry URLs used to publish to when containers are generated for consumption.
	ImagePushRegistries []string `yaml:"imagePushRegistries"`
}
