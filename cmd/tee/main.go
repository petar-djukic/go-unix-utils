// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tee: read stdin and write to stdout and files.
// Implements srd017-tee R1.1, R1.2, R1.3, R1.4.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "tee"

func main() {
	// D1: install SIGPIPE handler first.
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run executes the tee logic and returns the exit code.
// R1.2: with no file arguments, acts as passthrough (stdin to stdout).
// R1.1: writes stdin bytes to stdout and all named files simultaneously.
func run(args []string) int {
	files, exitCode := openFiles(args)
	defer closeFiles(files)

	writers := buildWriters(files)
	if err := copyToAll(os.Stdin, writers); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	return exitCode
}

// openFiles opens each file argument for writing (truncate mode).
// R1.3: creates files that do not exist; truncates existing files.
// R1.4: "-" is treated as stdout (not opened as a file).
// R3.3/R1.4 (partial): on open failure, prints diagnostic and continues.
func openFiles(args []string) ([]*os.File, int) {
	var files []*os.File
	exitCode := 0

	for _, name := range args {
		if name == "-" {
			// R1.4: "-" means stdout; no additional file to open.
			continue
		}
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		if err != nil {
			reportOpenError(name, err)
			exitCode = 1
			continue
		}
		files = append(files, f)
	}
	return files, exitCode
}

// reportOpenError prints a GNU-compatible diagnostic for a failed file open.
func reportOpenError(name string, err error) {
	var pe *os.PathError
	if errors.As(err, &pe) {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, name, pe.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, name, err)
}

// buildWriters returns a slice of writers: stdout plus all open files.
// R1.1: stdout is always included as the first destination.
func buildWriters(files []*os.File) []io.Writer {
	writers := make([]io.Writer, 0, 1+len(files))
	writers = append(writers, os.Stdout)
	for _, f := range files {
		writers = append(writers, f)
	}
	return writers
}

// copyToAll reads from r and writes every byte to all writers.
// D2: uses io.MultiWriter for fan-out.
// R1.5: output order matches stdin order (io.Copy preserves order).
func copyToAll(r io.Reader, writers []io.Writer) error {
	mw := io.MultiWriter(writers...)
	_, err := io.Copy(mw, r)
	return err
}

// closeFiles closes all open file handles, ignoring errors.
func closeFiles(files []*os.File) {
	for _, f := range files {
		f.Close() // best-effort close
	}
}
