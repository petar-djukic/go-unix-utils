// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd012-yes: Repeatedly Output a String.
// Covers R1.1-R1.4 (core output behavior), R2.1-R2.2 (help/version output).
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// Defaults to "dev" when the linker variable is not set.
var version = "dev"

func main() {
	// R3.1: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.1-R2.2: check for --help and --version as first argument.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			os.Exit(printHelp())
		case "--version":
			os.Exit(printVersion())
		case "--":
			// R1.3: arguments after "--" are treated as output strings.
			args = args[1:]
		}
	}

	// R1.1: default output is "y" when no arguments are given.
	// R1.2: join all arguments with single spaces.
	line := "y"
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}

	writeLoop(line)
}

// writeLoop repeatedly writes line followed by a newline to stdout using
// a buffered writer. R2.1: uses bufio.Writer for buffered output.
// R2.2: flushes before exiting on write error.
func writeLoop(line string) {
	w := bufio.NewWriter(os.Stdout)
	for {
		_, err := w.WriteString(line)
		if err != nil {
			// best-effort flush before exit
			_ = w.Flush()
			os.Exit(1)
		}
		err = w.WriteByte('\n')
		if err != nil {
			_ = w.Flush()
			os.Exit(1)
		}
	}
}

// printHelp writes usage information to stdout and returns the exit code.
// R2.1: prints help to stdout.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: yes [STRING]...
  or:  yes OPTION
Repeatedly output a line with all specified STRING(s), or 'y'.

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
// R2.2: prints version to stdout.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "yes (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
