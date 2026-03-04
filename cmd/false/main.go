// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the false utility that exits with code 1 unconditionally.
//
// Implements prd013-false (R1, R2, R3, R4).
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// R2.2: defaults to "dev" when the linker variable is not set.
var version = "dev"

func main() {
	// R3.1: handle SIGPIPE by exiting cleanly with code 0.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	go func() {
		<-sigpipe
		os.Exit(0)
	}()

	// R2: check for --help and --version flags.
	// R4.2, R4.3: --help and --version override the default exit code to 0.
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--help":
			// R2.1: print usage summary to stdout and exit 0.
			fmt.Fprintln(os.Stdout, "Usage: false [ignored command line arguments]")
			fmt.Fprintln(os.Stdout, "  or:  false OPTION")
			fmt.Fprintln(os.Stdout, "Exit with a status code indicating failure.")
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, "      --help     display this help and exit")
			fmt.Fprintln(os.Stdout, "      --version  output version information and exit")
			os.Exit(0)
		case "--version":
			// R2.2: print version string to stdout and exit 0.
			fmt.Fprintln(os.Stdout, version)
			os.Exit(0)
		}
	}

	// R1.1, R1.2, R1.3: exit 1 with no output.
	// R1.5, R4.1: non-flag arguments are silently ignored; exit is always 1.
	os.Exit(1)
}
