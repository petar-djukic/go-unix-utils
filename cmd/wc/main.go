// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd005-wc R1.1–R1.4: default wc behavior with line, word,
// and byte counting from stdin or named files, with totals for multiple files.
// Implements prd005-wc R2.1–R2.6: flag behavior for -l, -w, -c, -m, -L.
// Implements prd005-wc R3.1–R3.3: multi-file column alignment, total line,
// and --total=auto|always|only|never.
// Implements prd005-wc R4.1–R4.4: dash as stdin, binary input, empty input,
// --files0-from.
// Implements prd005-wc R5.1–R5.2: LC_ALL=C locale, -m/-c identity.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

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

// totalMode controls when the total line is printed. R3.3.
type totalMode int

const (
	totalAuto   totalMode = iota // print total when >1 file (default)
	totalAlways                  // always print total
	totalOnly                    // print only the total, not per-file lines
	totalNever                   // never print total
)

// options controls which counts to print. R2.6: output order is fixed as
// lines, words, chars/bytes, max-line-length regardless of flag order.
type options struct {
	printLines      bool
	printWords      bool
	printBytes      bool
	printChars      bool
	printMaxLineLen bool
	total           totalMode
	files0From      string // R4.4: read NUL-delimited filenames from this file
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
// R4.4: when --files0-from is set, reads filenames from the specified source.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files := parseArgs(args)

	if opts.files0From != "" {
		return runFiles0From(opts, files, stdin, stdout, stderr)
	}

	if len(files) == 0 {
		// R1.3: implicit stdin, no filename printed.
		files = []string{""}
	}
	return processFiles(files, opts, computeWidth(files), stdin, stdout, stderr)
}

// runFiles0From handles --files0-from mode. R4.4: reads NUL-delimited
// filenames from the specified file and processes them.
func runFiles0From(opts options, cmdFiles []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(cmdFiles) > 0 {
		fmt.Fprintf(stderr, "%s: extra operand %q\n", progName, cmdFiles[0])
		fmt.Fprintf(stderr, "file operands cannot be combined with --files0-from\n")
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}
	files, err := readFiles0From(opts.files0From, stdin, stderr)
	if err != nil {
		return 1
	}
	if len(files) == 0 {
		return 0
	}
	// R4.4: when --files0-from=-, GNU wc does not pre-stat files for width.
	width := 1
	if opts.files0From != "-" {
		width = computeWidth(files)
	}
	return processFiles(files, opts, width, stdin, stdout, stderr)
}

// readFiles0From reads NUL-delimited filenames from the specified source.
// R4.4: when source is "-", reads from stdin.
func readFiles0From(source string, stdin io.Reader, stderr io.Writer) ([]string, error) {
	var r io.Reader
	if source == "-" {
		r = stdin
	} else {
		f, err := os.Open(source)
		if err != nil {
			fmt.Fprintf(stderr, "%s: cannot open %q for reading: %s\n",
				progName, source, unwrapPathError(err))
			return nil, err
		}
		defer f.Close() // best-effort close on read-only file
		r = f
	}
	data, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return nil, err
	}
	return splitNulFiles(data), nil
}

// splitNulFiles splits NUL-delimited data into filenames.
// A trailing NUL is treated as a delimiter, not an empty filename.
func splitNulFiles(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	parts := bytes.Split(data, []byte{0})
	var files []string
	for i, p := range parts {
		s := string(p)
		// Trailing NUL produces empty last element — skip it.
		if i == len(parts)-1 && s == "" {
			continue
		}
		files = append(files, s)
	}
	return files
}

// parseArgs extracts options and file arguments from the command line.
// Handles "--" as end-of-flags, "-" as explicit stdin, --total=MODE,
// and --files0-from=FILE.
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
		if parseLongOption(&opts, arg) {
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
		defaults := defaultOptions()
		defaults.total = opts.total
		defaults.files0From = opts.files0From
		return defaults, files
	}
	return opts, files
}

// parseLongOption handles GNU-style long options. R3.3: --total=MODE.
// R4.4: --files0-from=FILE. Returns true if the argument was consumed.
func parseLongOption(opts *options, arg string) bool {
	if strings.HasPrefix(arg, "--total=") {
		val := arg[len("--total="):]
		switch val {
		case "auto":
			opts.total = totalAuto
		case "always":
			opts.total = totalAlways
		case "only":
			opts.total = totalOnly
		case "never":
			opts.total = totalNever
		}
		return true
	}
	if strings.HasPrefix(arg, "--files0-from=") {
		opts.files0From = arg[len("--files0-from="):]
		return true
	}
	return false
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
// R3.3: --total mode controls when total line is printed.
func processFiles(files []string, opts options, width int, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	// GNU wc uses minimum width when single input with single count field.
	if len(files) <= 1 && countFields(opts) == 1 {
		width = 1
	}
	// R3.3: --total=only prints a single line; use minimum width.
	if opts.total == totalOnly {
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
		if opts.total != totalOnly {
			printCounts(w, c, width, name, opts)
		}
	}
	if shouldPrintTotal(opts.total, len(files)) {
		label := "total"
		if opts.total == totalOnly {
			label = ""
		}
		printCounts(w, total, width, label, opts)
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// shouldPrintTotal returns true when the total line should be printed.
// R3.3: auto (>1 file), always, only (always), never.
func shouldPrintTotal(mode totalMode, nfiles int) bool {
	switch mode {
	case totalAlways, totalOnly:
		return true
	case totalNever:
		return false
	default: // totalAuto
		return nfiles > 1
	}
}

// computeWidth determines the column width for count formatting.
// Uses fstat on files to match GNU wc's pre-processing width calculation.
// When no files can be statted (stdin-only), defaults to 7 matching GNU wc.
// When stdin is mixed with files, uses at least 7 (GNU wc behavior).
func computeWidth(files []string) int {
	maxSize := int64(-1)
	hasStdin := false
	for _, name := range files {
		if name == "" || name == "-" {
			hasStdin = true
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
	w := numDigits(maxSize)
	if hasStdin && w < 7 {
		return 7
	}
	return w
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
// R5.2: under LC_ALL=C, chars == bytes (each byte is one character).
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
			// R5.2: under LC_ALL=C, each byte is one character.
			c.chars = c.bytes
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
