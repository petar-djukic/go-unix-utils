// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd028-uniq R1.1–R1.4: default adjacent-duplicate suppression,
// stdin/file input, optional output file, case-sensitive comparison.
// Implements prd028-uniq R2.1–R2.4: -d, -D, -u, -c output selection flags.
// Implements prd028-uniq R3.1–R3.4: -i, -f, -s, -w comparison option flags.
// Implements prd028-uniq R4.1–R4.4: exit codes and SIGPIPE handling.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "uniq"

// options holds parsed command-line flags for uniq.
type options struct {
	countMode  bool // R2.4: -c prefix lines with occurrence count
	dupOnly    bool // R2.1: -d print only duplicate runs (one copy)
	allDups    bool // R2.2: -D print all lines of duplicate runs
	uniqueOnly bool // R2.3: -u print only unique lines
	ignoreCase bool // R3.1: -i case-insensitive comparison
	skipFields int  // R3.2: -f N skip first N fields
	skipChars  int  // R3.3: -s N skip first N characters
	checkChars int  // R3.4: -w N compare only first N chars; -1 = no limit
	inputFile  string
	outputFile string
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes input, returning the exit code.
// R1.3: reads stdin when no file argument given; accepts optional output file.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	reader, closer, err := openInput(opts.inputFile, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if closer != nil {
		defer closer.Close() // best-effort close on read-only file
	}
	writer, wCloser, err := openOutput(opts.outputFile, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if wCloser != nil {
		defer wCloser.Close() // best-effort close on output file
	}
	return processInput(reader, writer, stderr, opts)
}

// parseArgs extracts flags and input/output file arguments.
func parseArgs(args []string) (options, error) {
	opts := options{checkChars: -1}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			if err := parseFlagGroup(&opts, arg[1:], args, &i); err != nil {
				return options{}, err
			}
			continue
		}
		positional = append(positional, arg)
	}
	if len(positional) > 2 {
		return options{}, fmt.Errorf("extra operand '%s'", positional[2])
	}
	if len(positional) >= 1 {
		opts.inputFile = positional[0]
	}
	if len(positional) >= 2 {
		opts.outputFile = positional[1]
	}
	return opts, nil
}

// parseFlagGroup processes a string of short flags, delegating to
// parseNumericFlag when a flag requires a numeric argument.
func parseFlagGroup(opts *options, flags string, args []string, idx *int) error {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'c':
			opts.countMode = true
		case 'd':
			opts.dupOnly = true
		case 'D':
			opts.allDups = true
		case 'u':
			opts.uniqueOnly = true
		case 'i':
			opts.ignoreCase = true
		case 'f', 's', 'w':
			return parseNumericFlag(opts, flags[j], flags[j+1:], args, idx)
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return nil
}

// parseNumericFlag parses a flag that takes a numeric argument. The value
// is taken from rest (inline, e.g. -f3) or from the next argument (-f 3).
func parseNumericFlag(opts *options, flag byte, rest string, args []string, idx *int) error {
	val := rest
	if val == "" {
		if *idx+1 >= len(args) {
			return fmt.Errorf("option requires an argument -- '%c'", flag)
		}
		*idx++
		val = args[*idx]
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of %s: '%s'", flagDesc(flag), val)
	}
	assignNumericOpt(opts, flag, n)
	return nil
}

// flagDesc returns a human-readable description for a numeric flag.
func flagDesc(flag byte) string {
	switch flag {
	case 'f':
		return "fields to skip"
	case 's':
		return "bytes to skip"
	case 'w':
		return "bytes to compare"
	}
	return "unknown"
}

// assignNumericOpt sets the appropriate option field for the given flag.
func assignNumericOpt(opts *options, flag byte, n int) {
	switch flag {
	case 'f':
		opts.skipFields = n
	case 's':
		opts.skipChars = n
	case 'w':
		opts.checkChars = n
	}
}

// openInput opens the input source. Returns the reader and an optional closer.
// R1.3: stdin when no file or "-" is given.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "" || name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, formatPathError(name, err)
	}
	return f, f, nil
}

// openOutput opens the output destination. Returns the writer and an optional closer.
// R1.3: stdout when no output file is given.
func openOutput(name string, stdout io.Writer) (io.Writer, io.Closer, error) {
	if name == "" {
		return stdout, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, formatPathError(name, err)
	}
	return f, f, nil
}

// processInput reads lines, groups adjacent duplicates, and applies output
// selection based on flags.
// R1.1: writes first occurrence of each run (default).
// R1.2: non-adjacent duplicates are unaffected.
// R1.4: comparison is case-sensitive, includes newline.
// R2.1–R2.4: output selection via -d, -D, -u, -c.
// R3.1–R3.4: comparison via -i, -f, -s, -w.
func processInput(reader io.Reader, writer io.Writer, stderr io.Writer, opts options) int {
	scanner := bufio.NewScanner(reader)
	w := bufio.NewWriter(writer)
	var runLines [][]byte
	var prevLine []byte
	first := true
	for scanner.Scan() {
		line := copyBytes(scanner.Bytes())
		if first {
			runLines = append(runLines, line)
			prevLine = line
			first = false
			continue
		}
		if compareEqual(line, prevLine, opts) {
			runLines = append(runLines, line)
			continue
		}
		if err := emitRun(w, runLines, opts); err != nil {
			fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
			return 1
		}
		runLines = runLines[:0]
		runLines = append(runLines, line)
		prevLine = line
	}
	return finishInput(scanner, w, stderr, runLines, opts)
}

// finishInput handles scanner errors and emits the final run.
func finishInput(scanner *bufio.Scanner, w *bufio.Writer, stderr io.Writer, runLines [][]byte, opts options) int {
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if len(runLines) > 0 {
		if err := emitRun(w, runLines, opts); err != nil {
			fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
			return 1
		}
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
		return 1
	}
	return 0
}

// compareEqual returns true if two lines are equal under the active comparison
// options. R3.1: -i case fold. R3.2–R3.4: key extraction via -f, -s, -w.
func compareEqual(a, b []byte, opts options) bool {
	ka := extractKey(a, opts)
	kb := extractKey(b, opts)
	if opts.ignoreCase {
		return bytes.EqualFold(ka, kb)
	}
	return bytes.Equal(ka, kb)
}

// extractKey applies -f (skip fields), -s (skip chars), and -w (limit chars)
// to produce the comparison key from a line.
func extractKey(line []byte, opts options) []byte {
	key := line
	if opts.skipFields > 0 {
		key = skipFieldsN(key, opts.skipFields)
	}
	if opts.skipChars > 0 {
		key = skipBytesN(key, opts.skipChars)
	}
	if opts.checkChars >= 0 && len(key) > opts.checkChars {
		key = key[:opts.checkChars]
	}
	return key
}

// skipFieldsN skips the first n whitespace-delimited fields in line.
// R3.2: a field is a run of blanks followed by non-blanks.
func skipFieldsN(line []byte, n int) []byte {
	pos := 0
	for i := 0; i < n && pos < len(line); i++ {
		for pos < len(line) && isBlank(line[pos]) {
			pos++
		}
		for pos < len(line) && !isBlank(line[pos]) {
			pos++
		}
	}
	return line[pos:]
}

// isBlank returns true if b is a space or tab (POSIX blank).
func isBlank(b byte) bool {
	return b == ' ' || b == '\t'
}

// skipBytesN skips the first n bytes of line. R3.3.
func skipBytesN(line []byte, n int) []byte {
	if n >= len(line) {
		return nil
	}
	return line[n:]
}

// emitRun outputs a completed run of identical lines based on the active flags.
// R2.1: -d prints one copy if count > 1.
// R2.2: -D prints all copies if count > 1.
// R2.3: -u prints one copy if count == 1.
// R2.4: -c prefixes with right-justified count.
func emitRun(w *bufio.Writer, lines [][]byte, opts options) error {
	count := len(lines)
	if !shouldEmit(count, opts) {
		return nil
	}
	if opts.allDups {
		return emitAllDups(w, lines, opts)
	}
	return emitSingle(w, lines[0], count, opts)
}

// shouldEmit determines whether a run with the given count should be emitted.
func shouldEmit(count int, opts options) bool {
	if opts.dupOnly && count < 2 {
		return false
	}
	if opts.allDups && count < 2 {
		return false
	}
	if opts.uniqueOnly && count != 1 {
		return false
	}
	return true
}

// emitAllDups writes all lines of a duplicate run. R2.2: -D mode.
func emitAllDups(w *bufio.Writer, lines [][]byte, opts options) error {
	for _, line := range lines {
		if err := emitSingle(w, line, len(lines), opts); err != nil {
			return err
		}
	}
	return nil
}

// emitSingle writes a single output line, optionally prefixed with count.
func emitSingle(w *bufio.Writer, line []byte, count int, opts options) error {
	if opts.countMode {
		if _, err := fmt.Fprintf(w, "%7d ", count); err != nil {
			return err
		}
	}
	return writeLine(w, line)
}

// writeLine writes a line followed by a newline to the buffered writer.
func writeLine(w *bufio.Writer, line []byte) error {
	if _, err := w.Write(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// copyBytes returns a copy of b, since scanner.Bytes() is only valid until
// the next Scan call.
func copyBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// formatPathError formats an error with the filename for GNU-compatible
// error messages. R4.2: "uniq: filename: error message".
func formatPathError(name string, err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}
