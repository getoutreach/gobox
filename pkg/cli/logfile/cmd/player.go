// Copyright 2023 Outreach Corporation. All Rights Reserved.

// Description: This file contains a logfile player that can be used to replay
// logfiles.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getoutreach/gobox/pkg/cli/logfile"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: player <logfile>")
		os.Exit(1)
	}

	entries, err := logfile.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	for _, e := range entries {
		if e.IsMetadata() || !e.IsFrame() {
			continue
		}

		f := e.AsFrame()
		time.Sleep(f.Delay)
		os.Stdout.Write(f.Bytes)
	}
}
