// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd059-version R1.1–R1.4: version command binary that prints
// the repository's last known version tag.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// R1.2: defaults to "dev" for development builds.
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// run processes arguments and prints the version string.
// R1.1: no arguments prints version and exits 0.
// R1.4: --version/-v prints version; any other flag prints usage to stderr and exits 2.
func run(args []string) error {
	if len(args) == 0 {
		fmt.Println(version)
		return nil
	}

	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Println(version)
		return nil
	}

	return fmt.Errorf("usage: version [--version | -v]")
}
