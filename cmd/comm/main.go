// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/comm implements GNU comm: compare two sorted files line by line.
//
// Implements prd029-comm R1.1 (three-column output), R1.2 (sorted-order comparison),
// R1.3 (file exhaustion handling), R1.4 (byte-for-byte LC_ALL=C comparison),
// R2.1 (-1 suppresses column 1), R2.2 (-2 suppresses column 2),
// R2.3 (-3 suppresses column 3), R2.4 (indentation adjusts for suppressed columns),
// R3.1 (default order check with warning), R3.2 (--check-order fatal),
// R3.3 (--nocheck-order disables check), R3.4 (--output-delimiter).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "comm"

// errFatalOrder signals --check-order detected unsorted input.
var errFatalOrder = errors.New("fatal order violation")

// commConfig holds the parsed flags for a comm invocation.
type commConfig struct {
	suppress1      bool
	suppress2      bool
	suppress3      bool
	checkOrder     bool
	noCheckOrder   bool
	outputDelim    string
	hasOutputDelim bool
	file1          string
	file2          string
}

// columnPrefixes holds the computed indentation prefix for each column.
// R2.4: when columns are suppressed, remaining columns shift left.
type columnPrefixes struct {
	col1 string
	col2 string
	col3 string
}

// orderChecker tracks sort order for one file and reports violations.
type orderChecker struct {
	prev    string
	hasPrev bool
	warned  bool
	label   string
	stderr  io.Writer
	fatal   bool
	disable bool
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
	delim := "\t"
	if cfg.hasOutputDelim {
		delim = cfg.outputDelim
	}
	prefixes := computePrefixes(cfg, delim)
	return runCompare(r1, r2, stdout, stderr, cfg, prefixes)
}

// runCompare executes comparison and translates errors to exit codes.
func runCompare(r1, r2 io.Reader, stdout, stderr io.Writer, cfg commConfig, p columnPrefixes) int {
	orderViolation, err := compare(r1, r2, stdout, stderr, cfg, p)
	if err != nil {
		if errors.Is(err, errFatalOrder) {
			return 1
		}
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	if orderViolation {
		fmt.Fprintf(stderr, "%s: input is not in sorted order\n", programName)
		return 1
	}
	return 0
}

// parseArgs extracts flags and the two file operands.
func parseArgs(args []string) (commConfig, error) {
	var cfg commConfig
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if parsed, err := parseLongFlag(arg, &cfg); parsed {
			if err != nil {
				return cfg, err
			}
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
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

// parseLongFlag handles --check-order, --nocheck-order, --output-delimiter=STR.
func parseLongFlag(arg string, cfg *commConfig) (bool, error) {
	switch {
	case arg == "--check-order":
		cfg.checkOrder = true
		return true, nil
	case arg == "--nocheck-order":
		cfg.noCheckOrder = true
		return true, nil
	case strings.HasPrefix(arg, "--output-delimiter="):
		cfg.outputDelim = arg[len("--output-delimiter="):]
		cfg.hasOutputDelim = true
		return true, nil
	case strings.HasPrefix(arg, "--"):
		return true, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return false, nil
}

// parseFlagArg parses a single flag argument like "-1", "-23", "-123".
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

// newOrderChecker creates an order checker for one file.
func newOrderChecker(label string, stderr io.Writer, cfg commConfig) orderChecker {
	return orderChecker{
		label:   label,
		stderr:  stderr,
		fatal:   cfg.checkOrder,
		disable: cfg.noCheckOrder,
	}
}

// check validates that line is >= the previously checked line.
// R3.1: default prints warning, returns false. R3.2: --check-order returns true (fatal).
// R3.3: --nocheck-order skips check entirely.
func (o *orderChecker) check(line string) bool {
	if o.disable || !o.hasPrev || line >= o.prev {
		o.prev = line
		o.hasPrev = true
		return false
	}
	fmt.Fprintf(o.stderr, "%s: %s is not in sorted order\n", programName, o.label)
	o.warned = true
	o.prev = line
	return o.fatal
}

// compare reads two sorted inputs and writes column output.
// Returns (orderViolation, error). orderViolation is true if non-fatal warnings were issued.
func compare(r1, r2 io.Reader, w, stderr io.Writer, cfg commConfig, p columnPrefixes) (bool, error) {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	bw := bufio.NewWriter(w)
	oc1 := newOrderChecker("file 1", stderr, cfg)
	oc2 := newOrderChecker("file 2", stderr, cfg)
	have1 := s1.Scan()
	have2 := s2.Scan()
	if err := mainLoop(bw, s1, s2, &have1, &have2, cfg, p, &oc1, &oc2); err != nil {
		bw.Flush() // best-effort flush of buffered output
		return false, err
	}
	if err := checkScanErrors(s1, s2); err != nil {
		return false, err
	}
	if err := drainFile(bw, s1, have1, p.col1, cfg.suppress1, &oc1); err != nil {
		bw.Flush() // best-effort flush
		return false, err
	}
	if err := drainFile(bw, s2, have2, p.col2, cfg.suppress2, &oc2); err != nil {
		bw.Flush() // best-effort flush
		return false, err
	}
	if err := bw.Flush(); err != nil {
		return false, err
	}
	return oc1.warned || oc2.warned, nil
}

// mainLoop runs the comparison loop until one file is exhausted or an error occurs.
func mainLoop(
	bw *bufio.Writer, s1, s2 *bufio.Scanner,
	have1, have2 *bool, cfg commConfig, p columnPrefixes,
	oc1, oc2 *orderChecker,
) error {
	for *have1 && *have2 {
		l1, l2 := s1.Text(), s2.Text()
		if err := stepPair(bw, s1, s2, l1, l2, have1, have2, cfg, p, oc1, oc2); err != nil {
			return err
		}
	}
	return nil
}

// stepPair compares one line from each file and writes the appropriate column.
func stepPair(
	bw *bufio.Writer, s1, s2 *bufio.Scanner,
	l1, l2 string, have1, have2 *bool,
	cfg commConfig, p columnPrefixes,
	oc1, oc2 *orderChecker,
) error {
	if l1 < l2 {
		return stepCol1(bw, s1, l1, have1, cfg, p, oc1)
	}
	if l2 < l1 {
		return stepCol2(bw, s2, l2, have2, cfg, p, oc2)
	}
	return stepCol3(bw, s1, s2, l1, l2, have1, have2, cfg, p, oc1, oc2)
}

// stepCol1 handles a line unique to file1.
func stepCol1(bw *bufio.Writer, s1 *bufio.Scanner, l1 string, have1 *bool, cfg commConfig, p columnPrefixes, oc1 *orderChecker) error {
	if oc1.check(l1) {
		return errFatalOrder
	}
	if err := writeColumn(bw, p.col1, l1, cfg.suppress1); err != nil {
		return err
	}
	*have1 = s1.Scan()
	return nil
}

// stepCol2 handles a line unique to file2.
func stepCol2(bw *bufio.Writer, s2 *bufio.Scanner, l2 string, have2 *bool, cfg commConfig, p columnPrefixes, oc2 *orderChecker) error {
	if oc2.check(l2) {
		return errFatalOrder
	}
	if err := writeColumn(bw, p.col2, l2, cfg.suppress2); err != nil {
		return err
	}
	*have2 = s2.Scan()
	return nil
}

// stepCol3 handles a line common to both files.
func stepCol3(bw *bufio.Writer, s1, s2 *bufio.Scanner, l1, l2 string, have1, have2 *bool, cfg commConfig, p columnPrefixes, oc1, oc2 *orderChecker) error {
	if oc1.check(l1) || oc2.check(l2) {
		return errFatalOrder
	}
	if err := writeColumn(bw, p.col3, l1, cfg.suppress3); err != nil {
		return err
	}
	*have1 = s1.Scan()
	*have2 = s2.Scan()
	return nil
}

// checkScanErrors checks both scanners for read errors.
func checkScanErrors(s1, s2 *bufio.Scanner) error {
	if err := s1.Err(); err != nil {
		return err
	}
	return s2.Err()
}

// writeColumn writes a line with prefix unless the column is suppressed.
func writeColumn(w *bufio.Writer, prefix, line string, suppress bool) error {
	if suppress {
		return nil
	}
	return writeLine(w, prefix, line)
}

// drainFile writes all remaining lines from a scanner with the given prefix.
// R1.3: when one file is exhausted, remaining lines go to the appropriate column.
func drainFile(w *bufio.Writer, s *bufio.Scanner, hasLine bool, prefix string, suppress bool, oc *orderChecker) error {
	if suppress {
		return nil
	}
	for hasLine {
		line := s.Text()
		if oc.check(line) {
			return errFatalOrder
		}
		if err := writeLine(w, prefix, line); err != nil {
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
