// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/yes implements GNU yes: repeatedly output a string until killed.
//
// Implements prd012-yes R1.1, R1.2, R1.3, R2.1.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
// R2.1: uses bufio.Writer with bufSize buffer to reduce syscall overhead.
func writeLoop(stdout *os.File, line string) int {
	w := bufio.NewWriterSize(stdout, bufSize)
	lineBytes := []byte(line + "\n")
	for {
		_, err := w.Write(lineBytes)
		if err != nil {
			return 1
		}
	}
}
