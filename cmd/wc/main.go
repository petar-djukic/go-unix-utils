// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd005-wc R1.1–R1.4: default wc behavior with line, word,
// and byte counting from stdin or named files, with totals for multiple files.
// Implements prd005-wc R2.1–R2.6: flag behavior for -l, -w, -c, -m, -L.
// Implements prd005-wc R3.1–R3.2: multi-file column alignment and total line.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "wc"

// counts holds line, word, byte, character, and max-line-length counts
// for a single input.
type counts struct {
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
}

// options controls which counts to print. R2.6: output order is fixed as
// lines, words, chars/bytes, max-line-length regardless of flag order.
type options struct {
	printLines      bool
	printWords      bool
	printBytes      bool
	printChars      bool
	printMaxLineLen bool
}

// defaultOptions returns default behavior: print lines, words, bytes. R1.1.
func defaultOptions() options {
	return options{printLines: true, printWords: true, printBytes: true}
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes files, returning the exit code.
// R1.2: reads stdin when no file args; reads named files in order.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files := parseArgs(args)
	if len(files) == 0 {
		// R1.3: implicit stdin, no filename printed.
		files = []string{""}
	}
	return processFiles(files, opts, stdin, stdout, stderr)
}

// parseArgs extracts options and file arguments from the command line.
// Handles "--" as end-of-flags and "-" as explicit stdin.
func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string
	flagsSet := false
	flagsDone := false

	for _, arg := range args {
		if flagsDone || arg == "-" {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				if setFlag(&opts, ch) {
					flagsSet = true
				}
			}
			continue
		}
		files = append(files, arg)
	}
	if !flagsSet {
		return defaultOptions(), files
	}
	return opts, files
}

// setFlag sets the option corresponding to the flag character.
// Returns true if the character is a recognized flag.
func setFlag(opts *options, ch rune) bool {
	switch ch {
	case 'l':
		opts.printLines = true
	case 'w':
		opts.printWords = true
	case 'c':
		opts.printBytes = true
	case 'm':
		opts.printChars = true
	case 'L':
		opts.printMaxLineLen = true
	default:
		return false
	}
	return true
}

// countFields returns the number of active count fields in opts.
func countFields(opts options) int {
	n := 0
	if opts.printLines {
		n++
	}
	if opts.printWords {
		n++
	}
	if opts.printChars {
		n++
	}
	if opts.printBytes {
		n++
	}
	if opts.printMaxLineLen {
		n++
	}
	return n
}

// processFiles counts and prints results for all files.
// R1.4: prints a total line when more than one file is given.
// R3.1: right-aligns counts in columns. R3.2: total line label is "total".
func processFiles(files []string, opts options, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	width := computeWidth(files)
	// GNU wc uses minimum width when single input with single count field.
	if len(files) <= 1 && countFields(opts) == 1 {
		width = 1
	}
	exitCode := 0
	var total counts

	for _, name := range files {
		c, err := countFile(name, stdin)
		if err != nil {
			w.Flush() // best-effort flush before stderr
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		total = addCounts(total, c)
		printCounts(w, c, width, name, opts)
	}
	if len(files) > 1 {
		printCounts(w, total, width, "total", opts)
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// computeWidth determines the column width for count formatting.
// Uses fstat on files to match GNU wc's pre-processing width calculation.
// When no files can be statted (stdin-only), defaults to 7 matching GNU wc.
func computeWidth(files []string) int {
	maxSize := int64(-1)
	for _, name := range files {
		if name == "" || name == "-" {
			continue
		}
		fi, err := os.Stat(name)
		if err != nil {
			continue
		}
		if fi.Size() > maxSize {
			maxSize = fi.Size()
		}
	}
	if maxSize < 0 {
		return 7 // no statable files; GNU wc default for stdin/pipes
	}
	return numDigits(maxSize)
}

// numDigits returns the number of decimal digits in n.
func numDigits(n int64) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

// countFile opens a file (or reads stdin) and returns its counts.
// R1.2: "" and "-" both read from stdin.
func countFile(name string, stdin io.Reader) (counts, error) {
	if name == "" || name == "-" {
		return countReader(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return counts{}, err
	}
	defer f.Close() // best-effort close on read-only file
	return countReader(f)
}

// countReader reads all data from r and returns line, word, byte, char,
// and max-line-length counts. R1.1: counts newlines, words, bytes.
// R2.4: counts characters as non-continuation UTF-8 bytes.
// R2.5: computes max display width with tab expansion.
func countReader(r io.Reader) (counts, error) {
	var c counts
	buf := make([]byte, 32*1024)
	inWord := false
	var lineWidth int64
	for {
		n, err := r.Read(buf)
		c.bytes += int64(n)
		for _, b := range buf[:n] {
			if b == '\n' {
				c.lines++
				if lineWidth > c.maxLineLen {
					c.maxLineLen = lineWidth
				}
				lineWidth = 0
			} else {
				lineWidth = advanceLineWidth(lineWidth, b)
			}
			if isCharStart(b) {
				c.chars++
			}
			if isSpaceByte(b) {
				inWord = false
			} else if !inWord {
				c.words++
				inWord = true
			}
		}
		if err == io.EOF {
			if lineWidth > c.maxLineLen {
				c.maxLineLen = lineWidth
			}
			return c, nil
		}
		if err != nil {
			return c, err
		}
	}
}

// advanceLineWidth returns the updated display column after processing
// byte b. R2.5: tabs advance to the next multiple of 8. Under LC_ALL=C,
// only ASCII printable characters (0x20–0x7E) have display width 1.
func advanceLineWidth(width int64, b byte) int64 {
	switch {
	case b == '\t':
		return width + int64(8-width%8)
	case b == '\b':
		if width > 0 {
			return width - 1
		}
		return 0
	case isPrintableByte(b):
		return width + 1
	default:
		return width
	}
}

// isPrintableByte returns true for C locale printable characters (0x20–0x7E).
func isPrintableByte(b byte) bool {
	return b >= 0x20 && b <= 0x7E
}

// isCharStart returns true if b is a UTF-8 character start byte.
// Continuation bytes (10xxxxxx) return false. R2.4.
func isCharStart(b byte) bool {
	return b&0xC0 != 0x80
}

// isSpaceByte returns true for C locale whitespace characters.
// Matches isspace() under LC_ALL=C: space, tab, newline, vtab, formfeed, cr.
func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' ||
		b == '\v' || b == '\f' || b == '\r'
}

// printCounts writes a formatted counts line with only selected fields.
// R2.6: output order is lines, words, chars, bytes, max-line-length.
// R1.3: counts followed by filename; no filename for implicit stdin (name="").
func printCounts(w *bufio.Writer, c counts, width int, name string, opts options) {
	first := true
	printField := func(val int64) {
		if !first {
			w.WriteByte(' ')
		}
		fmt.Fprintf(w, "%*d", width, val)
		first = false
	}
	// R2.6: fixed order — lines, words, chars, bytes, max-line-length.
	if opts.printLines {
		printField(c.lines)
	}
	if opts.printWords {
		printField(c.words)
	}
	if opts.printChars {
		printField(c.chars)
	}
	if opts.printBytes {
		printField(c.bytes)
	}
	if opts.printMaxLineLen {
		printField(c.maxLineLen)
	}
	if name != "" {
		fmt.Fprintf(w, " %s", name)
	}
	w.WriteByte('\n')
}

// addCounts returns the element-wise sum of two counts, except maxLineLen
// which takes the maximum (matching GNU wc total behavior for -L).
func addCounts(a, b counts) counts {
	return counts{
		lines:      a.lines + b.lines,
		words:      a.words + b.words,
		bytes:      a.bytes + b.bytes,
		chars:      a.chars + b.chars,
		maxLineLen: max(a.maxLineLen, b.maxLineLen),
	}
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
