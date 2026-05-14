// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat implements srd006-cat R1.1-R1.4: basic file concatenation.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	for _, name := range args {
		if err := catFile(name); err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s\n", formatErr(err))
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// R1.1, R1.2: read a named file or stdin ("-") and write to stdout.
func catFile(name string) error {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	_, err := io.Copy(os.Stdout, r)
	return err
}

func formatErr(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Sprintf("%s: %s", pe.Path, pe.Err)
	}
	return err.Error()
}
