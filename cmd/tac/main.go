// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd021-tac R1.1-R1.4, R2.1-R2.4: cmd/tac reads input, reverses
// the order of records, and writes to stdout. Reads from stdin when no file
// arguments are given or when "-" appears as a filename. Multiple files are
// processed independently in argument order. Supports custom record separators
// via -s/--separator (R2.1) and before-mode separator placement via
// -b/--before (R2.2-R2.4). Installs SIGPIPE handler for clean exit on broken
// pipe (R3.4).
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU tac format.
const progName = "tac"

func main() {
	sys.InstallSIGPIPEHandler()

	sep, before, files := parseArgs(os.Args[1:])
	exitCode := 0

	if len(files) == 0 {
		// R1.3: no file arguments — read from stdin.
		if err := tacReader(os.Stdin, sep, before); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
		os.Exit(exitCode)
	}

	// R1.4: process each file independently in argument order.
	for _, name := range files {
		if name == "-" {
			// R1.3: "-" means read from stdin.
			if err := tacReader(os.Stdin, sep, before); err != nil {
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
		if err := tacReader(f, sep, before); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close; read errors already reported
	}

	os.Exit(exitCode)
}

// parseArgs extracts flags and file arguments from command-line args.
// Returns the separator string, the before flag, and the list of files.
//
// R2.1: -s SEP / --separator=SEP sets the record separator.
// R2.2-R2.3: -b / --before places separator before each record.
func parseArgs(args []string) (separator string, before bool, files []string) {
	separator = "\n" // default separator is newline
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}

		// R2.2: -b / --before flag.
		if arg == "-b" || arg == "--before" {
			before = true
			continue
		}

		// R2.1: -s SEP (separate argument).
		if arg == "-s" || arg == "--separator" {
			if i+1 < len(args) {
				i++
				separator = args[i]
			}
			continue
		}

		// R2.1: -sSEP (value concatenated with short flag).
		if strings.HasPrefix(arg, "-s") && len(arg) > 2 && arg[2] != '-' {
			separator = arg[2:]
			continue
		}

		// R2.1: --separator=SEP.
		if strings.HasPrefix(arg, "--separator=") {
			separator = arg[len("--separator="):]
			continue
		}

		files = append(files, arg)
	}

	return separator, before, files
}

// tacReader reads all content from r, splits into records on sep, reverses
// the record order, and writes to stdout.
//
// R1.1: split on separator and reverse.
// R1.2: trailing separator is the terminator of the last record, not a
// separator before an empty record.
// R2.1: sep may be any string (default newline).
// R2.2-R2.3: when before is true, the separator is attached to the beginning
// of each record instead of the end.
func tacReader(r io.Reader, sep string, before bool) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	sepBytes := []byte(sep)
	var records [][]byte
	if before {
		records = splitRecordsBefore(data, sepBytes)
	} else {
		records = splitRecordsAfter(data, sepBytes)
	}

	// Write records in reverse order.
	for i := len(records) - 1; i >= 0; i-- {
		if _, werr := os.Stdout.Write(records[i]); werr != nil {
			return werr
		}
	}

	return nil
}

// splitRecordsAfter splits data into records where the separator is attached
// to the end of each record (default mode).
//
// For input "a\nb\nc\n" with sep "\n", records are ["a\n", "b\n", "c\n"].
// R1.2: trailing empty record after final separator is dropped.
func splitRecordsAfter(data, sep []byte) [][]byte {
	var records [][]byte
	for len(data) > 0 {
		idx := bytes.Index(data, sep)
		if idx < 0 {
			records = append(records, data)
			break
		}
		end := idx + len(sep)
		records = append(records, data[:end])
		data = data[end:]
	}
	// R1.2: drop empty trailing record.
	if len(records) > 0 && len(records[len(records)-1]) == 0 {
		records = records[:len(records)-1]
	}
	return records
}

// splitRecordsBefore splits data into records where the separator is attached
// to the beginning of each record (-b mode).
//
// R2.2-R2.3: for input ":a:b:c" with sep ":", records are [":a", ":b", ":c"].
// Content before the first separator (if any) forms a record without a leading
// separator.
func splitRecordsBefore(data, sep []byte) [][]byte {
	var records [][]byte
	for len(data) > 0 {
		// Search for the next separator occurrence past the current leading
		// separator (if the data starts with one).
		startSearch := 0
		if bytes.HasPrefix(data, sep) {
			startSearch = len(sep)
		}

		idx := -1
		if startSearch < len(data) {
			found := bytes.Index(data[startSearch:], sep)
			if found >= 0 {
				idx = startSearch + found
			}
		}

		if idx < 0 {
			records = append(records, data)
			break
		}

		records = append(records, data[:idx])
		data = data[idx:]
	}
	// Drop empty trailing record.
	if len(records) > 0 && len(records[len(records)-1]) == 0 {
		records = records[:len(records)-1]
	}
	return records
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
