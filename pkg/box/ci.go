// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the ci configuration settings
// that can be set in box.

package box

// CI contains the ci configuration settings that can be set in box
type CI struct {
	// CircleCI contains the CircleCI configuration settings
	CircleCI struct {
		// Contexts are authentication contexts that can be used
		// to authenticate with CircleCI.
		Contexts struct {
			// AWS is the AWS authentication context
			// The context should contain the following values:
			// AWS_ACCESS_KEY_ID: <access key id>
			// AWS_SECRET_ACCESS_KEY: <secret access key>
			AWS string `yaml:"aws"`

			// Github is the Github authentication context
			// The context should contain the following values:
			// GHACCESSTOKEN_GHAPP_1: <github app>
			// GHACCESSTOKEN_PAT_1: <github personal access token>
			//
			// For more information on this, see:
			// https://github.com/getoutreach/ci/blob/main/cmd/ghaccesstoken/token.go
			Github string `yaml:"github"`

			// Docker is the docker authentication context
			// Currently all that is supported is gcp.
			// The context should contain the following values:
			// GCLOUD_SERVICE_ACCOUNT: <gcp service account json>
			Docker string `yaml:"docker"`

			// NPM is the npm authentication context
			// The context should contain the following values:
			// NPM_TOKEN: <npm token>
			NPM string `yaml:"npm"`

			// ExtraContexts is a list of extra contexts to include
			// for every job
			ExtraContexts []string `yaml:"extraContexts"`
		} `yaml:"contexts"`
	} `yaml:"circleci"`
}
