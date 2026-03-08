// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the wc utility for counting lines, words, and bytes.
//
// Implements prd005-wc: default behavior (R1), flag behavior (R2),
// multi-file output and column alignment (R3), stdin and special inputs (R4),
// locale handling (R5), exit codes (R6).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// totalMode controls when the total line is printed. R3.3.
type totalMode int

const (
	totalAuto   totalMode = iota // print total when >1 file
	totalAlways                  // always print total
	totalOnly                    // print only the total line
	totalNever                   // never print total
)

// defaultMinWidth is the minimum column width GNU wc uses when input
// includes stdin or non-regular files (cannot be stat'd for size).
const defaultMinWidth = 7

// counts holds the accumulated counts for a single input.
type counts struct {
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
}

// config holds the parsed command-line options.
type config struct {
	printLines  bool
	printWords  bool
	printBytes  bool
	printChars  bool
	printMaxLen bool
	total       totalMode
	files       []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg := parseArgs(os.Args[1:])
	exitCode := 0

	// R1.1: Default to lines, words, bytes when no flags given.
	if !cfg.printLines && !cfg.printWords && !cfg.printBytes && !cfg.printChars && !cfg.printMaxLen {
		cfg.printLines = true
		cfg.printWords = true
		cfg.printBytes = true
	}

	var results []counts
	var names []string
	hasStdin := false
	numFileArgs := len(cfg.files)

	if numFileArgs == 0 {
		// R1.2: Read from stdin when no file arguments.
		hasStdin = true
		c, err := countReader(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: read error: %v\n", err)
			exitCode = 1
		}
		results = append(results, c)
		names = append(names, "")
	} else {
		for _, name := range cfg.files {
			var r io.Reader
			if name == "-" {
				// R4.1: "-" means stdin.
				hasStdin = true
				r = os.Stdin
			} else {
				f, err := os.Open(name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "wc: %s: No such file or directory\n", name)
					exitCode = 1
					continue
				}
				r = f
				defer f.Close()
			}
			c, err := countReader(r)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wc: %s: read error: %v\n", name, err)
				exitCode = 1
			}
			results = append(results, c)
			names = append(names, name)
		}
	}

	// Compute totals.
	var totalCounts counts
	for _, c := range results {
		totalCounts.lines += c.lines
		totalCounts.words += c.words
		totalCounts.bytes += c.bytes
		totalCounts.chars += c.chars
		if c.maxLineLen > totalCounts.maxLineLen {
			totalCounts.maxLineLen = c.maxLineLen
		}
	}

	// Determine if total line should be printed.
	// Use original file arg count for auto mode.
	showTotal := false
	switch cfg.total {
	case totalAuto:
		showTotal = numFileArgs > 1
	case totalAlways:
		showTotal = true
	case totalOnly:
		showTotal = true
	case totalNever:
		showTotal = false
	}

	// Compute column width from all values including total.
	width := columnWidth(results, totalCounts, showTotal, cfg)

	// GNU wc uses a default min width of 7 for stdin when multiple
	// count columns are active. Single-column output uses minimal width.
	activeColumns := countActiveColumns(cfg)
	if hasStdin && activeColumns > 1 && width < defaultMinWidth {
		width = defaultMinWidth
	}

	// Print per-file lines (unless --total=only).
	if cfg.total != totalOnly {
		for i, c := range results {
			printLine(c, names[i], width, cfg)
		}
	}

	// Print total line.
	if showTotal {
		totalLabel := "total"
		if cfg.total == totalOnly {
			totalLabel = "" // R3.3: --total=only omits the "total" label.
		}
		printLine(totalCounts, totalLabel, width, cfg)
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into config.
func parseArgs(args []string) config {
	var cfg config
	cfg.total = totalAuto

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			cfg.files = append(cfg.files, args[i:]...)
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--total=") {
			val := arg[len("--total="):]
			switch val {
			case "auto":
				cfg.total = totalAuto
			case "always":
				cfg.total = totalAlways
			case "only":
				cfg.total = totalOnly
			case "never":
				cfg.total = totalNever
			default:
				fmt.Fprintf(os.Stderr, "wc: invalid argument '%s' for '--total'\n", val)
				os.Exit(1)
			}
			i++
			continue
		}

		if strings.HasPrefix(arg, "--") {
			// Skip other long options for forward compatibility.
			i++
			continue
		}

		// Short flags.
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			for _, ch := range arg[1:] {
				switch ch {
				case 'l':
					cfg.printLines = true
				case 'w':
					cfg.printWords = true
				case 'c':
					cfg.printBytes = true
				case 'm':
					cfg.printChars = true
				case 'L':
					cfg.printMaxLen = true
				default:
					fmt.Fprintf(os.Stderr, "wc: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			i++
			continue
		}

		// File argument.
		cfg.files = append(cfg.files, arg)
		i++
	}

	return cfg
}

// countReader reads all data from r and returns the computed counts.
// R4.2: Handles binary input without corruption.
func countReader(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReader(r)

	var currentLineLen int64
	inWord := false

	buf := make([]byte, 32*1024)
	for {
		n, err := br.Read(buf)
		data := buf[:n]

		for i := 0; i < len(data); {
			b := data[i]

			// Decode rune for character counting and word detection.
			var r rune
			var size int
			if b < utf8.RuneSelf {
				r = rune(b)
				size = 1
			} else {
				remaining := data[i:]
				r, size = utf8.DecodeRune(remaining)
				if r == utf8.RuneError && size <= 1 {
					// Invalid byte; count as one character. R2.4.
					size = 1
				}
			}

			c.bytes += int64(size)
			c.chars++

			// R2.1: Count newlines.
			if b == '\n' {
				c.lines++
				if currentLineLen > c.maxLineLen {
					c.maxLineLen = currentLineLen
				}
				currentLineLen = 0
				inWord = false
			} else {
				// R2.5: Tab expansion for -L.
				if b == '\t' {
					currentLineLen = (currentLineLen/8 + 1) * 8
				} else if size == 1 {
					currentLineLen++
				} else {
					// Multi-byte rune: count as 1 display column.
					currentLineLen++
				}

				// R2.2: Word counting.
				if unicode.IsSpace(r) {
					inWord = false
				} else {
					if !inWord {
						c.words++
						inWord = true
					}
				}
			}

			i += size
		}

		if err != nil {
			if err != io.EOF {
				if currentLineLen > c.maxLineLen {
					c.maxLineLen = currentLineLen
				}
				return c, err
			}
			break
		}
	}

	// Check final line length (line without trailing newline).
	if currentLineLen > c.maxLineLen {
		c.maxLineLen = currentLineLen
	}

	return c, nil
}

// countActiveColumns returns the number of count columns that will be printed.
func countActiveColumns(cfg config) int {
	n := 0
	if cfg.printLines {
		n++
	}
	if cfg.printWords {
		n++
	}
	if cfg.printChars {
		n++
	}
	if cfg.printBytes {
		n++
	}
	if cfg.printMaxLen {
		n++
	}
	return n
}

// columnWidth computes the minimum field width for printing counts.
// R3.1: Right-aligned columns wide enough for the largest count.
func columnWidth(results []counts, total counts, showTotal bool, cfg config) int {
	maxVal := int64(0)

	for _, c := range results {
		updateMax(&maxVal, c, cfg)
	}
	if showTotal {
		updateMax(&maxVal, total, cfg)
	}

	// Compute digit count.
	w := 1
	v := maxVal
	for v >= 10 {
		w++
		v /= 10
	}
	return w
}

// updateMax updates maxVal with the largest count from c based on active flags.
func updateMax(maxVal *int64, c counts, cfg config) {
	if cfg.printLines && c.lines > *maxVal {
		*maxVal = c.lines
	}
	if cfg.printWords && c.words > *maxVal {
		*maxVal = c.words
	}
	if cfg.printBytes && c.bytes > *maxVal {
		*maxVal = c.bytes
	}
	if cfg.printChars && c.chars > *maxVal {
		*maxVal = c.chars
	}
	if cfg.printMaxLen && c.maxLineLen > *maxVal {
		*maxVal = c.maxLineLen
	}
}

// printLine prints a single output line with right-aligned counts.
// R2.6: Fixed column order: lines, words, chars, bytes, max-line-length.
func printLine(c counts, name string, width int, cfg config) {
	fields := make([]string, 0, 5)

	if cfg.printLines {
		fields = append(fields, fmt.Sprintf("%*d", width, c.lines))
	}
	if cfg.printWords {
		fields = append(fields, fmt.Sprintf("%*d", width, c.words))
	}
	if cfg.printChars {
		fields = append(fields, fmt.Sprintf("%*d", width, c.chars))
	}
	if cfg.printBytes {
		fields = append(fields, fmt.Sprintf("%*d", width, c.bytes))
	}
	if cfg.printMaxLen {
		fields = append(fields, fmt.Sprintf("%*d", width, c.maxLineLen))
	}

	line := strings.Join(fields, " ")
	if name != "" {
		line += " " + name
	}
	fmt.Fprintln(os.Stdout, line)
}
