// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd021-tac R1.1-R1.4: cmd/tac reads input, reverses the order
// of lines, and writes to stdout. Reads from stdin when no file arguments
// are given or when "-" appears as a filename. Multiple files are processed
// independently in argument order. Installs SIGPIPE handler for clean exit
// on broken pipe (R3.4).
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU tac format.
const progName = "tac"

func main() {
	sys.InstallSIGPIPEHandler()

	files := parseArgs(os.Args[1:])
	exitCode := 0

	if len(files) == 0 {
		// R1.3: no file arguments — read from stdin.
		if err := tacReader(os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
		os.Exit(exitCode)
	}

	// R1.4: process each file independently in argument order.
	for _, name := range files {
		if name == "-" {
			// R1.3: "-" means read from stdin.
			if err := tacReader(os.Stdin); err != nil {
				fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
				exitCode = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			// R3.2: print error to stderr in GNU tac format, continue processing.
			fmt.Fprintf(os.Stderr, "%s: failed to open '%s' for reading: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		if err := tacReader(f); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close; read errors already reported
	}

	os.Exit(exitCode)
}

// parseArgs extracts file arguments, skipping "--" as end-of-flags marker.
// R1.1-R1.4 task scope has no flags; future tasks will add -s, -b, -r.
func parseArgs(args []string) []string {
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		files = append(files, arg)
	}

	return files
}

// tacReader reads all content from r, reverses the record order, and writes
// to stdout.
//
// R1.1: split on newlines and reverse.
// R1.2: trailing newline is the terminator of the last record, not a
// separator before an empty record. "a\nb\n" reversed produces "b\na\n".
//
// GNU tac keeps the separator attached to (after) each record. For input
// "a\nb\nc", the records are ["a\n", "b\n", "c"]. Reversed output is
// "c" + "b\n" + "a\n" = "cb\na\n". This matches gtac byte-for-byte.
func tacReader(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	// Split input into records where the newline separator stays attached
	// to the end of each record.
	var records [][]byte
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			// Last chunk with no trailing newline.
			records = append(records, data)
			break
		}
		// Include the newline in the record.
		records = append(records, data[:idx+1])
		data = data[idx+1:]
	}

	// R1.2: if the last record is empty (from a trailing newline producing
	// an empty slice after the final split), drop it. This happens when
	// input ends with "\n" — the split above produces the last record as
	// the content up to and including that newline, so no empty trailing
	// record is created. This guard handles the edge case defensively.
	if len(records) > 0 && len(records[len(records)-1]) == 0 {
		records = records[:len(records)-1]
	}

	// Write records in reverse order.
	for i := len(records) - 1; i >= 0; i-- {
		if _, werr := os.Stdout.Write(records[i]); werr != nil {
			return werr
		}
	}

	return nil
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
