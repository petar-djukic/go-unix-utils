// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1-R1.4: cmd/cat binary with basic file
// concatenation, stdin reading, and error handling with exit codes.

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

	// Handle --help and --version as the first argument.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			if err := printHelp(os.Stdout); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			if err := printVersion(os.Stdout); err != nil {
				os.Exit(1)
			}
			return
		}
	}

	// R1.2: read from stdin when no file arguments are given.
	files := os.Args[1:]
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, name := range files {
		if err := catFile(name); err != nil {
			// R1.4: print error to stderr and continue with remaining files.
			fmt.Fprintf(os.Stderr, "cat: %s: %v\n", name, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// catFile reads the contents of a single file and writes them to stdout.
// R1.2: if name is "-", it reads from stdin.
func catFile(name string) error {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		// R1.1: open the named file for reading.
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}

	// R1.1, R1.3, R1.4: copy contents verbatim to stdout.
	_, err := io.Copy(os.Stdout, r)
	return err
}

// printHelp writes a usage message to w matching GNU cat --help format.
func printHelp(w io.Writer) error {
	_, err := fmt.Fprintln(w, `Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

      --help     display this help and exit
      --version  output version information and exit`)
	return err
}

// printVersion writes version information to w.
func printVersion(w io.Writer) error {
	_, err := fmt.Fprintf(w, "cat %s\n", version)
	return err
}
