// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the version command for reporting build version.
//
// Implements prd011-magefiles (R1.1, R1.2, R1.4).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// R1.2: defaults to "dev" when the linker variable is not set.
var version = "dev"

func main() {
	// R3: handle SIGPIPE by exiting cleanly with code 0.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	go func() {
		<-sigpipe
		os.Exit(0)
	}()

	// R1.4: --version and -v print the version string.
	// Any unknown flag causes flag.Parse to print usage to stderr and exit 2.
	flag.Bool("version", false, "print version and exit")
	flag.Bool("v", false, "print version and exit (short)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: version [--version | -v]")
	}
	flag.Parse()

	// R1.1: print version string to stdout followed by a newline and exit 0.
	if _, err := fmt.Fprintln(os.Stdout, version); err != nil {
		os.Exit(1)
	}
}
