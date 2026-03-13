// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd017-tee R1.1–R1.5
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R1.5: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.1, R1.2: Build a list of writers: always include stdout.
	writers := []io.Writer{os.Stdout}
	var files []*os.File

	exitCode := 0

	for _, arg := range args {
		// R1.4: "-" is an additional reference to stdout; data is not duplicated
		// because io.MultiWriter handles it.
		if arg == "-" {
			continue
		}
		// R1.3: Create files that do not exist; truncate existing files.
		f, err := os.OpenFile(arg, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tee: %v\n", err)
			exitCode = 1
			continue
		}
		files = append(files, f)
		writers = append(writers, f)
	}

	// R1.1: Read all of stdin and write to stdout and each file simultaneously.
	mw := io.MultiWriter(writers...)
	if _, err := io.Copy(mw, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "tee: %v\n", err)
		exitCode = 1
	}

	// Close all opened files.
	for _, f := range files {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "tee: %v\n", err)
			exitCode = 1
		}
	}

	// R1.4: Exit 0 on success.
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
