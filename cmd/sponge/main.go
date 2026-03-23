// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd007-sponge: Soak Stdin and Write to File.
// Covers R1.1-R1.5 (core soak-before-write contract),
// R2.1 (atomic write via temp file rename).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R5.4/shared protocol: Install SIGPIPE handler for piped output.
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])
	exitCode := run(opts, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// spongeOptions holds the parsed arguments for a sponge invocation.
type spongeOptions struct {
	outputFile string // positional argument; empty means passthrough to stdout
}

// parseArgs extracts the output filename from the argument list.
// R1.2: first positional argument is the output file.
// R1.3: no argument means write to stdout.
func parseArgs(args []string) spongeOptions {
	var opts spongeOptions
	for _, arg := range args {
		if arg == "--version" {
			fmt.Println("sponge (go-unix-utils) dev")
			os.Exit(0)
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		// First non-flag argument is the output file.
		if opts.outputFile == "" {
			opts.outputFile = arg
		}
	}
	return opts
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: sponge [FILE]
Soak up standard input and write to FILE.

If no FILE is given, write to standard output.
`)
}

// run reads all stdin, then writes to the output file or stdout.
// Returns the process exit code.
func run(opts spongeOptions, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	// R1.1: Read all of stdin before opening the output file.
	// R1.4: Works with pipes and redirects via io.ReadAll.
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "sponge: error reading stdin: %s\n", err)
		return 1
	}

	// R1.3/R4.1: No output file means passthrough to stdout.
	if opts.outputFile == "" {
		return writeToStdout(data, stdout, stderr)
	}

	// R1.5/R2.1: Write via temp file for atomicity, allowing same
	// file as both pipe source and sponge target.
	return writeToFile(data, opts.outputFile, stderr)
}

// writeToStdout writes buffered data to stdout.
func writeToStdout(data []byte, stdout io.Writer, stderr io.Writer) int {
	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintf(stderr, "sponge: write error: %s\n", err)
		return 1
	}
	return 0
}

// writeToFile writes data atomically to the named file via a temp
// file in the same directory, then renames.
// R2.1: temp file created in the same directory as the output file.
// R1.5: output file is never opened until stdin is fully consumed.
func writeToFile(data []byte, path string, stderr io.Writer) int {
	dir := filepath.Dir(path)

	// R1.4/R2.1: Create temp file in the output file's directory.
	tmp, err := os.CreateTemp(dir, "sponge.*")
	if err != nil {
		fmt.Fprintf(stderr, "sponge: cannot create temp file: %s\n", err)
		return 1
	}
	tmpName := tmp.Name()

	// R1.5/R5.4: Ensure temp file is cleaned up on any error path.
	defer cleanupTemp(tmpName)

	if err := writeTempAndClose(tmp, data); err != nil {
		fmt.Fprintf(stderr, "sponge: write error: %s\n", err)
		return 1
	}

	// R2.1: Atomic rename of temp file to output path.
	if err := os.Rename(tmpName, path); err != nil {
		fmt.Fprintf(stderr, "sponge: cannot rename to %s: %s\n", path, err)
		return 1
	}

	return 0
}

// writeTempAndClose writes data to the temp file and closes it.
func writeTempAndClose(f *os.File, data []byte) error {
	_, err := f.Write(data)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

// cleanupTemp removes the temp file if it still exists.
// Best-effort: errors are ignored since the file may have been renamed.
func cleanupTemp(path string) {
	_ = os.Remove(path) // best-effort cleanup; file may already be renamed
}
