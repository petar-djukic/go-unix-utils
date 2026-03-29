// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/comm implements GNU comm: compare two sorted files line by line.
//
// Implements prd029-comm R1.1 (three-column output), R1.2 (sorted-order comparison),
// R1.3 (file exhaustion handling), R1.4 (byte-for-byte LC_ALL=C comparison),
// R2.1 (-1 suppresses column 1), R2.2 (-2 suppresses column 2),
// R2.3 (-3 suppresses column 3), R2.4 (indentation adjusts for suppressed columns).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "comm"

// commConfig holds the parsed flags for a comm invocation.
type commConfig struct {
	suppress1 bool
	suppress2 bool
	suppress3 bool
	file1     string
	file2     string
}

// columnPrefixes holds the computed indentation prefix for each column.
// R2.4: when columns are suppressed, remaining columns shift left.
type columnPrefixes struct {
	col1 string
	col2 string
	col3 string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments, opens files, and performs the three-column comparison.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	r1, c1, err := openFile(cfg.file1, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close() // best-effort close
	}
	r2, c2, err := openFile(cfg.file2, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close() // best-effort close
	}
	prefixes := computePrefixes(cfg, "\t")
	if err := compare(r1, r2, stdout, cfg, prefixes); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return 0
}

// parseArgs extracts -1, -2, -3 flags and the two file operands.
func parseArgs(args []string) (commConfig, error) {
	var cfg commConfig
	var files []string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}
		if !parseFlagArg(arg, &cfg) {
			return cfg, fmt.Errorf("invalid option -- '%s'", arg)
		}
	}
	if len(files) != 2 {
		return cfg, fmt.Errorf("missing operand")
	}
	cfg.file1 = files[0]
	cfg.file2 = files[1]
	return cfg, nil
}

// parseFlagArg parses a single flag argument like "-1", "-23", "-123".
// Returns false if the argument contains an unknown flag character.
func parseFlagArg(arg string, cfg *commConfig) bool {
	for _, ch := range arg[1:] {
		switch ch {
		case '1':
			cfg.suppress1 = true
		case '2':
			cfg.suppress2 = true
		case '3':
			cfg.suppress3 = true
		default:
			return false
		}
	}
	return true
}

// computePrefixes calculates the tab prefix for each column based on
// which columns are suppressed. R2.4: the leftmost visible column has
// no leading delimiter; each subsequent visible column adds one delimiter.
func computePrefixes(cfg commConfig, delim string) columnPrefixes {
	var p columnPrefixes
	p.col1 = ""
	col2Offset := 0
	if !cfg.suppress1 {
		col2Offset = 1
	}
	p.col2 = strings.Repeat(delim, col2Offset)
	col3Offset := 0
	if !cfg.suppress1 {
		col3Offset++
	}
	if !cfg.suppress2 {
		col3Offset++
	}
	p.col3 = strings.Repeat(delim, col3Offset)
	return p
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

// compare reads two sorted inputs and writes column output.
// R1.1: col1 = unique to file1, col2 = unique to file2, col3 = common.
// R1.2: lexicographic byte comparison determines column.
// R2.1-R2.3: suppressed columns are not written.
func compare(r1, r2 io.Reader, w io.Writer, cfg commConfig, p columnPrefixes) error {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	bw := bufio.NewWriter(w)
	have1 := s1.Scan()
	have2 := s2.Scan()
	for have1 && have2 {
		l1, l2 := s1.Text(), s2.Text()
		var err error
		if l1 < l2 {
			err = writeColumn(bw, p.col1, l1, cfg.suppress1)
			have1 = s1.Scan()
		} else if l2 < l1 {
			err = writeColumn(bw, p.col2, l2, cfg.suppress2)
			have2 = s2.Scan()
		} else {
			err = writeColumn(bw, p.col3, l1, cfg.suppress3)
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
	if err := drainRemaining(bw, s1, have1, p.col1, cfg.suppress1); err != nil {
		return err
	}
	if err := drainRemaining(bw, s2, have2, p.col2, cfg.suppress2); err != nil {
		return err
	}
	return bw.Flush()
}

// writeColumn writes a line with prefix unless the column is suppressed.
func writeColumn(w *bufio.Writer, prefix, line string, suppress bool) error {
	if suppress {
		return nil
	}
	return writeLine(w, prefix, line)
}

// drainRemaining writes all remaining lines from a scanner with the given prefix.
// R1.3: when one file is exhausted, remaining lines go to the appropriate column.
func drainRemaining(w *bufio.Writer, s *bufio.Scanner, hasLine bool, prefix string, suppress bool) error {
	if suppress {
		return nil
	}
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
