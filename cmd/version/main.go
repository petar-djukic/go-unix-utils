// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/version prints the repository's last known version tag.
// Implements srd059-version R1.1, R1.2, R1.4, R1.5.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Version is the build version, set via -ldflags "-X main.Version=<tag>".
// R1.5: exported so other cmd/ packages can reference the ldflags pattern.
var Version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) == 1 {
		fmt.Println(Version)
		return
	}

	switch os.Args[1] {
	case "--version", "-v":
		fmt.Println(Version)
	default:
		fmt.Fprintln(os.Stderr, "Usage: version [--version | -v]")
		os.Exit(2)
	}
}
