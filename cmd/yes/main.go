// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd012-yes R1.1-R1.4, R2.1-R2.2, R3.1-R3.3:
// cmd/yes outputs a string repeatedly until killed or a write error occurs.
// With no arguments, outputs "y". With arguments, joins them with spaces.
// Uses buffered I/O for throughput and installs SIGPIPE handler for clean exit.
package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// bufSize is the output buffer size.
// R2.1: at least 8192 bytes for performance.
const bufSize = 8192

func main() {
	// R3.1, R3.3: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	// R1.1, R1.2, R1.3: determine the line to output.
	// R1.3: skip "--" separator if it is the first argument.
	args := os.Args[1:]
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
