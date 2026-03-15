// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd013-true R1.1-R1.3, R2.1-R2.3, R3.1-R3.2: cmd/true binary
// that exits 0 unconditionally, supports --help and --version output.

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is the project version string, set at build time via
// -ldflags "-X main.version=<tag>". Defaults to "dev" for development builds.
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: only --help and --version as the first argument trigger output.
	// R1.2: all other arguments are ignored.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.1: print usage to stdout, exit 0.
			// R2.3: exit 1 on write error.
			if err := printHelp(os.Stdout); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			// R2.2: print version to stdout, exit 0.
			// R2.3: exit 1 on write error.
			if err := printVersion(os.Stdout); err != nil {
				os.Exit(1)
			}
			return
		}
	}

	// R1.1, R1.2, R1.3: exit 0 with no output.
}

// printHelp writes a usage message to w matching GNU true --help format.
func printHelp(w io.Writer) error {
	_, err := fmt.Fprintln(w, `Usage: true [ignored command line arguments]
  or:  true OPTION
Exit with a status code indicating success.

      --help     display this help and exit
      --version  output version information and exit`)
	return err
}

// printVersion writes version information to w.
func printVersion(w io.Writer) error {
	_, err := fmt.Fprintf(w, "true %s\n", version)
	return err
}
