// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the version utility (srd059-version R1.1-R1.5).
// It prints the repository's last known version tag and exits.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Version is the build version, set via -ldflags "-X main.Version=<tag>".
// Other cmd/ packages can import this variable to report the same version.
// R1.2: defaults to "dev" when the linker variable is not set.
var Version = "dev"

const usageMsg = "Usage: version [--version | -v]\n"

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) == 1 {
		printVersion()
		return
	}

	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "--version", "-v":
			// R1.4: print same version string as no-argument invocation.
			printVersion()
			return
		}
	}

	// R1.4: any other flag prints usage to stderr and exits 2.
	fmt.Fprint(os.Stderr, usageMsg)
	os.Exit(2)
}

// GetVersion returns the version string. R1.5: exported so that other cmd/
// packages can call this function to report the same version without
// duplicating the ldflags mechanism.
func GetVersion() string {
	return Version
}

// printVersion writes the version string followed by a newline to stdout.
func printVersion() {
	fmt.Println(Version)
}
