// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd059-version R1.1-R1.4: cmd/version binary.
// Prints the repository version tag to stdout and exits 0.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// R1.2: defaults to "dev" for development builds.
var version = "dev"

// Version returns the version string for use by other cmd/ packages.
// R1.5: exported function so other binaries can import and report the version.
func Version() string {
	return version
}

func main() {
	sys.InstallSIGPIPEHandler()

	switch {
	case len(os.Args) == 1:
		// R1.1: no arguments — print version and exit 0.
		fmt.Println(version)
	case len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-v"):
		// R1.4: --version or -v prints the same version string.
		fmt.Println(version)
	default:
		// R1.4: any other flag — usage message to stderr, exit 2.
		fmt.Fprintf(os.Stderr, "Usage: %s [--version|-v]\n", os.Args[0])
		os.Exit(2)
	}
}
