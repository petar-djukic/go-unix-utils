// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd029-comm R1.1–R1.4: three-column comparison of two sorted
// files, byte-for-byte under LC_ALL=C, with proper exhaustion handling.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "comm"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and executes the comm comparison, returning exit code.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		return 1
	}
	r1, c1, err := openFile(args[0], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close() // best-effort close on read-only file
	}
	r2, c2, err := openFile(args[1], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close() // best-effort close on read-only file
	}
	return compareFiles(r1, r2, stdout, stderr)
}

// openFile opens a file for reading; "-" means stdin.
func openFile(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, unwrapPathError(err)
	}
	return f, f, nil
}

// compareFiles reads two sorted files and writes three-column output.
// R1.1: col1 = file1-only (no indent), col2 = file2-only (1 tab),
// col3 = common (2 tabs).
// R1.2: lexicographic comparison determines column assignment.
// R1.3: remaining lines after exhaustion go to the appropriate column.
// R1.4: byte-for-byte comparison under LC_ALL=C.
func compareFiles(r1, r2 io.Reader, stdout io.Writer, stderr io.Writer) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	w := bufio.NewWriter(stdout)

	have1 := s1.Scan()
	have2 := s2.Scan()

	for have1 && have2 {
		line1 := s1.Text()
		line2 := s2.Text()
		if err := emitPair(w, line1, line2); err != nil {
			return writeError(stderr, err)
		}
		cmp := compareLine(line1, line2)
		have1, have2 = advance(s1, s2, cmp)
	}
	if err := drainRemaining(w, s1, have1, ""); err != nil {
		return writeError(stderr, err)
	}
	if err := drainRemaining(w, s2, have2, "\t"); err != nil {
		return writeError(stderr, err)
	}
	if err := checkScanErr(s1, s2, stderr); err != nil {
		return 1
	}
	if err := w.Flush(); err != nil {
		return writeError(stderr, err)
	}
	return 0
}

// emitPair writes the appropriate column output for two current lines.
func emitPair(w *bufio.Writer, line1, line2 string) error {
	cmp := compareLine(line1, line2)
	switch {
	case cmp < 0:
		return writeLine(w, "", line1)
	case cmp > 0:
		return writeLine(w, "\t", line2)
	default:
		return writeLine(w, "\t\t", line1)
	}
}

// advance moves the appropriate scanner(s) forward based on comparison.
func advance(s1, s2 *bufio.Scanner, cmp int) (bool, bool) {
	switch {
	case cmp < 0:
		return s1.Scan(), true
	case cmp > 0:
		return true, s2.Scan()
	default:
		return s1.Scan(), s2.Scan()
	}
}

// compareLine returns <0 if a<b, 0 if a==b, >0 if a>b (byte-for-byte).
func compareLine(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// drainRemaining writes all remaining lines from a scanner to the given column.
func drainRemaining(w *bufio.Writer, s *bufio.Scanner, haveLine bool, prefix string) error {
	if haveLine {
		if err := writeLine(w, prefix, s.Text()); err != nil {
			return err
		}
	}
	for s.Scan() {
		if err := writeLine(w, prefix, s.Text()); err != nil {
			return err
		}
	}
	return nil
}

// writeLine writes a prefix and line followed by a newline.
func writeLine(w *bufio.Writer, prefix, line string) error {
	if _, err := w.WriteString(prefix); err != nil {
		return err
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// checkScanErr checks both scanners for read errors.
func checkScanErr(s1, s2 *bufio.Scanner, stderr io.Writer) error {
	if err := s1.Err(); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return err
	}
	if err := s2.Err(); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return err
	}
	return nil
}

// writeError reports a write error and returns exit code 1.
func writeError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
	return 1
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
