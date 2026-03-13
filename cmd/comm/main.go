// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/comm implements the comm (compare two sorted files line by line) command.
// Implements: prd029-comm R1.1, R1.2, R1.3, R1.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "comm"

func main() {
	// R1.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}
}

// run parses arguments and executes the comm logic.
func run(args []string) error {
	// R1.2: Accept exactly two file operands.
	if len(args) != 2 {
		if len(args) < 2 {
			return fmt.Errorf("missing operand")
		}
		return fmt.Errorf("extra operand '%s'", args[2])
	}

	file1, file2 := args[0], args[1]

	// R1.3: Support reading from stdin when a file operand is '-'.
	r1, err := openInput(file1)
	if err != nil {
		return err
	}
	defer func() { _ = r1.Close() }() // best-effort cleanup, error ignored

	r2, err := openInput(file2)
	if err != nil {
		return err
	}
	defer func() { _ = r2.Close() }() // best-effort cleanup, error ignored

	w := bufio.NewWriter(os.Stdout)
	if err := compareFiles(r1, r2, w); err != nil {
		return err
	}

	// R1.4: Flush buffered output; report write error.
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	return nil
}

// stdinReader wraps os.Stdin so Close is a no-op.
type stdinReader struct {
	io.Reader
}

// Close is a no-op for stdin.
func (s stdinReader) Close() error { return nil }

// openInput opens a file for reading, or returns stdin if name is "-".
//
// R1.3: When a file operand is '-', read from stdin.
func openInput(name string) (io.ReadCloser, error) {
	if name == "-" {
		return stdinReader{os.Stdin}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// lineReader wraps a bufio.Scanner to provide one-line-at-a-time reading
// with an explicit "done" flag.
type lineReader struct {
	scanner *bufio.Scanner
	line    string
	done    bool
	err     error
}

// newLineReader creates a lineReader and reads the first line.
func newLineReader(r io.Reader) *lineReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lr := &lineReader{scanner: s}
	lr.advance()
	return lr
}

// advance reads the next line from the scanner.
func (lr *lineReader) advance() {
	if lr.scanner.Scan() {
		lr.line = lr.scanner.Text()
	} else {
		lr.done = true
		lr.err = lr.scanner.Err()
	}
}

// compareFiles reads two sorted files line by line and produces three-column output.
//
// R1.1: Lines present only in file1 → column 1 (no indent).
//        Lines present only in file2 → column 2 (one tab indent).
//        Lines present in both → column 3 (two tabs indent).
// R1.2: Comparison is lexicographic (bytes, LC_ALL=C).
// R1.3: When one file is exhausted, remaining lines go to the appropriate column.
func compareFiles(r1, r2 io.Reader, w *bufio.Writer) error {
	lr1 := newLineReader(r1)
	lr2 := newLineReader(r2)

	for !lr1.done && !lr2.done {
		// R1.2: Compare lines lexicographically.
		if lr1.line < lr2.line {
			// Column 1: unique to file1.
			if err := writeLine(w, "", lr1.line); err != nil {
				return err
			}
			lr1.advance()
		} else if lr1.line > lr2.line {
			// Column 2: unique to file2.
			if err := writeLine(w, "\t", lr2.line); err != nil {
				return err
			}
			lr2.advance()
		} else {
			// Column 3: common to both.
			if err := writeLine(w, "\t\t", lr1.line); err != nil {
				return err
			}
			lr1.advance()
			lr2.advance()
		}
	}

	// R1.3: Drain remaining lines from file1.
	for !lr1.done {
		if err := writeLine(w, "", lr1.line); err != nil {
			return err
		}
		lr1.advance()
	}

	// R1.3: Drain remaining lines from file2.
	for !lr2.done {
		if err := writeLine(w, "\t", lr2.line); err != nil {
			return err
		}
		lr2.advance()
	}

	// Check for read errors.
	if lr1.err != nil {
		return fmt.Errorf("read error: %w", lr1.err)
	}
	if lr2.err != nil {
		return fmt.Errorf("read error: %w", lr2.err)
	}

	return nil
}

// writeLine writes a line with the given prefix to the writer.
func writeLine(w *bufio.Writer, prefix, line string) error {
	if _, err := w.WriteString(prefix); err != nil {
		return err
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
