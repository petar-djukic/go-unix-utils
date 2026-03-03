// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sponge reads all of stdin into memory before writing to the named output
// file, making pipelines such as "sort file | sponge file" safe for in-place
// overwrites.
//
// Implements: prd007-sponge R1-R4
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const programName = "sponge"

func main() {
	code := run(os.Args[1:], os.Stdin, os.Stderr)
	os.Exit(code)
}

// run parses arguments, soaks stdin, and writes to the named file. Returns the
// exit code. Separating I/O from os.Exit allows testing without subprocess
// spawning.
func run(args []string, stdin io.Reader, stderr io.Writer) int {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	appendMode := fs.Bool("a", false, "append to the output file instead of truncating")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "Usage: %s [-a] <file>\n", programName) // best-effort stderr write
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return 1
	}

	filename := fs.Arg(0)

	// Read all of stdin before opening the output file (prd007-sponge R1).
	data, err := io.ReadAll(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: reading stdin: %v\n", programName, err) // best-effort stderr write
		return 1
	}

	// Open the output file after stdin is fully consumed (prd007-sponge R2, R3).
	var openFlags int
	if *appendMode {
		openFlags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		openFlags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	f, err := os.OpenFile(filename, openFlags, 0666)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", programName, err) // best-effort stderr write
		return 1
	}

	if _, err := f.Write(data); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: writing %s: %v\n", programName, filename, err) // best-effort stderr write
		_ = f.Close() // best-effort close; write already failed
		return 1
	}

	if err := f.Close(); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: closing %s: %v\n", programName, filename, err) // best-effort stderr write
		return 1
	}

	return 0
}
