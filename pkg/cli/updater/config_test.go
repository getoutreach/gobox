// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: This file defines the user configuration for the updater that
// is stored on the user's machine. This is not configuration that the updater
// takes in. This is, however, loaded into the updater's configuration.

package updater

import (
	"reflect"
	"testing"
)

func Test_readConfig(t *testing.T) {
	tests := []struct {
		name    string
		want    *config
		config  *config
		wantErr bool
	}{
		{
			name: "should load a default config",
			want: &config{
				Version:                    ConfigVersion,
				GlobalConfig:               &updateConfiguration{},
				PerRepositoryConfiguration: map[string]*updateConfiguration{},
				UpdaterCache:               map[string]updateCache{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			// persist the configuration to disk for the test to use
			if tt.config != nil {
				if err := tt.config.Save(); err != nil {
					t.Fatal(err)
				}
			}

			got, err := readConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("readConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
