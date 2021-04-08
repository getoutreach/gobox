package cfg_test

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
)

func Example() {
	type HoneycombConfig struct {
		Disable    bool
		Dataset    string
		APIHost    string
		SampleRate float64
		Key        cfg.Secret
	}

	expected := HoneycombConfig{
		Disable:    true,
		Dataset:    "boo",
		APIHost:    "hoo",
		SampleRate: 5.3,
		Key:        cfg.Secret{Path: "someSecretPath"},
	}

	defer env.FakeTestConfig("honeycomb.yaml", expected)()
	defer secretstest.Fake("someSecretPath", "someSecretData")()

	var hcConfig HoneycombConfig
	if err := cfg.Load("honeycomb.yaml", &hcConfig); err != nil {
		fmt.Println("Unexpected error", err)
	}

	if hcConfig != expected {
		fmt.Println("Unexpected differences")
	}

	data, err := hcConfig.Key.Data(context.Background())
	if err != nil {
		fmt.Println("Error fetching data")
	}
	if string(data) != "someSecretData" {
		fmt.Println("Unexpected secret data", data)
	}

	fmt.Println(data)

	// Output:
	// redacted
}
