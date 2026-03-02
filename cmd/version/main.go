// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd011-magefiles (R1)
//
// cmd/version prints the repository's version tag to stdout and exits 0.
// The version string is injected at build time via -ldflags "-X main.version=<tag>".
// When the linker variable is not set, the binary prints "dev".
package main

import (
	"fmt"
	"os"
)

// version is the build-time version tag, set via -ldflags "-X main.version=<tag>".
// Defaults to "dev" for development builds without ldflags. (prd011-magefiles R1.2)
var version = "dev"

// Version returns the build-time version string so other cmd/ packages can
// import and report the same value without duplicating the ldflags mechanism.
// (prd011-magefiles R1.5)
func Version() string {
	return version
}

// usage is the help message printed to stderr for unrecognized flags.
const usage = "Usage: version [--version | -v]"

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints version information. Returns the exit code.
// (prd011-magefiles R1.1, R1.4)
func run(args []string) int {
	if len(args) == 0 {
		fmt.Println(version)
		return 0
	}

	for _, arg := range args {
		switch arg {
		case "--version", "-v":
			fmt.Println(version)
			return 0
		default:
			fmt.Fprintln(os.Stderr, usage)
			return 2
		}
	}

	return 0
}
