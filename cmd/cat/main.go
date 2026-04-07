// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cat: concatenate and display files.
// Implements srd006-cat R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// catFile copies the contents of one file (or stdin) to stdout.
// R1.1: writes contents verbatim to stdout.
// R1.4: no transformation — io.Copy preserves binary data.
func catFile(name string, w io.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	_, err = io.Copy(w, r)
	return err
}

// openInput returns os.Stdin for "-" or empty name, otherwise opens the file.
// R1.2: stdin when filename is "-" or absent.
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.2: no arguments means read stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	// R1.1, R1.3: process each file in argument order, concatenated.
	for _, name := range args {
		if err := catFile(name, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
