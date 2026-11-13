// Copyright 2026 Outreach Corporation. All Rights Reserved.

// Description: Examples that inject a fake Exec into code under test.

package execstub_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/getoutreach/gobox/pkg/exec"
	"github.com/getoutreach/gobox/pkg/exec/execstub"
)

// latestTag returns the latest tag of the Git repository in dir. It takes an
// exec.Exec, so that a test can fake git.
func latestTag(ctx context.Context, ex exec.Exec, dir string) (string, error) {
	cmd := ex.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git describe: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// Production code passes exec.NewRealExec() to latestTag. A test passes a
// fake, and can check the command that latestTag ran.
func ExampleNewFuncExec() {
	ex := execstub.NewFuncExec(func(cmd *exec.Cmd, name string, args []string) ([]byte, error) {
		fmt.Printf("ran %s %s in %s\n", name, strings.Join(args, " "), cmd.Dir)
		return []byte("v1.2.3\n"), nil
	})

	tag, err := latestTag(context.Background(), ex, "/src/repo")
	fmt.Println(tag, err)

	// Output:
	// ran git describe --tags --abbrev=0 in /src/repo
	// v1.2.3 <nil>
}

// A failing stub gives the same *exec.ExitError as a real command.
func ExampleNewStubbedCmdWithCode() {
	ex := execstub.NewStubbedCmdWithCode("", "fatal: No names found\n", 128)

	_, err := latestTag(context.Background(), ex, "/src/repo")

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		fmt.Println(exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
	}

	// Output:
	// 128 fatal: No names found
}
