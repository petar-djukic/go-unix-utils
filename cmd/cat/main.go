// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat implements GNU cat: concatenate files to stdout.
//
// Implements prd006-cat R1.1, R1.2, R1.3, R1.4.
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

// run concatenates files to stdout.
// R1.1: reads each named file in argument order.
// R1.2: reads stdin when no files given or when "-" is a filename.
// R1.3: concatenates multiple files with no separator.
// R1.4: passes binary data through without corruption.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	for _, name := range args {
		if err := catFile(name, stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "cat: %s: %v\n", name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// catFile copies one file (or stdin if name is "-") to stdout.
func catFile(name string, stdin io.Reader, stdout io.Writer) error {
	var r io.Reader
	if name == "-" {
		r = stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}

	_, err := io.Copy(stdout, r)
	return err
}
