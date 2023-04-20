// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: See package comment

// Package logfile implements a hook that will re-run the current process
// with a PTY attached to it, and then hook into the PTY's stdout/stderr
// to record logs. Also exposed is the lower level functions (recorder, storage)
// that are used to implement the hook.
package logfile

import (
	"path/filepath"
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
