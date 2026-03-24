// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd029-comm: Compare Two Sorted Files Line by Line.
// Covers R1.1-R1.4 (three-column comparison), R2.1-R2.2 (column suppression).
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

// config holds parsed flag state.
type config struct {
	suppress1 bool // -1: suppress column 1 (lines unique to file1)
	suppress2 bool // -2: suppress column 2 (lines unique to file2)
	suppress3 bool // -3: suppress column 3 (lines common to both)
}

func main() {
	// R4.4 / D1: Install SIGPIPE handler per shared protocol.
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
	if err := compare(cfg, r1, r2, bw); err != nil {
		fmt.Fprintf(os.Stderr, "comm: %s\n", err)
		return 1
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "comm: write error: %s\n", err)
		return 1
	}
	return 0
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
func formatPathError(name string, err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// columnPrefixes holds the precomputed prefix strings for each column.
type columnPrefixes struct {
	col1 string // prefix for lines unique to file1
	col2 string // prefix for lines unique to file2
	col3 string // prefix for lines common to both
}

// buildPrefixes computes tab prefixes based on suppressed columns.
// R1.2: default delimiter is tab; leading tabs align columns.
// R2.4: suppressed columns shift remaining columns left.
func buildPrefixes(cfg config) columnPrefixes {
	delim := "\t"
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

// compare performs the sorted merge comparison of two files.
// R1.1: lines unique to file1 go to col1, unique to file2 to col2, common to col3.
// R1.2: sorted-order merge using string less-than comparison.
func compare(cfg config, r1, r2 io.Reader, bw *bufio.Writer) error {
	s1 := bufio.NewScanner(r1)
	s2 := bufio.NewScanner(r2)
	s1.Buffer(make([]byte, 64*1024), 1024*1024)
	s2.Buffer(make([]byte, 64*1024), 1024*1024)

	pfx := buildPrefixes(cfg)
	has1 := s1.Scan()
	has2 := s2.Scan()

	for has1 && has2 {
		line1 := s1.Text()
		line2 := s2.Text()
		var err error
		if line1 < line2 {
			err = emitCol1(cfg, bw, pfx, line1)
			has1 = s1.Scan()
		} else if line2 < line1 {
			err = emitCol2(cfg, bw, pfx, line2)
			has2 = s2.Scan()
		} else {
			err = emitCol3(cfg, bw, pfx, line1)
			has1 = s1.Scan()
			has2 = s2.Scan()
		}
		if err != nil {
			return err
		}
	}

	if err := drainRemaining(cfg, bw, pfx, s1, s2, has1, has2); err != nil {
		return err
	}
	return checkScannerErrors(s1, s2)
}

// drainRemaining outputs lines from whichever file still has content.
// R1.3: remaining lines go to col1 (file1 left) or col2 (file2 left).
func drainRemaining(
	cfg config, bw *bufio.Writer, pfx columnPrefixes,
	s1, s2 *bufio.Scanner, has1, has2 bool,
) error {
	for has1 {
		if err := emitCol1(cfg, bw, pfx, s1.Text()); err != nil {
			return err
		}
		has1 = s1.Scan()
	}
	for has2 {
		if err := emitCol2(cfg, bw, pfx, s2.Text()); err != nil {
			return err
		}
		has2 = s2.Scan()
	}
	return nil
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
// R2.2: -3 suppresses this column.
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
		exit := dispatchFlag(arg, &cfg)
		if exit >= 0 {
			return config{}, "", "", exit
		}
	}
	return validatePositionals(cfg, positionals)
}

// dispatchFlag routes to long or short flag parsing.
func dispatchFlag(arg string, cfg *config) int {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, cfg)
	}
	return parseShortFlags(arg[1:], cfg)
}

// parseLongFlag handles --flag arguments.
func parseLongFlag(arg string, cfg *config) int {
	switch arg {
	case "--help":
		return printHelp()
	case "--version":
		return printVersion()
	default:
		fmt.Fprintf(os.Stderr, "comm: unrecognized option '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'comm --help' for more information.")
		return 1
	}
}

// parseShortFlags handles combined short flags like -12 or -123.
// D3: supports combined short flags.
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
