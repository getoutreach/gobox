// Copyright 2022 Outreach Corporation. All Rights Reserved.

//go:build or_dev || or_test || or_e2e
// +build or_dev or_test or_e2e

// Description: Unit tests for configuration readers

package env

import (
	"context"
	"fmt"
	"testing"

	requirepkg "github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"gotest.tools/v3/assert"
)

type TestConfig struct {
	ListenHost string `yaml:"ListenHost"`
	HTTPPort   int    `yaml:"HTTPPort"`
	GRPCPort   int    `yaml:"GRPCPort"`
}

// LoadTestConfig returns a new Config type that has been loaded in accordance to the environment
func LoadTestConfig(ctx context.Context, input TestConfig) (*TestConfig, error) {
	c := TestConfig{
		ListenHost: input.ListenHost,
		HTTPPort:   input.HTTPPort,
		GRPCPort:   input.GRPCPort,
	}

	return &c, nil
}

// TestFakeTestConfigWithErrorMultipleConfigs tests multiple config files that
// are created with different names
func TestFakeTestConfigWithErrorMultipleConfigs(t *testing.T) {
	type args struct {
		fName  string
		config TestConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful single config file",
			args: args{
				fName: "test1.yaml",
				config: TestConfig{
					ListenHost: "another-url",
					HTTPPort:   8000,
					GRPCPort:   9000,
				},
			},
			wantErr: false,
		},
		{
			name: "second successful single test config file",
			args: args{
				fName: "test.yaml",
				config: TestConfig{
					ListenHost: "someURL",
					HTTPPort:   8080,
					GRPCPort:   9090,
				},
			},
			wantErr: false,
		},
		{
			name: "unsuccessful test config file",
			args: args{
				fName: "test.yaml",
				config: TestConfig{
					ListenHost: "someURL",
					HTTPPort:   8080,
					GRPCPort:   9090,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := requirepkg.New(t)
			var deserializedExample TestConfig
			configInputMarshal, _ := yaml.Marshal(tt.args.config)
			err := yaml.Unmarshal(configInputMarshal, &deserializedExample)
			require.NoError(err, "converting hard-coded example to YAML not fail")

			_, err = FakeTestConfigWithError(tt.args.fName, deserializedExample)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestFakeTestConfigWithErrorMultipleConfigs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

// TestFakeTestConfigWithErrorRepeatedTestOverride tests when multiple config files
// with the same name are created
func TestFakeTestConfigWithErrorRepeatedTestOverride(t *testing.T) {
	type args struct {
		fName  string
		config TestConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "single config",
			args: args{
				fName: "asyncEntry.yaml",
				config: TestConfig{
					ListenHost: "another-url",
					HTTPPort:   8000,
					GRPCPort:   9000,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := requirepkg.New(t)
			var deserializedExample TestConfig
			configInputMarshal, _ := yaml.Marshal(tt.args.config)
			err := yaml.Unmarshal(configInputMarshal, &deserializedExample)
			require.NoError(err, "converting hard-coded example to YAML not fail")

			// first config call should be successful
			deleteFunc, err := FakeTestConfigWithError(tt.args.fName, deserializedExample)
			assert.NilError(t, err)

			// second config call should be unsuccessful and throw an error
			_, err = FakeTestConfigWithError(tt.args.fName, deserializedExample)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestFakeTestConfigWithErrorRepeatedTestOverride error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// expect an error as the second FakeTestConfigWithError call should fail
			assert.Error(t, err, fmt.Sprintf("repeated test override of '%s'", tt.args.fName))

			defer deleteFunc()
		})
	}
}
