// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd021-tac R1.1–R1.4: core reversal behavior.
// Reads files (or stdin), splits into records on newline, and writes
// records in reverse order to stdout.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tac"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes files, returning the exit code.
// R1.3: reads stdin when no args or "-" is given.
// R1.4: each file is reversed independently.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	files := parseArgs(args)
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, name := range files {
		if err := tacOne(name, stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
// R1.1–R1.4 only: no flags are implemented in this batch.
func parseArgs(args []string) []string {
	var files []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		// Unrecognized flags are treated as filenames for forward compatibility
		files = append(files, arg)
	}
	return files
}

// tacOne reads a single file (or stdin for "-") and writes its lines reversed.
// R1.1: split on newline, write records in reverse.
// R1.2: trailing newline is terminator, not empty-record separator.
// R1.3: "-" means stdin.
func tacOne(name string, stdin io.Reader, stdout io.Writer) error {
	data, err := readInput(name, stdin)
	if err != nil {
		return err
	}
	return writeReversed(data, stdout)
}

// readInput reads the entire contents of a file or stdin.
func readInput(name string, stdin io.Reader) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // best-effort close on read-only file
	return io.ReadAll(f)
}

// writeReversed splits data into newline-delimited records and writes them
// in reverse order. Each record keeps its trailing separator attached.
// R1.1: records are split on "\n".
// R1.2: a trailing "\n" terminates the last record rather than creating
// an empty trailing record.
func writeReversed(data []byte, stdout io.Writer) error {
	if len(data) == 0 {
		return nil
	}
	records := splitRecords(string(data))
	for i := len(records) - 1; i >= 0; i-- {
		if _, err := io.WriteString(stdout, records[i]); err != nil {
			return err
		}
	}
	return nil
}

// splitRecords splits input into records where each record includes its
// trailing newline separator. The last record may lack a separator if the
// input does not end with one. R1.2: a trailing newline terminates the
// last record rather than producing an empty trailing record.
func splitRecords(s string) []string {
	var records []string
	for len(s) > 0 {
		idx := strings.Index(s, "\n")
		if idx < 0 {
			records = append(records, s)
			break
		}
		records = append(records, s[:idx+1])
		s = s[idx+1:]
	}
	return records
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
