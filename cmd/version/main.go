// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd059-version R1.1-R1.5: cmd/version binary that prints the
// project version string to stdout, supports --version and --help flags,
// and exports Version for import by other cmd/ packages.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Version is the project version string, set at build time via
// -ldflags "-X main.Version=<tag>". Defaults to "dev" for development builds.
// R1.2: linker-settable default. R1.5: exported for import by other cmd/ packages.
var Version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	// Suppress default output; we control where messages go.
	fs.SetOutput(io.Discard)

	versionFlag := fs.Bool("version", false, "print version string")

	err := fs.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		// R1.3 (task): --help prints usage to stdout and exits 0.
		printUsage(os.Stdout)
		return
	}
	if err != nil {
		// R1.4: unknown flags print error to stderr and exit 1.
		fmt.Fprintf(os.Stderr, "version: %v\n", err)
		os.Exit(1)
	}

	// R1.1: no arguments prints version. R1.4 (PRD R1.4): --version prints version.
	_ = versionFlag // both paths print version
	fmt.Println(Version)
}

// printUsage writes a short usage message to w.
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: version [--version] [--help]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Print the project version string.")
}
