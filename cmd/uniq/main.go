// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd028-uniq: Report or Filter Adjacent Duplicate Lines.
// Covers R1.1-R1.4 (default deduplication, input/output, case-sensitive comparison),
// R2.1 (-d duplicate-only), R2.2 (-D all-duplicates).
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

// outputMode specifies which lines to output.
type outputMode int

const (
	modeDefault  outputMode = iota // suppress adjacent duplicates, emit one per run
	modeDupFirst                   // -d: one copy of duplicated runs only
	modeDupAll                     // -D: all copies of duplicated runs
)

// config holds parsed flag state.
type config struct {
	count bool       // -c: prefix lines with occurrence count
	mode  outputMode // output filtering mode
}

func main() {
	// R1.4/R4.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, inFile, outFile, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, inFile, outFile))
}

// run processes input and returns the exit code.
// R1.3: reads stdin when no input file given.
// R4.1/R4.2: exit 0 on success, exit 1 on error.
func run(cfg config, inFile, outFile string) int {
	r, err := openInput(inFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		return 1
	}
	if inFile != "-" {
		defer r.Close()
	}
	w, cleanup, err := openOutput(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		return 1
	}
	defer cleanup()
	bw := bufio.NewWriter(w)
	if err := processInput(cfg, r, bw); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: write error: %s\n", err)
		return 1
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: write error: %s\n", err)
		return 1
	}
	return 0
}

// openInput opens a file or returns stdin for "-".
// R1.3: "-" means stdin.
func openInput(name string) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatPathError(name, err)
	}
	return f, nil
}

// openOutput opens an output file or returns stdout for "-".
// R1.3: optional output file as second argument.
func openOutput(name string) (io.Writer, func(), error) {
	if name == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(name)
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

// processInput reads lines and applies deduplication.
// R1.1: suppress adjacent duplicate lines.
// R1.2: non-adjacent duplicates are unaffected.
func processInput(cfg config, r io.Reader, bw *bufio.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var prev string
	count := 0
	hasPrev := false

	for scanner.Scan() {
		line := scanner.Text()
		if !hasPrev {
			prev = line
			count = 1
			hasPrev = true
			continue
		}
		if err := handleLine(cfg, line, &prev, &count, bw); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if hasPrev && cfg.mode != modeDupAll {
		return emitRun(bw, cfg, prev, count)
	}
	return nil
}

// handleLine processes a single line against the current run.
func handleLine(
	cfg config, line string, prev *string, count *int, bw *bufio.Writer,
) error {
	// R1.4: case-sensitive comparison by default.
	if line == *prev {
		*count++
		if cfg.mode == modeDupAll {
			return handleDupAll(bw, *prev, line, *count)
		}
		return nil
	}
	// Run ended; emit previous run for non-DupAll modes.
	if cfg.mode != modeDupAll {
		if err := emitRun(bw, cfg, *prev, *count); err != nil {
			return err
		}
	}
	*prev = line
	*count = 1
	return nil
}

// handleDupAll handles inline output for -D mode.
// R2.2: print all copies of duplicated runs.
func handleDupAll(bw *bufio.Writer, prev, line string, count int) error {
	if count == 2 {
		// First time dup detected: emit the first occurrence.
		if err := writeLine(bw, prev); err != nil {
			return err
		}
	}
	return writeLine(bw, line)
}

// emitRun outputs a completed run based on mode and count.
// R2.1: -d emits only runs with count >= 2.
func emitRun(bw *bufio.Writer, cfg config, line string, count int) error {
	switch cfg.mode {
	case modeDefault:
		// Always emit one copy.
	case modeDupFirst:
		if count < 2 {
			return nil
		}
	case modeDupAll:
		return nil // handled inline
	}
	if cfg.count {
		return writeCountLine(bw, count, line)
	}
	return writeLine(bw, line)
}

// writeCountLine writes a line prefixed with its occurrence count.
// R2.4: right-justified 7-wide count field, followed by a space and the line.
func writeCountLine(bw *bufio.Writer, count int, line string) error {
	_, err := fmt.Fprintf(bw, "%7d %s\n", count, line)
	return err
}

// writeLine writes a single line with a trailing newline.
func writeLine(bw *bufio.Writer, line string) error {
	if _, err := bw.WriteString(line); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// --- Flag parsing ---

// parseArgs processes command-line flags and returns config, input file, output file, exit code.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (config, string, string, int) {
	var cfg config
	inFile := "-"
	outFile := "-"
	positional := 0

	for i := 0; i < len(args); i++ {
		consumed, exit := parseArg(args, i, &cfg, &inFile, &outFile, &positional)
		if exit >= 0 {
			return config{}, "", "", exit
		}
		i += consumed - 1
	}
	return cfg, inFile, outFile, -1
}

// parseArg handles a single argument.
// Returns (args consumed, exit code). exit=-1 means continue.
func parseArg(
	args []string, i int, cfg *config,
	inFile, outFile *string, positional *int,
) (int, int) {
	arg := args[i]
	if arg == "--" {
		assignPositional(args[i+1:], inFile, outFile, positional)
		return len(args) - i, -1
	}
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		assignOnePositional(arg, inFile, outFile, positional)
		return 1, -1
	}
	return parseFlag(arg, cfg)
}

// parseFlag handles a single flag argument.
func parseFlag(arg string, cfg *config) (int, int) {
	switch arg {
	case "--help":
		return 1, printHelp()
	case "--version":
		return 1, printVersion()
	case "-c", "--count":
		cfg.count = true
		return 1, -1
	case "-d", "--repeated":
		cfg.mode = modeDupFirst
		return 1, -1
	case "-D", "--all-repeated":
		cfg.mode = modeDupAll
		return 1, -1
	default:
		return parseShortFlags(arg, cfg)
	}
}

// parseShortFlags handles combined short flags like -cd.
func parseShortFlags(arg string, cfg *config) (int, int) {
	for _, ch := range arg[1:] {
		switch ch {
		case 'c':
			cfg.count = true
		case 'd':
			cfg.mode = modeDupFirst
		case 'D':
			cfg.mode = modeDupAll
		default:
			fmt.Fprintf(os.Stderr, "uniq: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'uniq --help' for more information.")
			return 1, 1
		}
	}
	return 1, -1
}

// assignPositional assigns remaining args as positional parameters.
func assignPositional(args []string, inFile, outFile *string, pos *int) {
	for _, a := range args {
		assignOnePositional(a, inFile, outFile, pos)
	}
}

// assignOnePositional assigns a single positional argument.
// R1.3: first positional is input file, second is output file.
func assignOnePositional(arg string, inFile, outFile *string, pos *int) {
	switch *pos {
	case 0:
		*inFile = arg
	case 1:
		*outFile = arg
	default:
		fmt.Fprintf(os.Stderr, "uniq: extra operand '%s'\n", arg)
	}
	*pos++
}

// --- Help and version ---

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: uniq [OPTION]... [INPUT [OUTPUT]]
Filter adjacent matching lines from INPUT (or standard input),
writing to OUTPUT (or standard output).

With no options, matching lines are merged to the first occurrence.

  -c, --count           prefix lines by the number of occurrences
  -d, --repeated        only print duplicate lines, one for each group
  -D, --all-repeated    print all duplicate lines
      --help     display this help and exit
      --version  output version information and exit

A field is a run of blanks (usually spaces and/or TABs), then non-blank
characters. Fields are skipped before chars.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "uniq (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
