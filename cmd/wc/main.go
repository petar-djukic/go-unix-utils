// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd005-wc R1.1-R1.4, R2.1-R2.6, R3.1-R3.2, R4.1-R4.3, R5.1-R5.2, R6.1-R6.3:
// cmd/wc counts lines, words, bytes, characters, and max line length from files
// or stdin. Default mode prints line, word, and byte counts. Supports -l, -w,
// -c, -m, -L flags and their combinations. Multi-file output includes a totals
// line. Column alignment matches GNU wc dynamic-width behavior.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU wc format.
const progName = "wc"

// wcOptions holds the parsed flags for a wc invocation.
type wcOptions struct {
	lines   bool // -l: count newlines
	words   bool // -w: count words
	bytes   bool // -c: count bytes
	chars   bool // -m: count characters
	maxLine bool // -L: max line length
}

// isDefault returns true when no flags were explicitly set.
func (o *wcOptions) isDefault() bool {
	return !o.lines && !o.words && !o.bytes && !o.chars && !o.maxLine
}

// counts holds the counts for a single input.
type counts struct {
	lines   int64
	words   int64
	bytes   int64
	chars   int64
	maxLine int64
}

// add accumulates c2 into c.
func (c *counts) add(c2 counts) {
	c.lines += c2.lines
	c.words += c2.words
	c.bytes += c2.bytes
	c.chars += c2.chars
	if c2.maxLine > c.maxLine {
		c.maxLine = c2.maxLine
	}
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	// R1.1: default mode prints lines, words, bytes.
	if opts.isDefault() {
		opts.lines = true
		opts.words = true
		opts.bytes = true
	}

	exitCode := 0

	// fileResult holds the counted result for a single input.
	type fileResult struct {
		c    counts
		name string // filename for display; empty for unnamed stdin
		ok   bool   // false if the file could not be opened
	}

	var results []fileResult

	// maxFileSize tracks the largest file size across all inputs, used to
	// determine column width. GNU wc stats each file and uses the max file
	// size to compute the minimum column width. For stdin (not statable),
	// GNU wc defaults to a minimum width of 7.
	maxFileSize := int64(0)
	hasUnstatable := false // true if any input is stdin or non-statable

	if len(files) == 0 {
		// R1.2: no file arguments — read from stdin.
		hasUnstatable = true
		c, err := countReader(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
		results = append(results, fileResult{c: c, name: "", ok: true})
	} else {
		for _, name := range files {
			if name == "-" {
				// R4.1: "-" means read from stdin.
				hasUnstatable = true
				c, err := countReader(os.Stdin)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
					exitCode = 1
				}
				results = append(results, fileResult{c: c, name: "-", ok: true})
			} else {
				f, err := os.Open(name)
				if err != nil {
					// R6.2: print error, set exit 1, continue processing.
					// GNU wc does not print a count line for files it cannot open.
					fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
					exitCode = 1
					results = append(results, fileResult{c: counts{}, name: name, ok: false})
					continue
				}
				// Stat the file to determine size for column width.
				if fi, serr := f.Stat(); serr == nil {
					if sz := fi.Size(); sz > maxFileSize {
						maxFileSize = sz
					}
				} else {
					hasUnstatable = true
				}
				c, err := countReader(f)
				f.Close() // best-effort close
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
					exitCode = 1
				}
				results = append(results, fileResult{c: c, name: name, ok: true})
			}
		}
	}

	// R3.1: compute totals (only from successfully opened files).
	var total counts
	for _, r := range results {
		if r.ok {
			total.add(r.c)
		}
	}

	// R3.1: determine column width. GNU wc uses a minimum of 7 when more
	// than one counter is active, and 1 when exactly one counter is active.
	// For regular files, it increases the width based on file sizes.
	width := computeWidth(maxFileSize, activeCounters(opts), hasUnstatable)

	// Print per-file results (skip files that could not be opened).
	w := bufio.NewWriter(os.Stdout)
	for _, r := range results {
		if !r.ok {
			continue
		}
		if werr := printCounts(w, r.c, r.name, opts, width); werr != nil {
			if isEPIPE(werr) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, werr)
			os.Exit(1)
		}
	}

	// R1.3, R3.2: print total line when more than one file is given.
	if len(files) > 1 {
		if werr := printCounts(w, total, "total", opts, width); werr != nil {
			if isEPIPE(werr) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, werr)
			os.Exit(1)
		}
	}

	if werr := w.Flush(); werr != nil {
		if isEPIPE(werr) {
			os.Exit(0)
		}
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// countReader counts lines, words, bytes, characters, and max line length
// from r. R2.1: lines are newline characters. R2.2: words are maximal
// sequences of non-whitespace (using a simple in-word state machine).
// R2.4: characters count runes, with invalid UTF-8 bytes each counting as
// one character.
func countReader(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReaderSize(r, 64*1024)
	inWord := false
	var lineLen int64

	for {
		buf, err := br.ReadSlice('\n')
		if len(buf) > 0 {
			c.bytes += int64(len(buf))
			// R2.4: count characters (runes). Invalid UTF-8 bytes each count
			// as one character, matching Go's utf8.DecodeRune behavior.
			c.chars += int64(utf8.RuneCount(buf))

			for _, b := range buf {
				if b == '\n' {
					c.lines++
					// R2.5: check line length before resetting.
					if lineLen > c.maxLine {
						c.maxLine = lineLen
					}
					lineLen = 0
				} else if b == '\t' {
					// R2.5: tab advances to next multiple of 8.
					lineLen = (lineLen/8 + 1) * 8
				} else {
					lineLen++
				}

				isSpace := b == ' ' || b == '\t' || b == '\n' || b == '\r' ||
					b == '\f' || b == '\v'
				if isSpace {
					inWord = false
				} else if !inWord {
					inWord = true
					c.words++
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			if err == bufio.ErrBufferFull {
				continue
			}
			return c, err
		}
	}

	// R2.5: check final line length (no trailing newline).
	if lineLen > c.maxLine {
		c.maxLine = lineLen
	}

	return c, nil
}

// multiCounterMinWidth is the minimum column width GNU wc uses when more
// than one counter type is active (e.g., default mode prints lines, words,
// bytes — three counters).
const multiCounterMinWidth = 7

// activeCounters returns the number of counter types that will be printed.
func activeCounters(opts *wcOptions) int {
	n := 0
	if opts.lines {
		n++
	}
	if opts.words {
		n++
	}
	// R2.3: -m takes precedence over -c, but only one is printed.
	if opts.chars || opts.bytes {
		n++
	}
	if opts.maxLine {
		n++
	}
	return n
}

// computeWidth returns the column width for formatting counts. GNU wc
// behavior: when only one counter is active, width is always 1. When
// multiple counters are active, width is the digit count of the max file
// size (from fstat), with a minimum of 7 if any input is stdin/pipe.
func computeWidth(maxFileSize int64, numCounters int, hasUnstatable bool) int {
	if numCounters <= 1 {
		return 1
	}
	minWidth := 1
	if hasUnstatable {
		minWidth = multiCounterMinWidth
	}
	w := digitCount(maxFileSize)
	if w < minWidth {
		return minWidth
	}
	return w
}

// digitCount returns the number of decimal digits in n.
func digitCount(n int64) int {
	if n == 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

// printCounts writes a single line of wc output. R2.6: column order is
// lines, words, chars-or-bytes, max-line-length.
func printCounts(w *bufio.Writer, c counts, name string, opts *wcOptions, width int) error {
	first := true
	writeFn := func(val int64) error {
		if first {
			_, err := fmt.Fprintf(w, "%*d", width, val)
			first = false
			return err
		}
		_, err := fmt.Fprintf(w, " %*d", width, val)
		return err
	}

	if opts.lines {
		if err := writeFn(c.lines); err != nil {
			return err
		}
	}
	if opts.words {
		if err := writeFn(c.words); err != nil {
			return err
		}
	}
	// R2.3: -m takes precedence over -c when both given.
	if opts.chars {
		if err := writeFn(c.chars); err != nil {
			return err
		}
	} else if opts.bytes {
		if err := writeFn(c.bytes); err != nil {
			return err
		}
	}
	if opts.maxLine {
		if err := writeFn(c.maxLine); err != nil {
			return err
		}
	}

	if name != "" {
		if _, err := fmt.Fprintf(w, " %s", name); err != nil {
			return err
		}
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return nil
}

// parseArgs separates flags from file arguments. GNU wc accepts flags
// before the first non-flag argument or after "--".
func parseArgs(args []string) (*wcOptions, []string) {
	opts := &wcOptions{}
	var files []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if flagsDone {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			flagsDone = true
			continue
		}

		if arg == "-" {
			files = append(files, arg)
			continue
		}

		// R4.1: --help prints usage to stdout and exits 0.
		if arg == "--help" {
			fmt.Fprintf(os.Stdout,
				"Usage: %s [OPTION]... [FILE]...\n"+
					"Print newline, word, and byte counts for each FILE, and a total line if\n"+
					"more than one FILE is specified.  A word is a non-zero-length sequence of\n"+
					"printable characters delimited by white space.\n\n"+
					"With no FILE, or when FILE is -, read standard input.\n\n"+
					"  -c, --bytes            print the byte counts\n"+
					"  -m, --chars            print the character counts\n"+
					"  -l, --lines            print the newline counts\n"+
					"  -L, --max-line-length  print the maximum display width\n"+
					"  -w, --words            print the word counts\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName,
			)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n",
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}

		// Long options.
		if arg == "--lines" || arg == "-l" {
			opts.lines = true
			continue
		}
		if arg == "--words" || arg == "-w" {
			opts.words = true
			continue
		}
		if arg == "--bytes" || arg == "-c" {
			opts.bytes = true
			continue
		}
		if arg == "--chars" || arg == "-m" {
			opts.chars = true
			continue
		}
		if arg == "--max-line-length" || arg == "-L" {
			opts.maxLine = true
			continue
		}

		// Short flag groups (e.g., -lwc).
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			for _, ch := range arg[1:] {
				switch ch {
				case 'l':
					opts.lines = true
				case 'w':
					opts.words = true
				case 'c':
					opts.bytes = true
				case 'm':
					opts.chars = true
				case 'L':
					opts.maxLine = true
				default:
					// Unknown flags silently accepted.
				}
			}
			continue
		}

		files = append(files, arg)
	}

	return opts, files
}

// isEPIPE returns true if err wraps a syscall.EPIPE error.
func isEPIPE(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EPIPE
	}
	return false
}

// unwrapPathError extracts the inner error from an *os.PathError.
func unwrapPathError(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
