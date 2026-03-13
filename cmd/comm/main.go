// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/comm implements the comm (compare two sorted files line by line) command.
// Implements: prd029-comm R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

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

// commOpts holds the parsed flags for the comm command.
type commOpts struct {
	suppress1 bool // R2.1: -1 suppresses column 1
	suppress2 bool // R2.2: -2 suppresses column 2
	suppress3 bool // R2.3: -3 suppresses column 3
}

// parseArgs extracts flags and file operands from args.
// R2.1, R2.2, R2.3: Recognises -1, -2, -3 and combined forms like -12, -123.
func parseArgs(args []string) (commOpts, []string, error) {
	var opts commOpts
	var operands []string

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' && a != "-" {
			// Check if all characters after '-' are valid flag digits.
			flagPart := a[1:]
			valid := true
			for _, ch := range flagPart {
				if ch != '1' && ch != '2' && ch != '3' {
					valid = false
					break
				}
			}
			if valid {
				if strings.ContainsRune(flagPart, '1') {
					opts.suppress1 = true
				}
				if strings.ContainsRune(flagPart, '2') {
					opts.suppress2 = true
				}
				if strings.ContainsRune(flagPart, '3') {
					opts.suppress3 = true
				}
				continue
			}
			return opts, nil, fmt.Errorf("unrecognized option '%s'", a)
		}
		operands = append(operands, a)
	}

	return opts, operands, nil
}

// columnPrefixes computes the tab prefix for each column based on which
// columns are suppressed.
//
// R2.4: When a column is suppressed, the tab indentation for remaining columns
// adjusts so that the leftmost output column has no leading tab.
func columnPrefixes(opts commOpts) (col1, col2, col3 string) {
	// Count how many visible columns precede each column.
	visibleBefore2 := 0
	if !opts.suppress1 {
		visibleBefore2++
	}
	visibleBefore3 := visibleBefore2
	if !opts.suppress2 {
		visibleBefore3++
	}

	col1 = ""
	col2 = strings.Repeat("\t", visibleBefore2)
	col3 = strings.Repeat("\t", visibleBefore3)
	return col1, col2, col3
}

// run parses arguments and executes the comm logic.
func run(args []string) error {
	opts, operands, err := parseArgs(args)
	if err != nil {
		return err
	}

	// R1.2: Accept exactly two file operands.
	if len(operands) != 2 {
		if len(operands) < 2 {
			return fmt.Errorf("missing operand")
		}
		return fmt.Errorf("extra operand '%s'", operands[2])
	}

	file1, file2 := operands[0], operands[1]

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
	if err := compareFiles(r1, r2, w, opts); err != nil {
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
// R2.1-R2.4: Column suppression with adjusted indentation.
func compareFiles(r1, r2 io.Reader, w *bufio.Writer, opts commOpts) error {
	lr1 := newLineReader(r1)
	lr2 := newLineReader(r2)

	prefix1, prefix2, prefix3 := columnPrefixes(opts)

	for !lr1.done && !lr2.done {
		// R1.2: Compare lines lexicographically.
		if lr1.line < lr2.line {
			// Column 1: unique to file1.
			if !opts.suppress1 {
				if err := writeLine(w, prefix1, lr1.line); err != nil {
					return err
				}
			}
			lr1.advance()
		} else if lr1.line > lr2.line {
			// Column 2: unique to file2.
			if !opts.suppress2 {
				if err := writeLine(w, prefix2, lr2.line); err != nil {
					return err
				}
			}
			lr2.advance()
		} else {
			// Column 3: common to both.
			if !opts.suppress3 {
				if err := writeLine(w, prefix3, lr1.line); err != nil {
					return err
				}
			}
			lr1.advance()
			lr2.advance()
		}
	}

	// R1.3: Drain remaining lines from file1.
	for !lr1.done {
		if !opts.suppress1 {
			if err := writeLine(w, prefix1, lr1.line); err != nil {
				return err
			}
		}
		lr1.advance()
	}

	// R1.3: Drain remaining lines from file2.
	for !lr2.done {
		if !opts.suppress2 {
			if err := writeLine(w, prefix2, lr2.line); err != nil {
				return err
			}
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
