// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the version command, which prints the repository's
// version tag to stdout and exits 0.
//
// The version string is injected at build time via:
//
//	go build -ldflags "-X main.version=<tag>" ./cmd/version/
//
// When the linker variable is not set (development builds), the binary prints
// "dev".
//
// Implements: prd011-magefiles R1.1, R1.2, R1.4, R1.5
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// When not set, defaults to "dev" for development builds.
//
// Implements: prd011-magefiles R1.2, R1.5
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints version information. Returns the exit code.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Println(version)
		return 0
	}

	switch args[0] {
	case "--version", "-v":
		fmt.Println(version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "usage: version [--version | -v]\n")
		return 2
	}
}
