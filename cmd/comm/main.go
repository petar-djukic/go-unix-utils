// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/comm implements GNU comm: compare two sorted files line by line.
//
// Implements prd029-comm R1.1 (three-column output), R1.2 (sorted-order comparison),
// R1.3 (file exhaustion handling), R1.4 (byte-for-byte LC_ALL=C comparison).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "comm"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments, opens files, and performs the three-column comparison.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintf(stderr, "%s: missing operand\n", programName)
		return 1
	}
	r1, c1, err := openFile(args[0], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close() // best-effort close
	}
	r2, c2, err := openFile(args[1], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close() // best-effort close
	}
	if err := compare(r1, r2, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return 0
}

// openFile opens a file for reading. "-" means stdin.
func openFile(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// compare reads two sorted inputs and writes three-column output.
// R1.1: col1 = unique to file1 (no indent), col2 = unique to file2 (one tab),
// col3 = common (two tabs). R1.2: lexicographic byte comparison determines column.
func compare(r1, r2 io.Reader, w io.Writer) error {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	bw := bufio.NewWriter(w)
	have1 := s1.Scan()
	have2 := s2.Scan()
	for have1 && have2 {
		l1, l2 := s1.Text(), s2.Text()
		var err error
		if l1 < l2 {
			err = writeLine(bw, "", l1)
			have1 = s1.Scan()
		} else if l2 < l1 {
			err = writeLine(bw, "\t", l2)
			have2 = s2.Scan()
		} else {
			err = writeLine(bw, "\t\t", l1)
			have1 = s1.Scan()
			have2 = s2.Scan()
		}
		if err != nil {
			return err
		}
	}
	if err := s1.Err(); err != nil {
		return err
	}
	if err := s2.Err(); err != nil {
		return err
	}
	if err := drainRemaining(bw, s1, have1, ""); err != nil {
		return err
	}
	if err := drainRemaining(bw, s2, have2, "\t"); err != nil {
		return err
	}
	return bw.Flush()
}

// drainRemaining writes all remaining lines from a scanner with the given prefix.
// R1.3: when one file is exhausted, remaining lines go to the appropriate column.
func drainRemaining(w *bufio.Writer, s *bufio.Scanner, hasLine bool, prefix string) error {
	for hasLine {
		if err := writeLine(w, prefix, s.Text()); err != nil {
			return err
		}
		hasLine = s.Scan()
	}
	return s.Err()
}

// writeLine writes a prefixed line followed by a newline.
func writeLine(w *bufio.Writer, prefix, line string) error {
	if _, err := w.WriteString(prefix); err != nil {
		return err
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
