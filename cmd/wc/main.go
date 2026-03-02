// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the wc command, which counts lines, words, bytes,
// characters, and maximum line length from files or stdin.
//
// Implements: prd005-wc R1, R2, R3, R4, R5, R6
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// counts holds the counting results for a single input.
type counts struct {
	lines   int64
	words   int64
	bytes   int64
	chars   int64
	maxLine int64
}

// run parses flags and processes inputs. Returns 0 on success, 1 on error.
//
// Implements: prd005-wc R1, R2, R3, R4, R6
func run(args []string) int {
	fs := flag.NewFlagSet("wc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		flagLines   = fs.Bool("l", false, "print the newline counts")
		flagWords   = fs.Bool("w", false, "print the word counts")
		flagBytes   = fs.Bool("c", false, "print the byte counts")
		flagChars   = fs.Bool("m", false, "print the character counts")
		flagMaxLine = fs.Bool("L", false, "print the maximum display width")
		totalMode   = fs.String("total", "auto", "when to print a line with total counts")
		files0From  = fs.String("files0-from", "", "read NUL-delimited file names from FILE")
	)

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Determine which counts to display (R1.1, R2.6).
	showLines := *flagLines
	showWords := *flagWords
	showBytes := *flagBytes
	showChars := *flagChars
	showMaxLine := *flagMaxLine

	// Default: lines, words, bytes (R1.1).
	if !showLines && !showWords && !showBytes && !showChars && !showMaxLine {
		showLines = true
		showWords = true
		showBytes = true
	}

	// -m overrides -c display (R2.3).
	if showChars && showBytes {
		showBytes = false
	}

	// Validate --total (R3.3).
	switch *totalMode {
	case "auto", "always", "only", "never":
		// valid
	default:
		fmt.Fprintf(os.Stderr, "wc: invalid argument '%s' for '--total'\n", *totalMode)
		return 1
	}

	// Collect input files.
	files := fs.Args()

	// Handle --files0-from (R4.4).
	if *files0From != "" {
		extra, err := readFiles0From(*files0From)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %v\n", err)
			return 1
		}
		files = append(files, extra...)
	}

	// No files: read from stdin (R1.2).
	stdinMode := len(files) == 0
	if stdinMode {
		files = []string{"-"}
	}

	// Pre-compute field width from file sizes (R3.1).
	fieldWidth := computeFieldWidth(files, stdinMode)

	// Determine total printing mode (R3.3).
	printPerFile := true
	printTotal := false
	switch *totalMode {
	case "auto":
		printTotal = len(files) > 1
	case "always":
		printTotal = true
	case "only":
		printTotal = true
		printPerFile = false
	case "never":
		// never
	}

	// Process each input in order.
	var total counts
	exitCode := 0

	for _, file := range files {
		var c counts
		var err error

		if file == "-" {
			c, err = countInput(os.Stdin)
		} else {
			var f *os.File
			f, err = os.Open(file)
			if err != nil {
				printFileError(file, err)
				exitCode = 1
				continue
			}
			c, err = countInput(f)
			f.Close()
		}

		if err != nil {
			printFileError(file, err)
			exitCode = 1
		}

		// Accumulate totals (R3.2).
		total.lines += c.lines
		total.words += c.words
		total.bytes += c.bytes
		total.chars += c.chars
		if c.maxLine > total.maxLine {
			total.maxLine = c.maxLine
		}

		// Print per-file result.
		if printPerFile {
			name := file
			if file == "-" && stdinMode {
				name = "" // No filename for implicit stdin (R1.3).
			}
			printLine(c, name, fieldWidth, showLines, showWords, showBytes, showChars, showMaxLine)
		}
	}

	// Print total line (R1.4, R3.2).
	if printTotal {
		printLine(total, "total", fieldWidth, showLines, showWords, showBytes, showChars, showMaxLine)
	}

	return exitCode
}

// readFiles0From reads NUL-delimited filenames from path. When path is "-",
// reads from stdin (R4.4).
func readFiles0From(path string) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, p := range strings.Split(string(data), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	return files, nil
}

// computeFieldWidth returns the column width for numeric output. For stdin-only
// input, returns 1 (minimal padding). For file arguments, computes based on the
// total file size across all named files, matching GNU wc's pre-counting width
// calculation.
func computeFieldWidth(files []string, stdinMode bool) int {
	if stdinMode {
		return 1
	}

	var totalSize int64
	for _, file := range files {
		if file == "-" {
			continue
		}
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	w := numWidth(totalSize)
	if w < 1 {
		w = 1
	}
	return w
}

// countInput reads all data from r and counts lines, words, bytes, characters,
// and maximum line length in a single pass (R2.1-R2.5). Handles binary input
// without panicking (R4.2).
func countInput(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReader(r)

	inWord := false
	var lineWidth int64

	for {
		ru, size, err := br.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return c, err
		}

		c.bytes += int64(size)

		// Invalid UTF-8 byte: counts as one character (R2.4).
		if ru == utf8.RuneError && size == 1 {
			c.chars++
			if !inWord {
				inWord = true
				c.words++
			}
			lineWidth++
			continue
		}

		c.chars++

		// Word boundary detection (R2.2): unicode.IsSpace determines whitespace.
		// Line width tracking for -L (R2.5): tab-expanded display columns.
		switch {
		case ru == '\n':
			c.lines++
			if lineWidth > c.maxLine {
				c.maxLine = lineWidth
			}
			lineWidth = 0
			inWord = false

		case ru == '\t':
			// Tab advances to next multiple of 8 (R2.5).
			lineWidth = (lineWidth/8 + 1) * 8
			inWord = false

		case unicode.IsSpace(ru):
			inWord = false
			lineWidth++

		default:
			if !inWord {
				inWord = true
				c.words++
			}
			lineWidth++
		}
	}

	// Handle final line without trailing newline (R2.5).
	if lineWidth > c.maxLine {
		c.maxLine = lineWidth
	}

	return c, nil
}

// printLine writes a single output line with counts and optional filename.
// Count order: lines, words, chars/bytes, max-line-length (R2.6).
func printLine(c counts, name string, width int, showLines, showWords, showBytes, showChars, showMaxLine bool) {
	first := true
	field := func(val int64) {
		if first {
			fmt.Printf("%*d", width, val)
			first = false
		} else {
			fmt.Printf(" %*d", width, val)
		}
	}

	if showLines {
		field(c.lines)
	}
	if showWords {
		field(c.words)
	}
	if showChars {
		field(c.chars)
	} else if showBytes {
		field(c.bytes)
	}
	if showMaxLine {
		field(c.maxLine)
	}

	if name != "" {
		fmt.Printf(" %s", name)
	}
	fmt.Println()
}

// printFileError writes a file access error to stderr (R6.2).
func printFileError(file string, err error) {
	if pe, ok := err.(*os.PathError); ok {
		fmt.Fprintf(os.Stderr, "wc: %s: %s\n", pe.Path, pe.Err)
	} else {
		fmt.Fprintf(os.Stderr, "wc: %s: %v\n", file, err)
	}
}

// numWidth returns the number of decimal digits needed to represent n.
func numWidth(n int64) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for n > 0 {
		w++
		n /= 10
	}
	return w
}
