// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cat: concatenate and display files.
// Implements srd006-cat R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3, R2.4,
// R4.1, R4.2, R4.3, R4.4, R4.5, R4.6, R4.7, R4.8, R5.1, R5.2, R5.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// options holds parsed command-line flags.
type options struct {
	numberAll       bool // -n: number all output lines
	numberNonBlank  bool // -b: number non-blank lines only
	showNonPrinting bool // -v: R4.1, R4.2
	showEnds        bool // -E: R4.3
	showTabs        bool // -T: R4.4
}

// needsLineProcessing returns true when flags require line-by-line processing.
func (o *options) needsLineProcessing() bool {
	return o.numberAll || o.numberNonBlank ||
		o.showNonPrinting || o.showEnds || o.showTabs
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) < 2 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if err := parseFlags(&opts, arg[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s\n", err)
			os.Exit(1)
		}
	}
	return opts, files
}

// parseFlags processes the characters in a combined flag string.
func parseFlags(opts *options, flags string) error {
	for _, ch := range flags {
		switch ch {
		case 'n':
			opts.numberAll = true
		case 'b':
			opts.numberNonBlank = true
		case 'v':
			opts.showNonPrinting = true
		case 'E':
			opts.showEnds = true
		case 'T':
			opts.showTabs = true
		case 'A':
			// R4.5: -A is equivalent to -v -E -T combined.
			opts.showNonPrinting = true
			opts.showEnds = true
			opts.showTabs = true
		case 'e':
			// R4.6: -e is equivalent to -v -E combined.
			opts.showNonPrinting = true
			opts.showEnds = true
		case 't':
			// R4.7: -t is equivalent to -v -T combined.
			opts.showNonPrinting = true
			opts.showTabs = true
		case 'u':
			// R4.8: accepted but ignored.
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// catPassthrough copies file contents verbatim to stdout.
// R1.1: writes contents verbatim. R1.4, R1.5: preserves all bytes and newlines.
func catPassthrough(name string, w io.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	_, err = io.Copy(w, r)
	return err
}

// catLines processes a file with line-by-line transformations.
func catLines(name string, w io.Writer, opts *options, lineNum *int) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return processLines(r, w, opts, lineNum)
}

// processLines reads from r line by line and applies transformations to w.
// R4.9 order: -v/-T, then -E, then -n/-b.
func processLines(r io.Reader, w io.Writer, opts *options, lineNum *int) error {
	reader := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if werr := writeTransformedLine(bw, line, opts, lineNum); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return bw.Flush()
		}
		if err != nil {
			// Flush what we have before returning.
			_ = bw.Flush() // best-effort flush
			return err
		}
	}
}

// writeTransformedLine writes a single line with display transformations and numbering.
func writeTransformedLine(w *bufio.Writer, line []byte, opts *options, lineNum *int) error {
	// R2.4: blank = line containing only a newline character.
	isBlank := len(line) == 1 && line[0] == '\n'

	// R4.9: prepend line number last in application order.
	if err := writeNumberPrefix(w, opts, lineNum, isBlank); err != nil {
		return err
	}

	if !opts.showNonPrinting && !opts.showEnds && !opts.showTabs {
		_, err := w.Write(line)
		return err
	}
	return writeDisplayBytes(w, line, opts)
}

// writeNumberPrefix writes the line number prefix if numbering is active.
// R2.1: format "%6d\t" for numbered lines.
// R2.2: blank lines get no prefix when numberNonBlank is set.
func writeNumberPrefix(w *bufio.Writer, opts *options, lineNum *int, isBlank bool) error {
	if !opts.numberAll && !opts.numberNonBlank {
		return nil
	}
	// R2.3: -b takes precedence over -n.
	if opts.numberNonBlank && isBlank {
		return nil
	}
	if _, err := fmt.Fprintf(w, "%6d\t", *lineNum); err != nil {
		return err
	}
	*lineNum++
	return nil
}

// writeDisplayBytes writes line bytes with -v, -T, -E transformations applied.
func writeDisplayBytes(w *bufio.Writer, line []byte, opts *options) error {
	for _, b := range line {
		if b == '\n' {
			// R4.3: append $ before newline.
			if opts.showEnds {
				if err := w.WriteByte('$'); err != nil {
					return err
				}
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			continue
		}
		if err := writeDisplayByte(w, b, opts); err != nil {
			return err
		}
	}
	return nil
}

// writeDisplayByte writes a single non-newline byte with -v and -T transformations.
func writeDisplayByte(w *bufio.Writer, b byte, opts *options) error {
	// R4.4: tab -> ^I when showTabs is active.
	if b == '\t' {
		if opts.showTabs {
			_, err := w.WriteString("^I")
			return err
		}
		return w.WriteByte(b)
	}
	// R4.2: -v does not alter tab or newline (newline handled by caller).
	if !opts.showNonPrinting {
		return w.WriteByte(b)
	}
	return writeNonPrintingByte(w, b)
}

// writeNonPrintingByte writes a byte using caret notation and M- prefix per R4.1.
func writeNonPrintingByte(w *bufio.Writer, b byte) error {
	if b < 0x20 {
		// Control chars 0x00-0x1F (tab handled by caller).
		if err := w.WriteByte('^'); err != nil {
			return err
		}
		return w.WriteByte(b + 64)
	}
	if b == 0x7F {
		_, err := w.WriteString("^?")
		return err
	}
	if b >= 0x80 {
		if _, err := w.WriteString("M-"); err != nil {
			return err
		}
		// Recurse on the low 7 bits to reuse ^X / ^? / printable logic.
		return writeNonPrintingByte(w, b&0x7F)
	}
	// Printable ASCII 0x20-0x7E.
	return w.WriteByte(b)
}

// openInput returns os.Stdin for "-", otherwise opens the named file.
// R1.2: stdin when filename is "-".
// R5.2: on failure, returns error formatted as "<name>: <reason>".
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

func main() {
	sys.InstallSIGPIPEHandler()

	opts, args := parseArgs(os.Args[1:])

	// R1.2: no arguments means read stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	lineNum := 1
	for _, name := range args {
		var err error
		if opts.needsLineProcessing() {
			err = catLines(name, os.Stdout, &opts, &lineNum)
		} else {
			err = catPassthrough(name, os.Stdout)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
