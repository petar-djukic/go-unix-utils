// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/version prints the repository's version tag and exits. The version string
// is set at build time via -ldflags "-X main.version=<tag>"; development builds
// without ldflags print "dev".
//
// Implements prd011-magefiles R1.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "version"

// version is set at build time via -ldflags "-X main.version=<tag>".
// When the linker variable is not set, defaults to "dev" (prd011-magefiles R1.2).
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) == 0 {
		printVersion()
	}

	parseArgs(args)
}

// parseArgs handles --version, -v, and rejects unrecognized flags
// (prd011-magefiles R1.4).
func parseArgs(args []string) {
	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			printVersion()
		}

		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
			fmt.Fprintf(os.Stderr, "Usage: %s [--version | -v]\n", progName)
			os.Exit(2)
		}

		// Non-flag argument: not accepted by cmd/version.
		fmt.Fprintf(os.Stderr, "%s: unexpected argument '%s'\n", progName, arg)
		fmt.Fprintf(os.Stderr, "Usage: %s [--version | -v]\n", progName)
		os.Exit(2)
	}
}

// printVersion writes the version string to stdout and exits 0
// (prd011-magefiles R1.1).
func printVersion() {
	out := bufio.NewWriter(os.Stdout)
	fmt.Fprintln(out, version)

	if err := out.Flush(); err != nil {
		os.Exit(1)
	}

	os.Exit(0)
}
