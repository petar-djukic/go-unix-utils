// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/version prints the repository's last known version tag.
// Implements srd059-version R1.1, R1.2, R1.4.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R1.2: set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) == 1 {
		fmt.Println(version)
		return
	}

	switch os.Args[1] {
	case "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Fprintln(os.Stderr, "Usage: version [--version | -v]")
		os.Exit(2)
	}
}
