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

	frames, err := logfile.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	for _, fm := range frames {
		if fm.Metadata != nil || fm.Frame == nil {
			continue
		}

		f := fm.Frame

		time.Sleep(f.Delay)
		os.Stdout.Write(f.Bytes)
	}
}
