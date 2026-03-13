// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd021-tac R1.1–R1.4, R3.1–R3.4
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.4: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	exitCode := 0

	if len(args) == 0 {
		// R1.3: no file arguments — read from stdin.
		if err := tacReader(os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "tac: write error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// R1.4: process each file independently in argument order.
		for _, arg := range args {
			if err := tacFile(arg); err != nil {
				fmt.Fprintf(os.Stderr, "tac: %v\n", err)
				exitCode = 1
			}
		}
	}

	// R3.1, R3.2: exit 0 on success, 1 if any file failed.
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// tacFile opens name and reverses its lines to stdout.
// R1.3: "-" reads from stdin.
func tacFile(name string) error {
	if name == "-" {
		return tacReader(os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close() // best-effort cleanup, error ignored
	return tacReader(f)
}

// tacReader reads all content from r, splits on newline, and writes records
// in reverse order to stdout. The separator is kept attached to the end of
// each record, matching GNU tac behavior.
//
// R1.1: split on newline separator, write records in reverse order.
// R1.2: a trailing newline terminates the last record, not a separator
// before an empty record. "a\nb\n" reversed produces "b\na\n".
// For input without a trailing separator (e.g. "a\nb\nc"), GNU tac treats
// the records as ["a\n", "b\n", "c"] and reverses to "cb\na\n".
func tacReader(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	// R1.1: split into records where each record includes its trailing
	// separator. The last chunk may lack a separator if the input does not
	// end with one.
	records := splitKeepSep(data, '\n')

	// Write records in reverse order.
	for i := len(records) - 1; i >= 0; i-- {
		if _, err := os.Stdout.Write(records[i]); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	return nil
}

// splitKeepSep splits data on the separator byte, keeping the separator
// attached to the end of each record. The final record may not end with
// the separator if the input does not.
func splitKeepSep(data []byte, sep byte) [][]byte {
	var records [][]byte
	for {
		idx := bytes.IndexByte(data, sep)
		if idx < 0 {
			if len(data) > 0 {
				records = append(records, data)
			}
			break
		}
		records = append(records, data[:idx+1])
		data = data[idx+1:]
	}
	return records
}
