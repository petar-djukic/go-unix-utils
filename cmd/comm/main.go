// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd029-comm R1.1–R1.4: three-column comparison of two sorted
// files, byte-for-byte under LC_ALL=C, with proper exhaustion handling.
// R2.1–R2.4: column suppression flags (-1, -2, -3) with indentation adjustment.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "comm"

// config holds parsed command-line options for comm.
type config struct {
	suppress1 bool   // R2.1: suppress column 1 (file1-only)
	suppress2 bool   // R2.2: suppress column 2 (file2-only)
	suppress3 bool   // R2.3: suppress column 3 (common)
	prefix1   string // computed prefix for column 1 output
	prefix2   string // computed prefix for column 2 output
	prefix3   string // computed prefix for column 3 output
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and executes the comm comparison, returning exit code.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	cfg, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	r1, c1, err := openFile(files[0], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if c1 != nil {
		defer c1.Close() // best-effort close on read-only file
	}
	r2, c2, err := openFile(files[1], stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if c2 != nil {
		defer c2.Close() // best-effort close on read-only file
	}
	return compareFiles(r1, r2, stdout, stderr, cfg)
}

// parseArgs extracts flags and file operands from args.
// R2.1–R2.4: supports -1, -2, -3 flags for column suppression.
func parseArgs(args []string) (config, []string, error) {
	cfg := config{}
	files := make([]string, 0, 2)
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}
		if err := parseFlag(arg, &cfg); err != nil {
			return config{}, nil, err
		}
	}
	if len(files) != 2 {
		return config{}, nil, fmt.Errorf("missing operand")
	}
	computePrefixes(&cfg)
	return cfg, files, nil
}

// parseFlag parses a single flag argument like "-1", "-2", "-3",
// or combined forms like "-12", "-123".
func parseFlag(arg string, cfg *config) error {
	for _, ch := range arg[1:] {
		switch ch {
		case '1':
			cfg.suppress1 = true
		case '2':
			cfg.suppress2 = true
		case '3':
			cfg.suppress3 = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// computePrefixes sets the tab prefix for each column based on which
// columns are suppressed. R2.4: leftmost visible column has no tab;
// each subsequent visible column adds one tab.
func computePrefixes(cfg *config) {
	delim := "\t"
	col := 0
	if !cfg.suppress1 {
		cfg.prefix1 = strings.Repeat(delim, col)
		col++
	}
	if !cfg.suppress2 {
		cfg.prefix2 = strings.Repeat(delim, col)
		col++
	}
	if !cfg.suppress3 {
		cfg.prefix3 = strings.Repeat(delim, col)
	}
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
// R1.1–R1.4: column assignment by lexicographic comparison.
// R2.1–R2.4: suppressed columns are skipped; prefixes are pre-computed.
func compareFiles(r1, r2 io.Reader, stdout io.Writer, stderr io.Writer, cfg config) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	w := bufio.NewWriter(stdout)

	have1 := s1.Scan()
	have2 := s2.Scan()

	for have1 && have2 {
		line1 := s1.Text()
		line2 := s2.Text()
		if err := emitPair(w, line1, line2, cfg); err != nil {
			return writeError(stderr, err)
		}
		cmp := compareLine(line1, line2)
		have1, have2 = advance(s1, s2, cmp)
	}
	if err := drainCol(w, s1, have1, cfg.suppress1, cfg.prefix1); err != nil {
		return writeError(stderr, err)
	}
	if err := drainCol(w, s2, have2, cfg.suppress2, cfg.prefix2); err != nil {
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
// R2.1–R2.3: suppressed columns produce no output.
func emitPair(w *bufio.Writer, line1, line2 string, cfg config) error {
	cmp := compareLine(line1, line2)
	switch {
	case cmp < 0:
		if cfg.suppress1 {
			return nil
		}
		return writeLine(w, cfg.prefix1, line1)
	case cmp > 0:
		if cfg.suppress2 {
			return nil
		}
		return writeLine(w, cfg.prefix2, line2)
	default:
		if cfg.suppress3 {
			return nil
		}
		return writeLine(w, cfg.prefix3, line1)
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

// drainCol writes remaining lines from a scanner if the column is not suppressed.
func drainCol(w *bufio.Writer, s *bufio.Scanner, haveLine, suppress bool, prefix string) error {
	if suppress {
		return nil
	}
	return drainRemaining(w, s, haveLine, prefix)
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
