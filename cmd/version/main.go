// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the version command per prd011-magefiles R1.
// It prints the project version string to stdout and exits 0.
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// When not set, defaults to "dev". (prd011-magefiles R1.2)
var version = "dev"

// Version returns the current version string so other cmd/ packages can
// reference it if needed. (prd011-magefiles R1.5)
func Version() string {
	return version
}

func main() {
	args := os.Args[1:]

	// R1.1: no arguments — print version and exit 0.
	if len(args) == 0 {
		fmt.Println(version)
		return
	}

	// R1.4: --version and -v print the same version string.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Println(version)
		return
	}

	// R1.4: any other flag — usage to stderr, exit 2.
	fmt.Fprintln(os.Stderr, "usage: version [--version | -v]")
	os.Exit(2)
}
