package cfg_test

import (
	"context"
	"fmt"

	"github.com/getoutreach/gobox/pkg/cfg"
	"github.com/getoutreach/gobox/pkg/env"
	"github.com/getoutreach/gobox/pkg/secrets/secretstest"
)

func Example() {
	type OtelConfig struct {
		Disable    bool
		Dataset    string
		Endpoint   string
		SampleRate float64
		Key        cfg.Secret
	}

	expected := OtelConfig{
		Disable:    true,
		Dataset:    "boo",
		Endpoint:   "hoo",
		SampleRate: 5.3,
		Key:        cfg.Secret{Path: "someSecretPath"},
	}

	deleteFunc, _ := env.FakeTestConfigHandler("trace.yaml", expected)
	defer deleteFunc()
	defer secretstest.Fake("someSecretPath", "someSecretData")()

	var hcConfig OtelConfig
	if err := cfg.Load("trace.yaml", &hcConfig); err != nil {
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
