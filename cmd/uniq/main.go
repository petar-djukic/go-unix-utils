// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd028-uniq: Report or Filter Adjacent Duplicate Lines.
// Covers R1.1-R1.4 (default deduplication), R2.1-R2.3 (-d, -D, -u),
// R2.4 (-c count prefix), R3.1-R3.4 (-i, -f, -s, -w), R4.1-R4.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
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
	modeUnique                     // -u: only runs of exactly one
)

// repeatMethod controls group separation for -D/--all-repeated.
type repeatMethod int

const (
	methodNone     repeatMethod = iota // no separators
	methodPrepend                      // blank line before each group
	methodSeparate                     // blank line between groups
)

// config holds parsed flag state.
type config struct {
	count      bool         // -c: prefix lines with occurrence count
	mode       outputMode   // output filtering mode
	method     repeatMethod // --all-repeated=METHOD
	ignoreCase bool         // -i: case-insensitive comparison
	skipFields int          // -f N: skip first N fields
	skipChars  int          // -s N: skip first N characters after fields
	checkChars int          // -w N: compare at most N chars; -1 = unlimited
}

func main() {
	// R4.4: Install SIGPIPE handler per shared protocol.
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

// --- Comparison ---

// compareKey extracts the comparison substring from a line.
// R3.2: skip fields, R3.3: skip chars, R3.4: check-chars, R3.1: case fold.
func compareKey(line string, cfg config) string {
	s := skipFieldsN(line, cfg.skipFields)
	if cfg.skipChars > 0 {
		if cfg.skipChars >= len(s) {
			s = ""
		} else {
			s = s[cfg.skipChars:]
		}
	}
	if cfg.checkChars >= 0 && cfg.checkChars < len(s) {
		s = s[:cfg.checkChars]
	}
	if cfg.ignoreCase {
		s = strings.ToLower(s)
	}
	return s
}

// skipFieldsN skips the first n whitespace-separated fields.
// R3.2: a field is a run of blanks then non-blank characters.
func skipFieldsN(s string, n int) string {
	idx := 0
	for f := 0; f < n && idx < len(s); f++ {
		for idx < len(s) && (s[idx] == ' ' || s[idx] == '\t') {
			idx++
		}
		for idx < len(s) && s[idx] != ' ' && s[idx] != '\t' {
			idx++
		}
	}
	return s[idx:]
}

// --- Processing ---

// processInput reads lines and applies deduplication.
// R1.1: suppress adjacent duplicate lines.
// R1.2: non-adjacent duplicates are unaffected.
func processInput(cfg config, r io.Reader, bw *bufio.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var prev string
	var prevKey string
	count := 0
	hasPrev := false
	dupGroups := 0

	for scanner.Scan() {
		line := scanner.Text()
		if !hasPrev {
			prev = line
			prevKey = compareKey(line, cfg)
			count = 1
			hasPrev = true
			continue
		}
		key := compareKey(line, cfg)
		var err error
		if key == prevKey {
			count++
			err = handleDup(cfg, bw, prev, line, count, &dupGroups)
		} else {
			err = endRun(cfg, bw, prev, count)
			prev = line
			prevKey = key
			count = 1
		}
		if err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if hasPrev {
		return endRun(cfg, bw, prev, count)
	}
	return nil
}

// handleDup processes a line that matches the previous run's key.
// R2.2: for -D mode, emit duplicate lines inline.
func handleDup(
	cfg config, bw *bufio.Writer, prev, line string,
	count int, dupGroups *int,
) error {
	if cfg.mode != modeDupAll {
		return nil
	}
	if count == 2 {
		if err := emitGroupSep(bw, cfg.method, *dupGroups); err != nil {
			return err
		}
		*dupGroups++
		if err := writeLine(bw, prev); err != nil {
			return err
		}
	}
	return writeLine(bw, line)
}

// endRun emits a completed run based on mode and count.
// R2.1: -d emits only runs with count >= 2.
// R2.3: -u emits only runs with count == 1.
func endRun(cfg config, bw *bufio.Writer, line string, count int) error {
	switch cfg.mode {
	case modeDupAll:
		return nil // handled inline
	case modeDupFirst:
		if count < 2 {
			return nil
		}
	case modeUnique:
		if count > 1 {
			return nil
		}
	}
	if cfg.count {
		return writeCountLine(bw, count, line)
	}
	return writeLine(bw, line)
}

// emitGroupSep writes a blank line separator for --all-repeated methods.
func emitGroupSep(bw *bufio.Writer, method repeatMethod, groupsSoFar int) error {
	switch method {
	case methodPrepend:
		return bw.WriteByte('\n')
	case methodSeparate:
		if groupsSoFar > 0 {
			return bw.WriteByte('\n')
		}
	}
	return nil
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
	cfg := config{checkChars: -1}
	inFile := "-"
	outFile := "-"
	pos := 0

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			assignPositional(args[i+1:], &inFile, &outFile, &pos)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			assignOnePositional(arg, &inFile, &outFile, &pos)
			continue
		}
		consumed, exit := dispatchFlag(args, i, &cfg)
		if exit >= 0 {
			return config{}, "", "", exit
		}
		i += consumed - 1
	}
	return cfg, inFile, outFile, -1
}

// dispatchFlag routes to long or short flag parsers.
func dispatchFlag(args []string, i int, cfg *config) (int, int) {
	if strings.HasPrefix(args[i], "--") {
		return parseLongFlag(args, i, cfg)
	}
	return parseShortFlags(args, i, cfg)
}

// parseLongFlag handles a --flag or --flag=value argument.
func parseLongFlag(args []string, i int, cfg *config) (int, int) {
	arg := args[i]
	name, val, hasEq := strings.Cut(arg, "=")

	switch name {
	case "--help":
		return 1, printHelp()
	case "--version":
		return 1, printVersion()
	case "--count":
		cfg.count = true
		return 1, -1
	case "--repeated":
		cfg.mode = modeDupFirst
		return 1, -1
	case "--unique":
		cfg.mode = modeUnique
		return 1, -1
	case "--ignore-case":
		cfg.ignoreCase = true
		return 1, -1
	case "--all-repeated":
		return parseLongAllRepeated(hasEq, val, cfg)
	case "--skip-fields":
		return requireLongInt(args, i, hasEq, val, name, &cfg.skipFields)
	case "--skip-chars":
		return requireLongInt(args, i, hasEq, val, name, &cfg.skipChars)
	case "--check-chars":
		return requireLongInt(args, i, hasEq, val, name, &cfg.checkChars)
	default:
		fmt.Fprintf(os.Stderr, "uniq: unrecognized option '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'uniq --help' for more information.")
		return 1, 1
	}
}

// parseLongAllRepeated parses --all-repeated or --all-repeated=METHOD.
func parseLongAllRepeated(hasEq bool, val string, cfg *config) (int, int) {
	cfg.mode = modeDupAll
	if !hasEq || val == "" || val == "none" {
		cfg.method = methodNone
		return 1, -1
	}
	switch val {
	case "prepend":
		cfg.method = methodPrepend
	case "separate":
		cfg.method = methodSeparate
	default:
		fmt.Fprintf(os.Stderr,
			"uniq: invalid argument '%s' for '--all-repeated'\n", val)
		return 1, 1
	}
	return 1, -1
}

// requireLongInt parses a long flag that requires an integer value.
func requireLongInt(
	args []string, i int, hasEq bool, val, name string, target *int,
) (int, int) {
	consumed := 1
	if !hasEq {
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr,
				"uniq: option '%s' requires an argument\n", name)
			return 1, 1
		}
		val = args[i+1]
		consumed = 2
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "uniq: invalid number '%s'\n", val)
		return consumed, 1
	}
	*target = n
	return consumed, -1
}

// parseShortFlags handles combined short flags like -ci or value flags like -f3.
func parseShortFlags(args []string, i int, cfg *config) (int, int) {
	chars := args[i][1:]
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 'c':
			cfg.count = true
		case 'd':
			cfg.mode = modeDupFirst
		case 'D':
			cfg.mode = modeDupAll
		case 'u':
			cfg.mode = modeUnique
		case 'i':
			cfg.ignoreCase = true
		case 'f':
			return parseShortInt(args, i, chars[j+1:], 'f', &cfg.skipFields)
		case 's':
			return parseShortInt(args, i, chars[j+1:], 's', &cfg.skipChars)
		case 'w':
			return parseShortInt(args, i, chars[j+1:], 'w', &cfg.checkChars)
		default:
			fmt.Fprintf(os.Stderr, "uniq: invalid option -- '%c'\n", chars[j])
			fmt.Fprintln(os.Stderr, "Try 'uniq --help' for more information.")
			return 1, 1
		}
	}
	return 1, -1
}

// parseShortInt handles -fN or -f N style integer value flags.
func parseShortInt(
	args []string, i int, rest string, flag byte, target *int,
) (int, int) {
	var val string
	consumed := 1
	if len(rest) > 0 {
		val = rest
	} else if i+1 < len(args) {
		val = args[i+1]
		consumed = 2
	} else {
		fmt.Fprintf(os.Stderr,
			"uniq: option requires an argument -- '%c'\n", flag)
		return 1, 1
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "uniq: invalid number '%s'\n", val)
		return consumed, 1
	}
	*target = n
	return consumed, -1
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
  -D                    print all duplicate lines
      --all-repeated[=METHOD]  like -D, but allow separating groups
                               with an empty line; METHOD={none(default),prepend,separate}
  -f, --skip-fields=N   avoid comparing the first N fields
  -i, --ignore-case     ignore differences in case when comparing
  -s, --skip-chars=N    avoid comparing the first N characters
  -u, --unique          only print unique lines
  -w, --check-chars=N   compare no more than N characters in lines
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
