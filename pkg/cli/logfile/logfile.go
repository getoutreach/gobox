// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment

// Package logfile contains code for intercepting CLI output
// and dropping it into a logfile.
package logfile

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/getoutreach/gobox/pkg/app"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/term"
)

// EnvironmentVariable is the environment variable that is set when
// the process is being re-ran with a PTY attached to it and its logs
// are being recorded.
const EnvironmentVariable = "OUTREACH_LOGGING_TO_FILE"

// InProgressSuffix is the suffix to denote that a log file is for an
// in-progress command. Meaning that it is not complete, or that the
// wrapper has crashed.
//
// Note: This does not include the file extension, which can be grabbed
// from LogExtension.
const InProgressSuffix = "_inprog"

// LogDirectoryBase is the directory where logs are stored
// relative to the user's home directory.
const LogDirectoryBase = ".outreach" + string(filepath.Separator) + "logs"

// LogExtension is the extension for log files
const LogExtension = "json"

// TracePortEnvironmentVariable is the environment variable for the socket port
// used to communicate traces between the child app and the logging wrapper.
const TracePortEnvironmentVariable = "OUTREACH_LOGGING_PORT"

// SocketType is the type of socket for the log file.
const TraceSocketType = "tcp"

// Hook re-runs the current process with a PTY attached to it, and then
// hooks into the PTY's stdout/stderr to record logs.
func Hook() error {
	if _, ok := os.LookupEnv(EnvironmentVariable); ok {
		// We're already logging to a file, so don't do anything.
		return nil
	}

	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get user's home directory")
	}

	// $HOME/.outreach/logs/appName
	logDir := filepath.Join(homeDir, LogDirectoryBase, app.Info().Name)

	// ensure that the log directory exists
	if _, err := os.Stat(logDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return errors.Wrap(err, "failed to create log directory")
		}
	}

	// logFile is the new file descriptor that we will write to
	// and replace the old one with
	logFile, err := os.Create(filepath.Join(logDir, fmt.Sprintf("%s_inprog.%s", uuid.New(), LogExtension)))
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}

	// create the command with an env var to prevent an infinite loop
	//nolint:gosec // Why: We're using the same command that was run to start the process
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=1", EnvironmentVariable))

	l, err := net.Listen(TraceSocketType, "localhost:0")
	if err != nil {
		return errors.Wrap(err, "failed to start trace server")
	}

	// Set the TracePortEnvironmentVariable to the port selected by the listener
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%d", TracePortEnvironmentVariable,
		l.Addr().(*net.TCPAddr).Port))

	var cmdErr error
	if isTerminal {
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return errors.Wrap(err, "failed to start pty")
		}

		// hook into the PTY's stdout/stderr and forward it to the log file
		// and stdout, as well as forward stdin to the PTY
		exited, err := ptyOutputHook(l, cmd, ptmx, logFile)
		if err != nil {
			return errors.Wrap(err, "failed to hook into pty output")
		}

		// Forward all signals to the PTY
		forwardSignals(exited, ptmx, cmd)

		// Handle the error after the logs have flushed
		cmdErr = cmd.Wait()

		// Close the PTY and wait for the output hook to flush
		//nolint:errcheck // Why: Best effort
		ptmx.Close()
		<-exited
	} else {
		rec := newRecorder(logFile, 0, 0, cmd.Path, cmd.Args)
		finishedChan := make(chan struct{})
		finishedTraceChan := newTraceServer(rec, l, finishedChan)

		cmd.Stdout = io.MultiWriter(os.Stdout, rec)
		cmd.Stderr = io.MultiWriter(os.Stderr, rec)
		cmdErr = cmd.Run()

		close(finishedChan)

		// wait for traces to flush
		<-finishedTraceChan
	}

	// Close the log file, since we're done writing to it
	logFile.Close()

	// Rename the log file to be completed
	logPath := logFile.Name()
	if err := os.Rename(logPath, strings.TrimSuffix(logPath, InProgressSuffix+"."+LogExtension)+"."+LogExtension); err != nil {
		return errors.Wrap(err, "failed to rename log file to be completed")
	}

	// Proxy the error from the command we ran
	if cmdErr != nil {
		// use the exit code from the command
		var execErr *exec.ExitError
		if errors.As(cmdErr, &execErr) {
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

// attachStdinToPty attaches the current os.Stdin to the
// provided PTY if running in a terminal
func attachStdinToPty() (func(), error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return func() {}, nil
	}

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	return func() {
		//nolint:errcheck // Why: Best effort
		term.Restore(int(os.Stdin.Fd()), oldState)
	}, nil
}

// ptyOutputHook reads the data from the PTY and writes it to the log file
// and stdout while also handling forwarding os.Stdin to the PTY.
func ptyOutputHook(l net.Listener, cmd *exec.Cmd, ptmx,
	logFile *os.File) (<-chan struct{}, error) {
	detachStdin, err := attachStdinToPty()
	if err != nil {
		return nil, errors.Wrap(err, "failed to attach stdin to pty")
	}

	// forward os.Stdin to the PTY
	//nolint:errcheck // Why: Best effort
	go io.Copy(ptmx, os.Stdin)

	finishedChan := make(chan struct{})

	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get terminal size")
	}

	rec := newRecorder(logFile, w, h, cmd.Path, cmd.Args[1:])

	// start a trace server to listen to listen to trace info
	stopTracerChan := make(chan struct{})
	finishedTraceServer := newTraceServer(rec, l, stopTracerChan)

	// forward the PTY to the log file and stdout
	go func() {
		//nolint:errcheck // Why: Best effort
		io.Copy(io.MultiWriter(os.Stdout, rec), ptmx)
		detachStdin()

		// tell the tracer server to stop
		close(stopTracerChan)

		// wait for the tracer server to finish (flush)
		<-finishedTraceServer

		// tell the caller we're done flushing all logs+traces to disk
		close(finishedChan)
	}()

	return finishedChan, nil
}

// newTraceServer creates a server that listens for traces on the default socket tand writes them to
// the provided recorder.
func newTraceServer(rec *recorder, l net.Listener, exited <-chan struct{}) <-chan struct{} {
	finishedChan := make(chan struct{})

	// terminate the listener when the command exits
	go func() {
		<-exited
		l.Close()
	}()

	// start a server to listen for traces, closing finishedChan when the server exits
	go func() {
		defer close(finishedChan)

		for {
			c, err := l.Accept()
			if err != nil {
				// if the listener was closed, we're done and can return
				if errors.Is(err, net.ErrClosed) {
					return
				}

				fmt.Printf("accept error: %v\n", err)
				return
			}
			handleConnection(rec, c)
		}
	}()

	return finishedChan
}

// handleConnection reads from a connection and writes out
// to the results to the provided writer.
func handleConnection(rec *recorder, c net.Conn) {
	defer c.Close()

	if err := rec.WriteTrace(c); err != nil {
		fmt.Printf("write trace err: %v\n", err)
		return
	}
}
