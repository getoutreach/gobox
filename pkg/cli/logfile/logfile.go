// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment

// Package logfile this file contains code for intercepting CLI output
// and dropping it into a logfile.
package logfile

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

// EnvironmentVariable is the environment variable that is set when
// the process is being re-ran with a PTY attached to it and it's logs
// are being recorded.
const EnvironmentVariable = "OUTREACH_LOGGING_TO_FILE"

// logDir is the directory where logs are stored
const logDir = ".outreach" + string(filepath.Separator) + "logs"

// Hook re-runs the current process with a PTY attached to it, and then
// hooks into the PTY's stdout/stderr to record logs.
func Hook() error {
	if _, ok := os.LookupEnv(EnvironmentVariable); ok {
		// We're already logging to a file, so don't do anything.
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get user's home directory")
	}
	homeLogDir := filepath.Join(homeDir, logDir)

	// ensure that the log directory exists
	if _, err := os.Stat(homeLogDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(homeLogDir, 0o755); err != nil {
			return errors.Wrap(err, "failed to create log directory")
		}
	}

	// logFile is the new file descriptor that we will write to
	// and replace the old one with
	logFile, err := os.Create(filepath.Join(homeLogDir, "devenv.log"))
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}

	// create the command with an env var to prevent an infinite loop
	//nolint:gosec // Why: We're using the same command that was run to start the process
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=1", EnvironmentVariable))

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return errors.Wrap(err, "failed to start pty")
	}

	// hook into the PTY's stdout/stderr and forward it to the log file
	// and stdout, as well as forward stdin to the PTY
	exited, err := ptyOutputHook(cmd, ptmx, logFile)
	if err != nil {
		return errors.Wrap(err, "failed to hook into pty output")
	}

	forwardSignals(exited, ptmx, cmd)

	// Handle the error after the logs have flushed
	err = cmd.Wait()

	// Clean up the PTY + log file
	ptmx.Close()

	// Wait for the logs to flush then close the log file
	<-exited
	logFile.Close()

	if err != nil {
		// use the exit code from the command
		var execErr *exec.ExitError
		if errors.As(err, &execErr) {
			os.Exit(execErr.ExitCode())
		}

		// fallback to 1 if we can't get the exit code
		os.Exit(1)
	}

	os.Exit(0)

	return nil
}

// forwardSignals forwards signals to the PTY as well as handles SIGWINCH
// to resize the PTY.
func forwardSignals(exited <-chan struct{}, ptmx *os.File, cmd *exec.Cmd) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-exited:
				signal.Stop(c)
				return
			case s := <-c:
				switch s {
				case syscall.SIGWINCH:
					//nolint:errcheck // Why: Best effort
					pty.InheritSize(os.Stdin, ptmx)
				default:
					//nolint:errcheck // Why: Best effort
					cmd.Process.Signal(s)
				}
			}
		}
	}()

	// Initial resize of the PTY
	c <- syscall.SIGWINCH
}

// ptyOutputHook reads the data from the PTY and writes it to the log file
// and stdout while also handling forwarding os.Stdin to the PTY.
func ptyOutputHook(cmd *exec.Cmd, ptmx, logFile *os.File) (<-chan struct{}, error) {
	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	// forward os.Stdin to the PTY
	//nolint:errcheck // Why: Best effort
	go io.Copy(ptmx, os.Stdin)

	exitChan := make(chan struct{})

	// forward the PTY to the log file and stdout
	//nolint:errcheck // Why: Best effort
	go func() {
		io.Copy(io.MultiWriter(newRecoder(logFile, cmd.Path, cmd.Args), os.Stdout), ptmx)
		term.Restore(int(os.Stdin.Fd()), oldState)
		close(exitChan)
	}()

	return exitChan, nil
}
