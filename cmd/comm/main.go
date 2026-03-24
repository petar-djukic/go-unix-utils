// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd029-comm: Compare Two Sorted Files Line by Line.
// Covers R1.1-R1.4 (three-column comparison), R2.1-R2.4 (column suppression),
// R3.1-R3.4 (order checking, output delimiter, total), R4.1-R4.4 (exit codes, SIGPIPE).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// orderMode controls how sort-order violations are handled.
type orderMode int

const (
	orderDefault orderMode = iota // R3.1: warn on stderr, continue
	orderCheck                    // R3.2: fatal error on unsorted input
	orderNoCheck                  // R3.3: suppress all order checking
)

// config holds parsed flag state.
type config struct {
	suppress1      bool      // -1: suppress column 1 (lines unique to file1)
	suppress2      bool      // -2: suppress column 2 (lines unique to file2)
	suppress3      bool      // -3: suppress column 3 (lines common to both)
	outputDelim    string    // R3.4: custom column separator
	hasOutputDelim bool      // whether --output-delimiter was specified
	order          orderMode // R3.1-R3.3: sort-order checking mode
	total          bool      // --total: append summary line with counts
}

// columnPrefixes holds the precomputed prefix strings for each column.
type columnPrefixes struct {
	col1 string // prefix for lines unique to file1
	col2 string // prefix for lines unique to file2
	col3 string // prefix for lines common to both
}

// counts tracks lines per column for --total.
type counts struct {
	col1 int
	col2 int
	col3 int
}

// orderChecker tracks sort order for a single file.
type orderChecker struct {
	prevLine string
	hasPrev  bool
	warned   bool
	fileNum  int
}

func main() {
	// R4.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, file1, file2, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, file1, file2))
}

// run opens both files, performs comparison, and returns exit code.
// R1.4: exit 0 on success, non-zero on error.
func run(cfg config, name1, name2 string) int {
	r1, r2, cleanup, err := openBothInputs(name1, name2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: %s\n", err)
		return 1
	}
	defer cleanup()

	bw := bufio.NewWriter(os.Stdout)
	exitCode := compare(cfg, r1, r2, bw)
	if flushErr := bw.Flush(); flushErr != nil {
		fmt.Fprintf(os.Stderr, "comm: write error: %s\n", flushErr)
		return 1
	}
	return exitCode
}

// openBothInputs opens both input files, supporting '-' for stdin.
// R1.3: accept '-' to read from stdin for one of the two files.
func openBothInputs(name1, name2 string) (io.Reader, io.Reader, func(), error) {
	r1, c1, err := openInput(name1)
	if err != nil {
		return nil, nil, nil, err
	}
	r2, c2, err := openInput(name2)
	if err != nil {
		c1()
		return nil, nil, nil, err
	}
	cleanup := func() { c1(); c2() }
	return r1, r2, cleanup, nil
}

// openInput opens a file or returns stdin for "-".
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, formatPathError(name, err)
	}
	return f, func() { f.Close() }, nil
}

// formatPathError produces a GNU-compatible error message.
// R3.4: error messages use 'comm: ' prefix.
func formatPathError(name string, err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// delimiter returns the column separator string.
// R3.4: --output-delimiter replaces the default tab.
// GNU comm uses NUL when --output-delimiter is set to empty string.
func delimiter(cfg config) string {
	if cfg.hasOutputDelim {
		if cfg.outputDelim == "" {
			return "\x00"
		}
		return cfg.outputDelim
	}
	return "\t"
}

// buildPrefixes computes delimiter prefixes based on suppressed columns.
// R1.2: default delimiter is tab; leading delimiters align columns.
// R2.4: suppressed columns shift remaining columns left.
func buildPrefixes(cfg config) columnPrefixes {
	delim := delimiter(cfg)
	col2Offset := 0
	if !cfg.suppress1 {
		col2Offset = 1
	}
	col3Offset := col2Offset
	if !cfg.suppress2 {
		col3Offset++
	}
	return columnPrefixes{
		col1: "",
		col2: strings.Repeat(delim, col2Offset),
		col3: strings.Repeat(delim, col3Offset),
	}
}

// checkLine verifies that the current line is in sorted order.
// R3.1: default mode warns and continues. R3.2: --check-order is fatal.
// Returns true if processing should stop (--check-order violation).
func (oc *orderChecker) checkLine(line string, mode orderMode) bool {
	if mode == orderNoCheck {
		oc.prevLine = line
		oc.hasPrev = true
		return false
	}
	if oc.hasPrev && line < oc.prevLine && !oc.warned {
		fmt.Fprintf(os.Stderr, "comm: file %d is not in sorted order\n", oc.fileNum)
		oc.warned = true
		if mode == orderCheck {
			return true
		}
	}
	oc.prevLine = line
	oc.hasPrev = true
	return false
}

// compare performs the sorted merge comparison of two files.
// R1.1: three-column output. R3.1-R3.3: order checking.
func compare(cfg config, r1, r2 io.Reader, bw *bufio.Writer) int {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	s1.Buffer(make([]byte, 64*1024), 1024*1024)
	s2.Buffer(make([]byte, 64*1024), 1024*1024)

	pfx := buildPrefixes(cfg)
	oc1 := &orderChecker{fileNum: 1}
	oc2 := &orderChecker{fileNum: 2}
	var ct counts

	has1 := s1.Scan()
	has2 := s2.Scan()

	code, has1, has2 := mergeLoop(cfg, s1, s2, has1, has2, bw, pfx, oc1, oc2, &ct)
	if code != 0 {
		return code
	}
	if code = drainRemaining(cfg, bw, pfx, s1, s2, has1, has2, oc1, oc2, &ct); code != 0 {
		return code
	}
	if cfg.total {
		if err := emitTotal(cfg, bw, ct); err != nil {
			fmt.Fprintf(os.Stderr, "comm: write error: %s\n", err)
			return 1
		}
	}
	if err := checkScannerErrors(s1, s2); err != nil {
		fmt.Fprintf(os.Stderr, "comm: %s\n", err)
		return 1
	}
	if oc1.warned || oc2.warned {
		fmt.Fprintln(os.Stderr, "comm: input is not in sorted order")
		return 1
	}
	return 0
}

// mergeLoop processes lines while both files have data.
// R1.2: sorted-order merge using string less-than comparison.
func mergeLoop(
	cfg config, s1, s2 *bufio.Scanner, has1, has2 bool,
	bw *bufio.Writer, pfx columnPrefixes,
	oc1, oc2 *orderChecker, ct *counts,
) (int, bool, bool) {
	for has1 && has2 {
		line1 := s1.Text()
		line2 := s2.Text()
		if oc1.checkLine(line1, cfg.order) || oc2.checkLine(line2, cfg.order) {
			return 1, has1, has2
		}
		var code int
		code, has1, has2 = mergeStep(cfg, s1, s2, has1, has2, bw, pfx, line1, line2, ct)
		if code != 0 {
			return code, has1, has2
		}
	}
	return 0, has1, has2
}

// mergeStep processes a single pair of lines from the merge.
func mergeStep(
	cfg config, s1, s2 *bufio.Scanner, has1, has2 bool,
	bw *bufio.Writer, pfx columnPrefixes,
	line1, line2 string, ct *counts,
) (int, bool, bool) {
	var err error
	if line1 < line2 {
		ct.col1++
		err = emitCol1(cfg, bw, pfx, line1)
		has1 = s1.Scan()
	} else if line2 < line1 {
		ct.col2++
		err = emitCol2(cfg, bw, pfx, line2)
		has2 = s2.Scan()
	} else {
		ct.col3++
		err = emitCol3(cfg, bw, pfx, line1)
		has1 = s1.Scan()
		has2 = s2.Scan()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "comm: write error: %s\n", err)
		return 1, has1, has2
	}
	return 0, has1, has2
}

// drainRemaining outputs lines from whichever file still has content.
// R1.3: remaining lines go to col1 (file1 left) or col2 (file2 left).
func drainRemaining(
	cfg config, bw *bufio.Writer, pfx columnPrefixes,
	s1, s2 *bufio.Scanner, has1, has2 bool,
	oc1, oc2 *orderChecker, ct *counts,
) int {
	if code := drainFile(cfg, bw, pfx, s1, has1, oc1, emitCol1, &ct.col1); code != 0 {
		return code
	}
	return drainFile(cfg, bw, pfx, s2, has2, oc2, emitCol2, &ct.col2)
}

// emitFunc is a function that emits a line for a specific column.
type emitFunc func(config, *bufio.Writer, columnPrefixes, string) error

// drainFile outputs remaining lines from a single file.
func drainFile(
	cfg config, bw *bufio.Writer, pfx columnPrefixes,
	s *bufio.Scanner, has bool, oc *orderChecker,
	emit emitFunc, count *int,
) int {
	for has {
		line := s.Text()
		if oc.checkLine(line, cfg.order) {
			return 1
		}
		*count++
		if err := emit(cfg, bw, pfx, line); err != nil {
			fmt.Fprintf(os.Stderr, "comm: write error: %s\n", err)
			return 1
		}
		has = s.Scan()
	}
	return 0
}

// checkScannerErrors checks both scanners for read errors.
func checkScannerErrors(s1, s2 *bufio.Scanner) error {
	if err := s1.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	if err := s2.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	return nil
}

// emitCol1 writes a column-1 line (unique to file1) if not suppressed.
// R2.1: -1 suppresses this column.
func emitCol1(cfg config, bw *bufio.Writer, pfx columnPrefixes, line string) error {
	if cfg.suppress1 {
		return nil
	}
	return writePrefixedLine(bw, pfx.col1, line)
}

// emitCol2 writes a column-2 line (unique to file2) if not suppressed.
// R2.2: -2 suppresses this column.
func emitCol2(cfg config, bw *bufio.Writer, pfx columnPrefixes, line string) error {
	if cfg.suppress2 {
		return nil
	}
	return writePrefixedLine(bw, pfx.col2, line)
}

// emitCol3 writes a column-3 line (common to both) if not suppressed.
// R2.3: -3 suppresses this column.
func emitCol3(cfg config, bw *bufio.Writer, pfx columnPrefixes, line string) error {
	if cfg.suppress3 {
		return nil
	}
	return writePrefixedLine(bw, pfx.col3, line)
}

// writePrefixedLine writes prefix + line + newline to the buffered writer.
func writePrefixedLine(bw *bufio.Writer, prefix, line string) error {
	if _, err := bw.WriteString(prefix); err != nil {
		return err
	}
	if _, err := bw.WriteString(line); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// emitTotal writes the --total summary line with column counts.
// GNU comm always prints all three column counts regardless of suppression.
func emitTotal(cfg config, bw *bufio.Writer, ct counts) error {
	delim := delimiter(cfg)
	if _, err := fmt.Fprintf(bw, "%d", ct.col1); err != nil {
		return err
	}
	bw.WriteString(delim) //nolint:errcheck // checked at final write
	if _, err := fmt.Fprintf(bw, "%d", ct.col2); err != nil {
		return err
	}
	bw.WriteString(delim) //nolint:errcheck // checked at final write
	if _, err := fmt.Fprintf(bw, "%d", ct.col3); err != nil {
		return err
	}
	bw.WriteString(delim) //nolint:errcheck // checked at final write
	_, err := bw.WriteString("total\n")
	return err
}

// --- Flag parsing ---

// parseArgs processes command-line flags and returns config, file names, exit code.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (config, string, string, int) {
	var cfg config
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			exit := parseLongFlag(arg, args, &i, &cfg)
			if exit >= 0 {
				return config{}, "", "", exit
			}
		} else {
			exit := parseShortFlags(arg[1:], &cfg)
			if exit >= 0 {
				return config{}, "", "", exit
			}
		}
	}
	return validatePositionals(cfg, positionals)
}

// parseLongFlag handles --flag arguments.
// R3.2-R3.4: --check-order, --nocheck-order, --output-delimiter, --total.
func parseLongFlag(arg string, args []string, i *int, cfg *config) int {
	if strings.HasPrefix(arg, "--output-delimiter=") {
		cfg.outputDelim = arg[len("--output-delimiter="):]
		cfg.hasOutputDelim = true
		return -1
	}
	switch arg {
	case "--output-delimiter":
		return parseDelimiterNextArg(args, i, cfg)
	case "--check-order":
		cfg.order = orderCheck
	case "--nocheck-order":
		cfg.order = orderNoCheck
	case "--total":
		cfg.total = true
	case "--help":
		return printHelp()
	case "--version":
		return printVersion()
	default:
		fmt.Fprintf(os.Stderr, "comm: unrecognized option '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'comm --help' for more information.")
		return 1
	}
	return -1
}

// parseDelimiterNextArg reads the next argument as the output delimiter value.
func parseDelimiterNextArg(args []string, i *int, cfg *config) int {
	*i++
	if *i >= len(args) {
		fmt.Fprintln(os.Stderr, "comm: option '--output-delimiter' requires an argument")
		fmt.Fprintln(os.Stderr, "Try 'comm --help' for more information.")
		return 1
	}
	cfg.outputDelim = args[*i]
	cfg.hasOutputDelim = true
	return -1
}

// parseShortFlags handles combined short flags like -12 or -123.
func parseShortFlags(chars string, cfg *config) int {
	for _, c := range chars {
		switch c {
		case '1':
			cfg.suppress1 = true
		case '2':
			cfg.suppress2 = true
		case '3':
			cfg.suppress3 = true
		default:
			fmt.Fprintf(os.Stderr, "comm: invalid option -- '%c'\n", c)
			fmt.Fprintln(os.Stderr, "Try 'comm --help' for more information.")
			return 1
		}
	}
	return -1
}

// validatePositionals checks that exactly two file arguments are provided.
// R3.4: error messages use 'comm: ' prefix.
func validatePositionals(cfg config, pos []string) (config, string, string, int) {
	if len(pos) < 2 {
		fmt.Fprintln(os.Stderr, "comm: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'comm --help' for more information.")
		return config{}, "", "", 1
	}
	if len(pos) > 2 {
		fmt.Fprintf(os.Stderr, "comm: extra operand '%s'\n", pos[2])
		fmt.Fprintln(os.Stderr, "Try 'comm --help' for more information.")
		return config{}, "", "", 1
	}
	return cfg, pos[0], pos[1], -1
}

// --- Help and version ---

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: comm [OPTION]... FILE1 FILE2
Compare sorted files FILE1 and FILE2 line by line.

With no options, produce three-column output. Column one contains
lines unique to FILE1, column two contains lines unique to FILE2,
and column three contains lines common to both files.

  -1              suppress column 1 (lines unique to FILE1)
  -2              suppress column 2 (lines unique to FILE2)
  -3              suppress column 3 (lines that appear in both files)
      --check-order     check that the input is correctly sorted, even
                          if all input lines are pairable
      --nocheck-order   do not check that the input is correctly sorted
      --output-delimiter=STR  separate columns with STR
      --total           output a summary
      --help      display this help and exit
      --version   output version information and exit

A file name of '-' means standard input.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "comm (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
