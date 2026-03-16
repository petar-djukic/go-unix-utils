// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd012-yes R1.1-R1.4, R2.1-R2.2, R3.1-R3.3:
// cmd/yes outputs a string repeatedly until killed or a write error occurs.
// With no arguments, outputs "y". With arguments, joins them with spaces.
// Uses buffered I/O for throughput and installs SIGPIPE handler for clean exit.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in help, version, and diagnostic output.
const progName = "yes"

// bufSize is the output buffer size.
// R2.1: at least 8192 bytes for performance.
const bufSize = 8192

func main() {
	// R3.1, R3.3: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	// R3.1: handle --help and --version as the first argument.
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s [STRING]...\n  or:  %s OPTION\nRepeatedly output a line with all specified STRING(s), or 'y'.\n\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName, progName,
			)
			os.Exit(0)
		case "--version":
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}
	}

	// R1.1, R1.2, R1.3: determine the line to output.
	// R1.3: skip "--" separator if it is the first argument.
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	line := "y\n"
	if len(args) > 0 {
		line = strings.Join(args, " ") + "\n"
	}

	// R2.1: buffer output to avoid per-line syscalls.
	w := bufio.NewWriterSize(os.Stdout, bufSize)
	lineBytes := []byte(line)

	for {
		// R1.1, R1.2, R1.4: write to stdout repeatedly.
		if _, err := w.Write(lineBytes); err != nil {
			// R2.2: flush partial buffer before exit.
			w.Flush() // best-effort flush, error ignored
			// R3.2: exit 1 on write error.
			os.Exit(1)
		}
	}
}
