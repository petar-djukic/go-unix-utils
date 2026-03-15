// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1-R1.4: cmd/cat core concatenation.
// Concatenates files to stdout, reads stdin when no arguments or "-" is given,
// and reports errors to stderr in GNU format.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU cat format.
const progName = "cat"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := 0

	// R1.2: no file arguments — read from stdin.
	if len(os.Args) < 2 {
		if err := catReader(os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "%s: stdin: %v\n", progName, err)
			exitCode = 1
		}
		os.Exit(exitCode)
	}

	// R1.1, R1.3: concatenate named files in argument order.
	for _, name := range os.Args[1:] {
		if name == "-" {
			// R1.2: "-" means read from stdin.
			if err := catReader(os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
				exitCode = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			// R1.4: print error to stderr in GNU format, continue processing.
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		if err := catReader(f); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close; read errors already reported
	}

	os.Exit(exitCode)
}

// catReader copies all data from r to stdout.
func catReader(r io.Reader) error {
	_, err := io.Copy(os.Stdout, r)
	return err
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
