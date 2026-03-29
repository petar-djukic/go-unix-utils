// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uniq implements GNU uniq: report or filter adjacent duplicate lines.
//
// Implements prd028-uniq R1.1-R1.4 (adjacent-line deduplication, I/O, SIGPIPE),
// R2.1 (-d duplicate-only), R2.2 (-D all-duplicates), R2.3 (-u unique-only),
// R2.4 (-c count prefix), R3.1 (-i case-insensitive), R3.2 (-f skip fields),
// R3.3 (-s skip chars), R3.4 (-w compare width).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "uniq"

// options holds the parsed command-line flags.
type options struct {
	count      bool // R2.4: -c prefix lines by count
	dupOnly    bool // R2.1: -d only print duplicate lines (one per run)
	allDup     bool // R2.2: -D print all duplicate lines
	uniqueOnly bool // R2.3: -u only print unique lines
	ignoreCase bool // R3.1: -i case-insensitive comparison
	skipFields int  // R3.2: -f N skip first N fields
	skipChars  int  // R3.3: -s N skip first N characters
	checkWidth int  // R3.4: -w N compare at most N characters
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags and positional arguments, then processes input.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	opts, positional, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	inputFile, outputFile := extractPositional(positional)
	r, closer, err := openInput(inputFile, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	w, wCloser, err := openOutput(outputFile, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if wCloser != nil {
		defer wCloser.Close() // best-effort close
	}
	if err := process(r, w, opts); err != nil {
		fmt.Fprintf(stderr, "%s: write error\n", programName)
		return 1
	}
	return 0
}

// parseFlags parses flag arguments and returns options and remaining positional args.
func parseFlags(args []string) (options, []string, error) {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts options
	fs.BoolVar(&opts.count, "c", false, "prefix lines by count")
	fs.BoolVar(&opts.dupOnly, "d", false, "only print duplicate lines")
	fs.BoolVar(&opts.allDup, "D", false, "print all duplicate lines")
	fs.BoolVar(&opts.uniqueOnly, "u", false, "only print unique lines")
	fs.BoolVar(&opts.ignoreCase, "i", false, "ignore case when comparing")
	fs.IntVar(&opts.skipFields, "f", 0, "skip first N fields")
	fs.IntVar(&opts.skipChars, "s", 0, "skip first N characters")
	fs.IntVar(&opts.checkWidth, "w", 0, "compare at most N characters")
	if err := fs.Parse(args); err != nil {
		return options{}, nil, err
	}
	return opts, fs.Args(), nil
}

// extractPositional extracts input-file and output-file from positional args.
// R1.2: first positional is input, second is output. Both are optional.
func extractPositional(args []string) (string, string) {
	inputFile := "-"
	outputFile := ""
	if len(args) >= 1 {
		inputFile = args[0]
	}
	if len(args) >= 2 {
		outputFile = args[1]
	}
	return inputFile, outputFile
}

// openInput returns a reader and optional closer for the given filename.
// R1.3: "-" means stdin.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: No such file or directory", name)
	}
	return f, f, nil
}

// openOutput returns a writer and optional closer for the given filename.
// Empty string means stdout.
func openOutput(name string, stdout io.Writer) (io.Writer, io.Closer, error) {
	if name == "" {
		return stdout, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	}
	return f, f, nil
}

// compareKey extracts the portion of a line used for comparison after
// applying -f (skip fields), -s (skip chars), and -w (check width).
func compareKey(line string, opts options) string {
	s := line
	s = skipFields(s, opts.skipFields)
	s = skipCharsN(s, opts.skipChars)
	if opts.checkWidth > 0 {
		s = limitWidth(s, opts.checkWidth)
	}
	if opts.ignoreCase {
		s = strings.ToUpper(s)
	}
	return s
}

// skipFields skips the first n whitespace-delimited fields. R3.2.
func skipFields(s string, n int) string {
	pos := 0
	for i := 0; i < n; i++ {
		// Skip leading whitespace before the field.
		for pos < len(s) && isBlank(s[pos]) {
			pos++
		}
		// Skip the field itself (non-whitespace).
		for pos < len(s) && !isBlank(s[pos]) {
			pos++
		}
	}
	return s[pos:]
}

// isBlank returns true for space and tab (GNU uniq field separators).
func isBlank(b byte) bool {
	return b == ' ' || b == '\t'
}

// skipCharsN skips the first n characters (runes) from s. R3.3.
func skipCharsN(s string, n int) string {
	for i := 0; i < n && len(s) > 0; i++ {
		_, size := decodeRune(s)
		s = s[size:]
	}
	return s
}

// decodeRune decodes the first rune of s without importing unicode/utf8
// to avoid unused import issues. Returns the rune and its byte width.
func decodeRune(s string) (rune, int) {
	for i, r := range s {
		_ = i
		if i == 0 {
			// Calculate byte length of first rune.
			for j := range s {
				if j > 0 {
					return r, j
				}
			}
			return r, len(s)
		}
	}
	return unicode.ReplacementChar, 0
}

// limitWidth returns at most n characters (runes) from s. R3.4.
func limitWidth(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// linesEqual returns true if two lines are equal for comparison purposes.
func linesEqual(a, b string, opts options) bool {
	return compareKey(a, opts) == compareKey(b, opts)
}

// process reads lines and writes output according to the selected options.
func process(r io.Reader, w io.Writer, opts options) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	bw := bufio.NewWriter(w)
	prev := ""
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if count == 0 {
			prev = line
			count = 1
			continue
		}
		if linesEqual(line, prev, opts) {
			count++
			continue
		}
		if err := flushRun(bw, prev, count, opts); err != nil {
			return err
		}
		prev = line
		count = 1
	}
	if count > 0 {
		if err := flushRun(bw, prev, count, opts); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// flushRun writes a completed run of identical lines according to opts.
func flushRun(w *bufio.Writer, line string, count int, opts options) error {
	if opts.allDup {
		return flushAllDup(w, line, count)
	}
	if opts.dupOnly && count <= 1 {
		return nil
	}
	if opts.uniqueOnly && count != 1 {
		return nil
	}
	return writeCountedLine(w, line, count, opts.count)
}

// flushAllDup writes every copy of a duplicate run (R2.2: -D).
func flushAllDup(w *bufio.Writer, line string, count int) error {
	if count <= 1 {
		return nil
	}
	for range count {
		if err := writeLine(w, line); err != nil {
			return err
		}
	}
	return nil
}

// writeCountedLine writes a line with an optional count prefix (R2.4: -c).
func writeCountedLine(w *bufio.Writer, line string, count int, showCount bool) error {
	if showCount {
		_, err := fmt.Fprintf(w, "%7d %s\n", count, line)
		return err
	}
	return writeLine(w, line)
}

// writeLine writes a single line followed by a newline.
func writeLine(w *bufio.Writer, line string) error {
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
