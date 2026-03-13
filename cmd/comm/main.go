// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/comm implements the comm (compare two sorted files line by line) command.
// Implements: prd029-comm R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4
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
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		// R3.1, R3.2: orderError already printed its message to stderr.
		// R4.2, R4.3: File open errors and write errors also exit 1.
		if _, ok := err.(*orderError); !ok {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		}
		os.Exit(1)
	}
	// R4.1: Exit 0 when all inputs processed successfully.
}

// orderMode controls how comm handles unsorted input.
type orderMode int

const (
	// orderDefault prints a warning to stderr, continues, then exits non-zero.
	orderDefault orderMode = iota
	// orderCheck makes unsorted input fatal (exit non-zero immediately).
	orderCheck
	// orderNoCheck disables the sorting check entirely.
	orderNoCheck
)

// commOpts holds the parsed flags for the comm command.
type commOpts struct {
	suppress1      bool      // R2.1: -1 suppresses column 1
	suppress2      bool      // R2.2: -2 suppresses column 2
	suppress3      bool      // R2.3: -3 suppresses column 3
	order          orderMode // R3.1, R3.2, R3.3: order checking mode
	outputDelimSet bool      // R3.4: whether --output-delimiter was specified
	outputDelim    string    // R3.4: custom column separator
}

// parseArgs extracts flags and file operands from args.
// R2.1, R2.2, R2.3: Recognises -1, -2, -3 and combined forms like -12, -123.
// R3.2, R3.3: Recognises --check-order and --nocheck-order.
// R3.4: Recognises --output-delimiter=STRING.
func parseArgs(args []string) (commOpts, []string, error) {
	var opts commOpts
	var operands []string

	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		// R3.2, R3.3, R3.4: Handle long options.
		if strings.HasPrefix(a, "--") {
			switch {
			case a == "--check-order":
				opts.order = orderCheck
			case a == "--nocheck-order":
				opts.order = orderNoCheck
			case a == "--output-delimiter":
				// --output-delimiter STRING (space-separated form)
				i++
				if i >= len(args) {
					return opts, nil, fmt.Errorf("option '--output-delimiter' requires an argument")
				}
				opts.outputDelimSet = true
				opts.outputDelim = args[i]
			case strings.HasPrefix(a, "--output-delimiter="):
				// --output-delimiter=STRING
				opts.outputDelimSet = true
				opts.outputDelim = a[len("--output-delimiter="):]
			default:
				return opts, nil, fmt.Errorf("unrecognized option '%s'", a)
			}
			continue
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

// columnPrefixes computes the delimiter prefix for each column based on which
// columns are suppressed and the configured output delimiter.
//
// R2.4: When a column is suppressed, the indentation for remaining columns
// adjusts so that the leftmost output column has no leading delimiter.
// R3.4: When --output-delimiter is set, uses STRING instead of tab.
// GNU comm uses a NUL byte when --output-delimiter= is empty.
func columnPrefixes(opts commOpts) (col1, col2, col3 string) {
	delim := "\t"
	if opts.outputDelimSet {
		delim = opts.outputDelim
		if delim == "" {
			// GNU comm writes NUL bytes when --output-delimiter= is empty.
			delim = "\x00"
		}
	}

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
	col2 = strings.Repeat(delim, visibleBefore2)
	col3 = strings.Repeat(delim, visibleBefore3)
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
	compareErr := compareFiles(r1, r2, w, opts)

	// Always flush buffered output, even on order errors — GNU comm writes
	// partial output before reporting order violations.
	if flushErr := w.Flush(); flushErr != nil && compareErr == nil {
		return fmt.Errorf("write error: %w", flushErr)
	}

	return compareErr
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
// with an explicit "done" flag. It tracks the previous line for order checking.
type lineReader struct {
	scanner *bufio.Scanner
	line    string
	prev    string // R3.1: previous line for order checking
	hasPrev bool   // R3.1: whether prev is valid (false for first line)
	done    bool
	err     error
}

// newLineReader creates a lineReader and reads the first line.
func newLineReader(r io.Reader) *lineReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lr := &lineReader{scanner: s}
	// Read the first line without setting prev/hasPrev.
	if lr.scanner.Scan() {
		lr.line = lr.scanner.Text()
	} else {
		lr.done = true
		lr.err = lr.scanner.Err()
	}
	return lr
}

// advance reads the next line from the scanner, saving the current line as prev.
func (lr *lineReader) advance() {
	lr.prev = lr.line
	lr.hasPrev = true
	if lr.scanner.Scan() {
		lr.line = lr.scanner.Text()
	} else {
		lr.done = true
		lr.err = lr.scanner.Err()
	}
}

// orderError is a sentinel error returned when order checking detects unsorted
// input and the mode requires a non-zero exit.
type orderError struct {
	msg string
}

func (e *orderError) Error() string { return e.msg }

// checkOrder validates that the current line is not less than the previous line.
// Returns true if an order violation was detected (regardless of whether it is fatal).
// R3.1: Default mode prints a warning and continues (returns nil error).
// R3.2: --check-order makes it fatal (returns orderError).
// R3.3: --nocheck-order skips the check entirely.
func checkOrder(lr *lineReader, fileNum int, opts commOpts) (bool, error) {
	if opts.order == orderNoCheck {
		return false, nil
	}
	if !lr.hasPrev || lr.done {
		return false, nil
	}
	if lr.line < lr.prev {
		msg := fmt.Sprintf("%s: file %d is not in sorted order", progName, fileNum)
		fmt.Fprintln(os.Stderr, msg)
		if opts.order == orderCheck {
			// R3.2: Fatal — return error to stop processing.
			return true, &orderError{msg: msg}
		}
		// R3.1: Default — warning printed, continue processing.
		return true, nil
	}
	return false, nil
}

// compareFiles reads two sorted files line by line and produces three-column output.
//
// R1.1: Lines present only in file1 → column 1 (no indent).
//
//	Lines present only in file2 → column 2 (one tab indent).
//	Lines present in both → column 3 (two tabs indent).
//
// R1.2: Comparison is lexicographic (bytes, LC_ALL=C).
// R1.3: When one file is exhausted, remaining lines go to the appropriate column.
// R2.1-R2.4: Column suppression with adjusted indentation.
// R3.1-R3.3: Order checking with default/check/nocheck modes.
func compareFiles(r1, r2 io.Reader, w *bufio.Writer, opts commOpts) error {
	lr1 := newLineReader(r1)
	lr2 := newLineReader(r2)

	prefix1, prefix2, prefix3 := columnPrefixes(opts)

	// R3.1: Track whether any order violation occurred for the summary message.
	orderViolated := false

	for !lr1.done && !lr2.done {
		// R3.1-R3.3: Check order of both files.
		if violated, err := checkOrder(lr1, 1, opts); err != nil {
			return err
		} else if violated {
			orderViolated = true
		}
		if violated, err := checkOrder(lr2, 2, opts); err != nil {
			return err
		} else if violated {
			orderViolated = true
		}

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
		if violated, err := checkOrder(lr1, 1, opts); err != nil {
			return err
		} else if violated {
			orderViolated = true
		}
		if !opts.suppress1 {
			if err := writeLine(w, prefix1, lr1.line); err != nil {
				return err
			}
		}
		lr1.advance()
	}

	// R1.3: Drain remaining lines from file2.
	for !lr2.done {
		if violated, err := checkOrder(lr2, 2, opts); err != nil {
			return err
		} else if violated {
			orderViolated = true
		}
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

	// R3.1: In default mode, print summary and exit non-zero if any violation occurred.
	if orderViolated && opts.order == orderDefault {
		msg := fmt.Sprintf("%s: input is not in sorted order", progName)
		fmt.Fprintln(os.Stderr, msg)
		return &orderError{msg: msg}
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
