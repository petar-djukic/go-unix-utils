// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cmd/wc binary.
// Counts lines, words, bytes, characters, and maximum line length for
// one or more files or stdin, with column-aligned output matching GNU wc.
//
// Implements: prd005-wc R1-R6
// Architecture: docs/ARCHITECTURE.yaml § cmd/
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// progName is the program name used in error diagnostics (R6.2).
const progName = "wc"

// totalMode controls when the totals line is printed (R3.3).
type totalMode int

const (
	totalAuto   totalMode = iota // print total when more than one file (default)
	totalAlways                  // always print total, even for one file
	totalOnly                    // print only the total line, not per-file lines
	totalNever                   // never print total
)

// counts holds the accumulated counts for a single input source.
type counts struct {
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
}

// add returns the element-wise sum of c and other.
func (c counts) add(other counts) counts {
	c.lines += other.lines
	c.words += other.words
	c.bytes += other.bytes
	c.chars += other.chars
	if other.maxLineLen > c.maxLineLen {
		c.maxLineLen = other.maxLineLen
	}
	return c
}

// wcConfig holds the resolved flag settings for a single wc invocation.
type wcConfig struct {
	printLines   bool
	printWords   bool
	printBytes   bool
	printChars   bool
	printMaxLen  bool
	totalMode    totalMode
	files0From   string // --files0-from=FILE; empty means not used
}

func main() {
	// R6.3 / R5.1 equivalent: SIGPIPE handler so piping to head exits cleanly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGPIPE)
	go func() {
		<-sigCh
		os.Exit(0)
	}()

	cfg, files := parseFlags()

	exitCode := 0
	var results []fileResult

	for _, name := range files {
		c, err := countInput(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, sysErr(err))
			exitCode = 1
			// R6.2: do not print a count line for failed files, but continue
			// processing remaining files.
			continue
		}
		results = append(results, fileResult{name: name, counts: c})
	}
	numArgs := len(files)

	// Compute totals.
	var total counts
	for _, r := range results {
		total = total.add(r.counts)
	}

	// Determine column width from the largest count that will be printed.
	width := numberWidth(total, cfg)

	// Print per-file lines unless --total=only.
	if cfg.totalMode != totalOnly {
		for _, r := range results {
			printCounts(r.counts, r.name, width, cfg)
		}
	}

	// Print total line per R3.3.
	showTotal := false
	switch cfg.totalMode {
	case totalAuto:
		showTotal = numArgs > 1
	case totalAlways:
		showTotal = true
	case totalOnly:
		showTotal = true
	case totalNever:
		showTotal = false
	}
	if showTotal {
		printCounts(total, "total", width, cfg)
	}

	os.Exit(exitCode)
}

// fileResult pairs a filename with its counts.
type fileResult struct {
	name   string
	counts counts
}

// parseFlags parses command-line arguments manually to support GNU-style long
// flags (--total=VALUE, --files0-from=FILE) alongside single-character flags.
// Go's flag package does not support --key=value syntax natively, so we parse
// by hand to match GNU wc flag behavior.
func parseFlags() (wcConfig, []string) {
	cfg := wcConfig{}
	var files []string
	explicitFlags := false

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// "--" terminates flag parsing.
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		// Long flags.
		if strings.HasPrefix(arg, "--") {
			if strings.HasPrefix(arg, "--total=") {
				val := arg[len("--total="):]
				switch val {
				case "auto":
					cfg.totalMode = totalAuto
				case "always":
					cfg.totalMode = totalAlways
				case "only":
					cfg.totalMode = totalOnly
				case "never":
					cfg.totalMode = totalNever
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid argument '%s' for '--total'\n", progName, val)
					os.Exit(1)
				}
				continue
			}
			if strings.HasPrefix(arg, "--files0-from=") {
				cfg.files0From = arg[len("--files0-from="):]
				continue
			}
			// Long flag aliases.
			switch arg {
			case "--lines":
				cfg.printLines = true
				explicitFlags = true
			case "--words":
				cfg.printWords = true
				explicitFlags = true
			case "--bytes":
				cfg.printBytes = true
				explicitFlags = true
			case "--chars":
				cfg.printChars = true
				explicitFlags = true
			case "--max-line-length":
				cfg.printMaxLen = true
				explicitFlags = true
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
				os.Exit(1)
			}
			continue
		}

		// Short flags (may be combined: -lwc).
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'l':
					cfg.printLines = true
					explicitFlags = true
				case 'w':
					cfg.printWords = true
					explicitFlags = true
				case 'c':
					cfg.printBytes = true
					explicitFlags = true
				case 'm':
					cfg.printChars = true
					explicitFlags = true
				case 'L':
					cfg.printMaxLen = true
					explicitFlags = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, ch)
					os.Exit(1)
				}
			}
			continue
		}

		// Not a flag — it's a filename.
		files = append(files, arg)
	}

	// R1.1: when no flags are given, default to lines + words + bytes.
	if !explicitFlags {
		cfg.printLines = true
		cfg.printWords = true
		cfg.printBytes = true
	}

	// R2.3: -m takes precedence over -c when both are given.
	if cfg.printChars && cfg.printBytes {
		cfg.printBytes = false
	}

	// Handle --files0-from (R4.4).
	if cfg.files0From != "" {
		f0files, err := readFiles0From(cfg.files0From)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			os.Exit(1)
		}
		files = append(files, f0files...)
	}

	// R1.2: when no file arguments, read from stdin.
	// Empty string signals implicit stdin (no filename in output).
	// Explicit "-" argument uses "-" as the display label.
	if len(files) == 0 {
		files = []string{""}
	}

	return cfg, files
}

// readFiles0From reads NUL-delimited filenames from the given path.
// When path is "-", filenames are read from stdin (R4.4).
func readFiles0From(path string) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("cannot open '%s' for reading: %w", path, sysErr(err))
		}
		defer func() { _ = f.Close() }() // best-effort cleanup
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading files0-from: %w", err)
	}

	var files []string
	for _, name := range strings.Split(string(data), "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}

// countInput reads an input source and returns its counts.
// When name is "-" or "", stdin is read (R4.1).
func countInput(name string) (counts, error) {
	var r io.Reader
	if name == "-" || name == "" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return counts{}, err
		}
		defer func() { _ = f.Close() }() // best-effort cleanup
		r = f
	}
	return countReader(r)
}

// countReader counts lines, words, bytes, characters, and max line length
// from r in a single pass. A word is a maximal sequence of non-whitespace
// characters (R2.2). Line length is measured with tab expansion to the next
// multiple of 8 (R2.5).
func countReader(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReader(r)
	inWord := false
	var lineLen int64

	for {
		buf := make([]byte, 32*1024)
		n, err := br.Read(buf)
		buf = buf[:n]
		c.bytes += int64(n)

		i := 0
		for i < len(buf) {
			ru, size := utf8.DecodeRune(buf[i:])
			if ru == utf8.RuneError && size == 1 {
				// R2.4: invalid byte counts as one character.
				c.chars++
				if inWord {
					// Invalid byte is not whitespace; stay in word.
				} else {
					inWord = true
					c.words++
				}
				lineLen++
				i++
				continue
			}

			c.chars++

			if ru == '\n' {
				c.lines++
				if inWord {
					inWord = false
				}
				// R2.5: end of line — check max.
				if lineLen > c.maxLineLen {
					c.maxLineLen = lineLen
				}
				lineLen = 0
			} else if ru == '\t' {
				// R2.5: tab advances to next multiple of 8.
				lineLen = (lineLen/8 + 1) * 8
				if inWord {
					inWord = false
				}
			} else if unicode.IsSpace(ru) {
				if inWord {
					inWord = false
				}
				lineLen++
			} else {
				if !inWord {
					inWord = true
					c.words++
				}
				lineLen++
			}

			i += size
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return c, err
		}
	}

	// R2.5: handle last line if it didn't end with newline.
	if lineLen > c.maxLineLen {
		c.maxLineLen = lineLen
	}

	return c, nil
}

// numberWidth returns the minimum field width for printing counts.
// It is the number of decimal digits in the largest count that will be printed,
// matching GNU wc's dynamic column-width behavior (R3.1).
func numberWidth(total counts, cfg wcConfig) int {
	maxVal := int64(0)
	if cfg.printLines && total.lines > maxVal {
		maxVal = total.lines
	}
	if cfg.printWords && total.words > maxVal {
		maxVal = total.words
	}
	if cfg.printBytes && total.bytes > maxVal {
		maxVal = total.bytes
	}
	if cfg.printChars && total.chars > maxVal {
		maxVal = total.chars
	}
	if cfg.printMaxLen && total.maxLineLen > maxVal {
		maxVal = total.maxLineLen
	}
	w := 1
	for maxVal >= 10 {
		maxVal /= 10
		w++
	}
	return w
}

// printCounts writes one output line with the requested counts and filename.
// Counts are right-justified to width columns (R3.1). The filename is omitted
// when name is "-" and it's the only input (stdin with no filename) (R1.3).
func printCounts(c counts, name string, width int, cfg wcConfig) {
	var parts []string

	if cfg.printLines {
		parts = append(parts, fmt.Sprintf("%*d", width, c.lines))
	}
	if cfg.printWords {
		parts = append(parts, fmt.Sprintf("%*d", width, c.words))
	}
	if cfg.printChars {
		parts = append(parts, fmt.Sprintf("%*d", width, c.chars))
	}
	if cfg.printBytes {
		parts = append(parts, fmt.Sprintf("%*d", width, c.bytes))
	}
	if cfg.printMaxLen {
		parts = append(parts, fmt.Sprintf("%*d", width, c.maxLineLen))
	}

	line := strings.Join(parts, " ")

	// R1.3: append filename. For stdin with no file arguments,
	// the filename is omitted. For explicit "-" argument, the label is "-".
	if name == "" {
		fmt.Println(line)
	} else {
		fmt.Printf("%s %s\n", line, name)
	}
}

// sysErr extracts the underlying system error from an os.PathError for
// cleaner diagnostic messages matching GNU wc format.
func sysErr(err error) error {
	if pe, ok := err.(*os.PathError); ok { //nolint:errorlint
		return pe.Err
	}
	return err
}
