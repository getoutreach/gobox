// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file contains the cd configuration settings
// that can be set in box.

package box

// CD contains the cd configuration settings that can be set in box
type CD struct {
	// Concourse contains the concourse configuration settings
	Concourse struct {
		// Address is the concourse host url
		Address string `yaml:"address"`
	} `yaml:"concourse"`
	// Maestro contains the maestro configuration settings
	Maestro struct {
		// Address is the maestro host url
		Address string `yaml:"address"`
	} `yaml:"maestro"`
}
