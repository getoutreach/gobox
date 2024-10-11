// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Support for env loading env vars as strings or as SecretData

package cfg

import (
	"os"

	"github.com/pkg/errors"
)

// EnvSecret looks up a secret from the environment
func EnvSecret(name string) (SecretData, error) {
	val, err := EnvString(name)
	if err != nil {
		return "", err
	}
	return SecretData(val), nil
}

// EnvString looks up a string from the environment.
func EnvString(name string) (string, error) {
	var (
		ok  bool
		val string
	)
	val, ok = os.LookupEnv(name)
	if !ok {
		return "", errors.Errorf("%q environment variable not set", name)
	}
	return val, nil
}
