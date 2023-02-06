// Copyright 2022 Outreach Corporation. All Rights Reserved.

//go:build or_dev || or_test || or_e2e
// +build or_dev or_test or_e2e

// Description: Provides configuration readers for various environments

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

// TestFakeTestConfigHandlerMultipleConfigs tests multiple config files that
// are created with different names
func TestFakeTestConfigHandlerMultipleConfigs(t *testing.T) {
	type args struct {
		fName  string
		config TestConfig
	}
	tests := []struct {
		name    string
		args    args
		want    func()
		wantErr bool
	}{
		{
			name: "another successful single config file",
			args: args{
				fName: "test1.yaml",
				config: TestConfig{
					ListenHost: "another-url",
					HTTPPort:   8000,
					GRPCPort:   9000,
				},
			},
			want:    func() {},
			wantErr: false,
		},
		{
			name: "successful single test config file",
			args: args{
				fName: "test.yaml",
				config: TestConfig{
					ListenHost: "someURL",
					HTTPPort:   8080,
					GRPCPort:   9090,
				},
			},
			want:    func() {},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := requirepkg.New(t)
			var deserializedExample TestConfig
			configInputMarshal, _ := yaml.Marshal(tt.args.config)
			err := yaml.Unmarshal(configInputMarshal, &deserializedExample)
			require.NoError(err, "converting hard-coded example to YAML not fail")

			deleteFunc, err := FakeTestConfigHandler(tt.args.fName, deserializedExample)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestFakeTestConfigHandlerMultipleConfigs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			defer deleteFunc()
		})
	}
}

// TestFakeTestConfigHandlerRepeatedTestOverride tests when multiple config files
// with the same name are created
func TestFakeTestConfigHandlerRepeatedTestOverride(t *testing.T) {
	type args struct {
		fName  string
		config TestConfig
	}
	tests := []struct {
		name    string
		args    args
		want    func()
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
			want:    func() {},
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
			deleteFunc, err := FakeTestConfigHandler(tt.args.fName, deserializedExample)
			assert.NilError(t, err)

			// second config call should be unsuccessful and throw an error
			_, err = FakeTestConfigHandler(tt.args.fName, deserializedExample)
			if (err != nil) != tt.wantErr {
				t.Errorf("TestFakeTestConfigHandlerParallel1() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// expect an error as the second FakeTestConfigHandler call should fail
			assert.Error(t, err, fmt.Sprintf("repeated test override of '%s'", tt.args.fName))

			defer deleteFunc()
		})
	}
}
