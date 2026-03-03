// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/wc counts lines, words, bytes, characters, and max line length for one
// or more files or stdin, matching GNU wc output format and behavior.
//
// Implements: prd005-wc R1-R6
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component)
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Constants for labels, program name, and formatting.
const (
	stdinName  = "-"
	totalLabel = "total"
	tabWidth   = 8
	progName   = "wc"
)

// totalMode controls whether and when a "total" line is printed.
// (prd005-wc R3.3)
type totalMode int

const (
	totalAuto   totalMode = iota // default: print total when >1 file
	totalAlways                  // print total even for 1 file
	totalOnly                    // print only the total line
	totalNever                   // never print total
)

// counts holds the computed counts for a single input.
type counts struct {
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
}

// add returns a new counts struct with element-wise sums. For maxLineLen the
// maximum of the two values is used.
func (c counts) add(other counts) counts {
	return counts{
		lines:      c.lines + other.lines,
		words:      c.words + other.words,
		bytes:      c.bytes + other.bytes,
		chars:      c.chars + other.chars,
		maxLineLen: maxInt64(c.maxLineLen, other.maxLineLen),
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// options holds parsed command-line options.
type options struct {
	showLines      bool
	showWords      bool
	showBytes      bool
	showChars      bool
	showMaxLineLen bool
	total          totalMode
	files0From     string // empty means not set
	files          []string
}

// parseArgs parses command-line arguments into options.
// (prd005-wc R1.1, R2.1-R2.6, R3.3, R4.4)
func parseArgs(args []string) (options, error) {
	var opts options
	opts.total = totalAuto
	anyCountFlag := false
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags {
			opts.files = append(opts.files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// Long options.
		switch {
		case arg == "--lines":
			opts.showLines = true
			anyCountFlag = true
			continue
		case arg == "--words":
			opts.showWords = true
			anyCountFlag = true
			continue
		case arg == "--bytes":
			opts.showBytes = true
			anyCountFlag = true
			continue
		case arg == "--chars":
			opts.showChars = true
			anyCountFlag = true
			continue
		case arg == "--max-line-length":
			opts.showMaxLineLen = true
			anyCountFlag = true
			continue
		case strings.HasPrefix(arg, "--total="):
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
			default:
				return opts, fmt.Errorf("%s: invalid argument '%s' for '--total'", progName, val)
			}
			continue
		case strings.HasPrefix(arg, "--files0-from="):
			opts.files0From = arg[len("--files0-from="):]
			continue
		}

		// Short flags (single or combined like -lwc).
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'l':
					opts.showLines = true
					anyCountFlag = true
				case 'w':
					opts.showWords = true
					anyCountFlag = true
				case 'c':
					opts.showBytes = true
					anyCountFlag = true
				case 'm':
					opts.showChars = true
					anyCountFlag = true
				case 'L':
					opts.showMaxLineLen = true
					anyCountFlag = true
				default:
					return opts, fmt.Errorf("%s: invalid option -- '%c'", progName, ch)
				}
			}
			continue
		}

		// Not a flag: treat as file argument.
		opts.files = append(opts.files, arg)
	}

	// Default: show lines, words, bytes when no counting flags given.
	// (prd005-wc R1.1)
	if !anyCountFlag {
		opts.showLines = true
		opts.showWords = true
		opts.showBytes = true
	}

	// -m takes precedence over -c when both are given.
	// (prd005-wc R2.3)
	if opts.showChars && opts.showBytes {
		opts.showBytes = false
	}

	return opts, nil
}

// errWriter wraps an io.Writer and captures the first write error so callers
// can detect stdout failures after formatting. (prd005-wc R6.3)
type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) Write(p []byte) (int, error) {
	if ew.err != nil {
		return 0, ew.err
	}
	n, err := ew.w.Write(p)
	if err != nil {
		ew.err = err
	}
	return n, err
}

// run processes wc arguments and writes output to stdout/stderr. Returns the
// exit code. Separating I/O from os.Exit allows direct testing without
// subprocess spawning. (prd005-wc R6.1, R6.2, R6.3)
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err) // best-effort stderr write
		return 1
	}

	// Collect filenames from --files0-from if specified. (prd005-wc R4.4)
	if opts.files0From != "" {
		names, ferr := readFiles0From(opts.files0From, stdin)
		if ferr != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %s\n", progName, ferr) // best-effort stderr write
			return 1
		}
		opts.files = append(opts.files, names...)
	}

	ew := &errWriter{w: stdout}

	// No files: read stdin. (prd005-wc R1.2)
	if len(opts.files) == 0 {
		c := countReader(stdin)
		formatLine(ew, c, "", opts, 1)
		if ew.err != nil {
			return 1
		}
		return 0
	}

	type result struct {
		c    counts
		name string
	}
	var results []result
	var totalSize int64
	exitCode := 0
	stdinUsed := false

	for _, name := range opts.files {
		if name == stdinName {
			var c counts
			if stdinUsed {
				c = counts{} // second read yields EOF
			} else {
				c = countReader(stdin)
				stdinUsed = true
			}
			results = append(results, result{c, stdinName})
			continue
		}

		f, openErr := os.Open(name)
		if openErr != nil {
			// (prd005-wc R6.2)
			_, _ = fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, sysErrMsg(openErr)) // best-effort stderr write
			exitCode = 1
			continue
		}
		info, statErr := f.Stat()
		if statErr == nil {
			totalSize += info.Size()
		}
		c := countReader(f)
		_ = f.Close() // best-effort close; error not actionable after successful read
		results = append(results, result{c, name})
	}

	showTotal := shouldShowTotal(opts.total, len(opts.files))

	// Compute totals.
	var total counts
	for _, r := range results {
		total = total.add(r.c)
	}

	// Column width matches GNU wc: based on file sizes (total.bytes proxied
	// by the sum of stat sizes for regular files; 0 for pipes/stdin).
	width := digitCount(totalSize)

	// Print per-file lines unless --total=only.
	if opts.total != totalOnly {
		for _, r := range results {
			formatLine(ew, r.c, r.name, opts, width)
		}
	}

	if showTotal {
		formatLine(ew, total, totalLabel, opts, width)
	}

	// Stdout write failure. (prd005-wc R6.3)
	if ew.err != nil {
		exitCode = 1
	}

	return exitCode
}

// shouldShowTotal returns whether a total line should be printed based on the
// total mode and the number of file arguments given. (prd005-wc R3.3)
func shouldShowTotal(mode totalMode, fileCount int) bool {
	switch mode {
	case totalAlways, totalOnly:
		return true
	case totalNever:
		return false
	default: // totalAuto
		return fileCount > 1
	}
}

// countReader reads all data from r and returns the computed counts.
// (prd005-wc R2.1-R2.5, R4.2, R4.3, R5.2)
func countReader(r io.Reader) counts {
	var c counts
	var lineLen int64
	inWord := false

	br := bufio.NewReader(r)
	for {
		b, err := br.ReadByte()
		if err != nil {
			// EOF or read error — finalize max line length for unterminated line.
			if lineLen > c.maxLineLen {
				c.maxLineLen = lineLen
			}
			break
		}

		c.bytes++
		// Under LC_ALL=C each byte is one character. (prd005-wc R5.2)
		c.chars++

		// Word detection: a word is a maximal run of non-whitespace.
		// (prd005-wc R2.2)
		if isSpaceByte(b) {
			inWord = false
		} else if !inWord {
			c.words++
			inWord = true
		}

		// Line counting and max line length. (prd005-wc R2.1, R2.5)
		switch b {
		case '\n':
			c.lines++
			if lineLen > c.maxLineLen {
				c.maxLineLen = lineLen
			}
			lineLen = 0
		case '\t':
			// Tab advances to next multiple of tabWidth. (prd005-wc R2.5)
			lineLen += int64(tabWidth) - (lineLen % int64(tabWidth))
		case '\r':
			lineLen = 0
		case '\b':
			if lineLen > 0 {
				lineLen--
			}
		default:
			// Printable under C locale: 0x20-0x7E.
			if b >= 0x20 && b < 0x7F {
				lineLen++
			}
		}
	}

	return c
}

// isSpaceByte returns true if b is whitespace under the C locale, matching
// the C isspace() function. (prd005-wc R2.2)
func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\v' || b == '\f' || b == '\r'
}

// digitCount returns the number of decimal digits needed to represent n.
// Returns 1 for n <= 0.
func digitCount(n int64) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for ; n > 0; n /= 10 {
		w++
	}
	return w
}

// formatLine writes a single output line to w with the requested counts and
// filename. Counts are printed in the fixed order: lines, words, chars/bytes,
// max-line-length. (prd005-wc R2.6, R1.3)
func formatLine(w io.Writer, c counts, name string, opts options, width int) {
	first := true

	printField := func(val int64) {
		if first {
			_, _ = fmt.Fprintf(w, "%*d", width, val)
			first = false
		} else {
			_, _ = fmt.Fprintf(w, " %*d", width, val)
		}
	}

	if opts.showLines {
		printField(c.lines)
	}
	if opts.showWords {
		printField(c.words)
	}
	if opts.showChars {
		printField(c.chars)
	}
	if opts.showBytes {
		printField(c.bytes)
	}
	if opts.showMaxLineLen {
		printField(c.maxLineLen)
	}

	if name != "" {
		_, _ = fmt.Fprintf(w, " %s", name)
	}
	_, _ = fmt.Fprintln(w)
}

// readFiles0From reads NUL-delimited filenames from the named file (or stdin
// when name is "-"). (prd005-wc R4.4)
func readFiles0From(name string, stdin io.Reader) ([]string, error) {
	var r io.Reader
	if name == stdinName {
		r = stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return nil, fmt.Errorf("cannot open %q for reading: %w", name, err)
		}
		defer func() { _ = f.Close() }() // best-effort close
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var names []string
	for _, field := range strings.Split(string(data), "\x00") {
		if field != "" {
			names = append(names, field)
		}
	}
	return names, nil
}

// sysErrMsg extracts the underlying OS error message from an *os.PathError
// and capitalizes the first letter to match GNU coreutils strerror() format.
func sysErrMsg(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		msg := pathErr.Err.Error()
		if len(msg) > 0 {
			return strings.ToUpper(msg[:1]) + msg[1:]
		}
		return msg
	}
	return err.Error()
}

func main() {
	code := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
