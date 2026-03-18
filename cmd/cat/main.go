// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1–R1.4: core file concatenation.
// Reads named files in argument order and writes contents verbatim to stdout.
// Reads from stdin when no arguments are given or when "-" is specified.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run processes files and returns the exit code.
// R1.1: reads each named file in argument order.
// R1.2: reads stdin when no args or "-" is given.
// R1.3: concatenates with no separator.
// R1.4: binary-safe via io.Copy.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		return catFiles([]string{"-"}, stdin, stdout, stderr)
	}
	return catFiles(args, stdin, stdout, stderr)
}

// catFiles iterates over filenames, copying each to stdout.
func catFiles(files []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	exitCode := 0
	for _, name := range files {
		if err := catOne(name, stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "cat: %s: %s\n", name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// catOne copies a single file (or stdin for "-") to stdout.
func catOne(name string, stdin io.Reader, stdout io.Writer) error {
	if name == "-" {
		_, err := io.Copy(stdout, stdin)
		return err
	}
	f, err := os.Open(name)
	if err != nil {
		return unwrapPathError(err)
	}
	defer f.Close() // best-effort close on read-only file
	_, err = io.Copy(stdout, f)
	return err
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
