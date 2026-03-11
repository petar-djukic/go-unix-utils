// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd059-version R1.1–R1.4 (version command).
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	// R1.4: reject any arguments.
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "usage: version\n")
		os.Exit(1)
	}

	// R1.2: read version from embedded build info; fall back to "(devel)".
	version := "(devel)"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		version = info.Main.Version
	}

	// R1.3: output format is "go-unix-utils <version>\n".
	fmt.Printf("go-unix-utils %s\n", version)
}
