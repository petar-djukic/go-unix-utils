// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tac: concatenate and print files in reverse.
// Implements srd021-tac R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// openInput returns os.Stdin for "-", otherwise opens the named file.
// R1.3: stdin when filename is "-".
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError extracts the underlying error from os.PathError to produce
// GNU-compatible error messages: "<name>: <reason>".
func formatOpenError(name string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// reverseRecords splits data on newline and writes records in reverse order.
// R1.1: split on separator (newline), write in reverse.
// R1.2: trailing separator terminates the last record, not an empty record.
func reverseRecords(data []byte, w io.Writer) error {
	sep := byte('\n')
	hasSuffix := len(data) > 0 && data[len(data)-1] == sep
	records := splitRecords(data, sep)
	return writeReversed(records, sep, hasSuffix, w)
}

// splitRecords splits data into records on the given separator byte.
// R1.2: a trailing separator terminates the last record.
func splitRecords(data []byte, sep byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	parts := bytes.Split(data, []byte{sep})
	// R1.2: if data ends with sep, the last element is empty; remove it.
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// writeReversed writes records in reverse order. Each record except the last
// output record is followed by sep. The last output record gets sep only if
// the original input ended with sep (hasSuffix).
func writeReversed(records [][]byte, sep byte, hasSuffix bool, w io.Writer) error {
	for i := len(records) - 1; i >= 0; i-- {
		if _, err := w.Write(records[i]); err != nil {
			return err
		}
		// Append separator after each record, except omit trailing sep
		// when the original input did not end with one.
		if i > 0 || hasSuffix {
			if _, err := w.Write([]byte{sep}); err != nil {
				return err
			}
		}
	}
	return nil
}

// tacFile reads the named file and writes its records in reverse to w.
// R1.4: each file is processed independently.
func tacFile(name string, w io.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("%s: %s", name, err)
	}
	return reverseRecords(data, w)
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: no arguments means read stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	// R1.4: process each file independently in argument order.
	for _, name := range args {
		if err := tacFile(name, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "tac: %s\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
