// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/version prints the project version string and exits 0.
//
// Implements: prd011-magefiles R1
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component, DD7 generation scope)
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// When the linker variable is not set, "dev" is used for development builds.
// (prd011-magefiles R1.2)
var version = "dev"

// Version is the exported view of version, allowing other cmd/ packages to
// import github.com/petar-djukic/go-unix-utils/cmd/version and read the
// same string without duplicating the ldflags mechanism. (prd011-magefiles R1.5)
//
// Note: because this lives in package main, other cmd/ packages must use
// the mage build ldflags pattern (-X <pkg>.version=<tag>) for their own
// version variable. This exported symbol serves as documentation of that
// contract and provides a single source for the "dev" default.
var Version = version

const usageMsg = "usage: version [--version|-v]\n"

// run processes the given arguments and returns stdout output, stderr output,
// and exit code. Separating I/O from os.Exit allows unit tests to call run
// directly without spawning a subprocess. (prd011-magefiles R1.1, R1.4)
func run(args []string) (stdout, stderr string, code int) {
	if len(args) == 0 {
		return version + "\n", "", 0
	}
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v") {
		return version + "\n", "", 0
	}
	return "", usageMsg, 2
}

func main() {
	stdout, stderr, code := run(os.Args[1:])
	if stdout != "" {
		fmt.Print(stdout)
	}
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}
	os.Exit(code)
}
