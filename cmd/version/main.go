// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/version per prd059 (R1.1, R1.2, R1.4, R1.5).
// Prints the build version string to stdout and exits 0.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Version is set at build time via -ldflags "-X main.Version=<tag>".
// R1.2: defaults to "dev" for development builds.
// R1.5: exported so other cmd/ packages can reference the ldflags pattern.
var Version = "dev"

// run parses arguments and prints the version string.
// Returns the exit code.
func run(args []string) int {
	switch {
	case len(args) == 0:
		// R1.1: no arguments — print version and exit 0.
		fmt.Println(Version)
		return 0
	case args[0] == "--version" || args[0] == "-v":
		// R1.4: --version or -v prints the same version string.
		fmt.Println(Version)
		return 0
	default:
		// R1.4: any other flag — usage to stderr, exit 2.
		fmt.Fprintf(os.Stderr, "usage: version [--version | -v]\n")
		return 2
	}
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}
