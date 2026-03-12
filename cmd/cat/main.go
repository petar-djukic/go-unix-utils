// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-cat R1.1–R1.4 (core I/O: read files and stdin, write to stdout).
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.2: when no file arguments are given, read from stdin.
	if len(args) == 0 {
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// R1.1, R1.3, R1.4: process each argument left to right; "-" means stdin.
	exitCode := 0
	for _, arg := range args {
		if err := catFile(arg); err != nil {
			fmt.Fprintf(os.Stderr, "cat: %v\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// catFile writes the contents of the named file to stdout.
// R1.3: if name is "-", reads from stdin at this position in the sequence.
func catFile(name string) error {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close() // best-effort cleanup, error ignored
		r = f
	}
	if _, err := io.Copy(os.Stdout, r); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}
