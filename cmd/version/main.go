// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd059-version R1.1–R1.5 (cmd/version binary with build-time
// version injection, --version/-v flags, and usage on unknown flags).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is the build-time version string injected via
// -ldflags "-X main.version=<tag>". Defaults to "dev" when not set. R1.2.
var version = "dev"

// Version returns the version string for use by other cmd/ packages. R1.5.
func Version() string {
	return version
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) == 0 {
		printVersion()
		return
	}

	// R1.4: --version and -v print the version string.
	for _, arg := range args {
		switch arg {
		case "--version", "-v":
			printVersion()
			return
		default:
			fmt.Fprintf(os.Stderr, "usage: %s [--version | -v]\n", os.Args[0])
			os.Exit(2)
		}
	}
}

// printVersion writes the version string to stdout and exits 0. R1.1.
func printVersion() {
	_, err := fmt.Println(version)
	if err != nil {
		os.Exit(1)
	}
}
