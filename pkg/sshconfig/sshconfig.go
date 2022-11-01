// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Package sshconfig implements a small ssh config parser
// based on the output of `ssh -G`.
package sshconfig

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// Get returns the value of a key in the ssh config for a given host.
// An error is only returned when a key is not found. Note: a field will
// usually have default values returned, e.g. IdentityFile.
func Get(ctx context.Context, host, field string) (string, error) {
	sshArgs := []string{}
	if path := os.Getenv("SSH_CONFIG_PATH"); path != "" {
		sshArgs = append(sshArgs, "-F", path)
	}
	sshArgs = append(sshArgs, "-G", host)

	b, err := exec.CommandContext(ctx, "ssh", sshArgs...).CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "failed to exec ssh: %s", string(b))
	}

	for _, l := range strings.Split(string(b), "\n") {
		spl := strings.Split(l, " ")
		if len(spl) < 2 {
			continue
		}

		k, v := spl[0], spl[1:]
		if strings.EqualFold(k, field) {
			return strings.Join(v, " "), nil
		}
	}

	return "", fmt.Errorf("failed to find field %s", field)
}
