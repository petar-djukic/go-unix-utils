// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU wc: print newline, word, and byte counts.
// Implements prd005-wc R1-R6.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// counts holds the computed statistics for a single input.
type counts struct {
	lines   int64
	words   int64
	bytes   int64
	chars   int64
	maxLine int64
}

// add accumulates another counts into this one. For maxLine, takes the max.
func (c *counts) add(other counts) {
	c.lines += other.lines
	c.words += other.words
	c.bytes += other.bytes
	c.chars += other.chars
	if other.maxLine > c.maxLine {
		c.maxLine = other.maxLine
	}
}

// options holds the parsed command-line flags.
type options struct {
	showLines   bool
	showWords   bool
	showBytes   bool
	showChars   bool
	showMaxLine bool
}

func main() {
	// R4, D2: Install SIGPIPE handler per ARCHITECTURE.yaml shared_protocols.
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	// R1.1: Default to lines, words, bytes when no flags given.
	if !opts.showLines && !opts.showWords && !opts.showBytes && !opts.showChars && !opts.showMaxLine {
		opts.showLines = true
		opts.showWords = true
		opts.showBytes = true
	}

	explicitFiles := len(files) > 0
	if !explicitFiles {
		files = []string{"-"}
	}

	// R3.1: Pre-compute column width from file sizes, matching GNU wc.
	numberWidth := computeNumberWidth(files)

	exitCode := 0
	var total counts
	type result struct {
		c  counts
		ok bool
	}
	results := make([]result, len(files))

	for i, file := range files {
		if file == "-" {
			c := countInput(os.Stdin)
			total.add(c)
			results[i] = result{c: c, ok: true}
		} else {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wc: %s: No such file or directory\n", file)
				exitCode = 1
				continue
			}
			c := countInput(f)
			f.Close() // best-effort close
			total.add(c)
			results[i] = result{c: c, ok: true}
		}
	}

	// Print results for each successfully processed file.
	for i, file := range files {
		if !results[i].ok {
			continue
		}
		label := file
		if file == "-" && !explicitFiles {
			label = ""
		}
		printLine(results[i].c, label, opts, numberWidth)
	}

	// R1.4: Print total when more than one file argument given.
	if len(files) > 1 {
		printLine(total, "total", opts, numberWidth)
	}

	os.Exit(exitCode)
}

// parseArgs parses wc flags from args, supporting combined short flags and --.
// R2.3: -c and -m are mutually exclusive; last one wins.
func parseArgs(args []string) (*options, []string) {
	opts := &options{}
	var files []string
	endFlags := false

	for _, arg := range args {
		if endFlags || arg == "" || (len(arg) > 0 && arg[0] != '-') {
			files = append(files, arg)
			continue
		}

		if arg == "-" {
			files = append(files, "-")
			continue
		}

		if arg == "--" {
			endFlags = true
			continue
		}

		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}

		// Combined short flags like -lwc.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'l':
					opts.showLines = true
				case 'w':
					opts.showWords = true
				case 'c':
					// R2.3: -c/-m last wins.
					opts.showBytes = true
					opts.showChars = false
				case 'm':
					// R2.3: -c/-m last wins.
					opts.showChars = true
					opts.showBytes = false
				case 'L':
					opts.showMaxLine = true
				default:
					fmt.Fprintf(os.Stderr, "wc: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}

		fmt.Fprintf(os.Stderr, "wc: unrecognized option '%s'\n", arg)
		os.Exit(1)
	}

	return opts, files
}

// isSpace returns true if b is a C locale whitespace character.
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v'
}

// countInput counts lines, words, bytes, chars, and max line length from r.
// R1, R2: Single-pass counting. Under LC_ALL=C, chars == bytes (R5.2).
func countInput(r io.Reader) counts {
	var c counts
	inWord := false
	var lineLen int64

	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		for i := 0; i < n; i++ {
			b := buf[i]
			c.bytes++
			c.chars++ // R5.2: Under LC_ALL=C, chars == bytes.

			if isSpace(b) {
				inWord = false
				switch b {
				case '\n':
					c.lines++
					if lineLen > c.maxLine {
						c.maxLine = lineLen
					}
					lineLen = 0
				case '\r', '\f':
					// R2.5: \r and \f reset line position (matching GNU wc).
					if lineLen > c.maxLine {
						c.maxLine = lineLen
					}
					lineLen = 0
				case '\t':
					// R2.5: Tab expands to next multiple of 8.
					lineLen += 8 - lineLen%8
				case ' ':
					lineLen++
				case '\v':
					// Vertical tab stays in same column (matching GNU wc).
				}
			} else {
				if !inWord {
					c.words++
					inWord = true
				}
				lineLen++
			}
		}
		if err != nil {
			break
		}
	}

	// Handle last line without trailing newline for -L.
	if lineLen > c.maxLine {
		c.maxLine = lineLen
	}

	return c
}

// computeNumberWidth determines the column width for output formatting.
// Matches GNU wc: width = digits_in(sum_of_file_sizes), minimum 7.
func computeNumberWidth(files []string) int {
	var totalSize int64
	for _, f := range files {
		if f == "-" {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}
	w := 1
	for s := totalSize; s >= 10; s /= 10 {
		w++
	}
	if w < 7 {
		w = 7
	}
	return w
}

// printLine prints a single line of wc output with right-aligned columns.
// R2.6: Fixed output order: lines, words, chars/bytes, max-line-length.
func printLine(c counts, label string, opts *options, width int) {
	first := true
	printField := func(n int64) {
		if !first {
			fmt.Print(" ")
		}
		fmt.Printf("%*d", width, n)
		first = false
	}

	if opts.showLines {
		printField(c.lines)
	}
	if opts.showWords {
		printField(c.words)
	}
	if opts.showChars {
		printField(c.chars)
	} else if opts.showBytes {
		printField(c.bytes)
	}
	if opts.showMaxLine {
		printField(c.maxLine)
	}

	if label != "" {
		fmt.Printf(" %s", label)
	}
	fmt.Println()
}

// printHelp prints usage information to stdout. R4.
func printHelp() {
	fmt.Println("Usage: wc [OPTION]... [FILE]...")
	fmt.Println("Print newline, word, and byte counts for each FILE, and a total line if")
	fmt.Println("more than one FILE is specified.  A word is a non-zero-length sequence of")
	fmt.Println("printable characters delimited by white space.")
	fmt.Println()
	fmt.Println("With no FILE, or when FILE is -, read standard input.")
	fmt.Println()
	fmt.Println("The options below may be used to select which counts are printed, always in")
	fmt.Println("the following order: newline, word, character, byte, maximum line length.")
	fmt.Println("  -c, --bytes            print the byte counts")
	fmt.Println("  -m, --chars            print the character counts")
	fmt.Println("  -l, --lines            print the newline counts")
	fmt.Println("  -L, --max-line-length  print the maximum display width")
	fmt.Println("  -w, --words            print the word counts")
	fmt.Println("      --help        display this help and exit")
	fmt.Println("      --version     output version information and exit")
}

// printVersion prints version information to stdout. R4.
func printVersion() {
	fmt.Println("wc (go-unix-utils) dev")
}
