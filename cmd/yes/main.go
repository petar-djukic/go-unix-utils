// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/yes implements GNU yes: repeatedly output a string until killed.
//
// Implements prd012-yes R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Version is set at build time via -ldflags "-X main.Version=<tag>".
// Defaults to "dev" for development builds.
var Version = "dev"

// helpText is the usage message printed when --help is the first argument.
const helpText = `Usage: yes [STRING]...
  or:  yes OPTION
Repeatedly output a line with all specified STRING(s), or 'y'.

      --help     display this help and exit
      --version  output version information and exit
`

// bufSize is the output buffer size for buffered writes (R2.1).
const bufSize = 8192

func main() {
	// R3.1: install SIGPIPE handler for clean exit on broken pipe.
	sys.InstallSIGPIPEHandler()

	os.Exit(run(os.Args[1:], os.Stdout))
}

// run implements the yes logic. Returns exit code.
func run(args []string, stdout *os.File) int {
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort
			return 0
		case "--version":
			// R2.2: version output matches GNU yes version string pattern.
			fmt.Fprintf(stdout, "yes (go-unix-utils) %s\n", Version) //nolint:errcheck // best-effort
			return 0
		}
	}

	line := buildLine(args)
	return writeLoop(stdout, line)
}

// buildLine constructs the output line from arguments.
// R1.1: no args → "y". R1.2: args joined by spaces.
func buildLine(args []string) string {
	if len(args) == 0 {
		return "y"
	}
	return strings.Join(args, " ")
}

// writeLoop writes the line repeatedly to stdout using buffered I/O.
// R1.4: terminates on write error without printing an error message.
// R2.1: uses bufio.Writer with bufSize buffer to reduce syscall overhead.
// R2.2: flushes partial buffer before exiting on write error.
func writeLoop(stdout *os.File, line string) int {
	w := bufio.NewWriterSize(stdout, bufSize)
	lineBytes := []byte(line + "\n")
	for {
		_, err := w.Write(lineBytes)
		if err != nil {
			_ = w.Flush() // R2.2: best-effort flush of partial buffer
			return exitCodeForWriteErr(err)
		}
	}
}

// exitCodeForWriteErr returns 0 for EPIPE (R3.1) and 1 for other errors (R3.2).
// R3.2: yes exits silently without writing to stderr on any write failure.
func exitCodeForWriteErr(err error) int {
	if errors.Is(err, syscall.EPIPE) {
		return 0
	}
	return 1
}
