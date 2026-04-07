// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/wc: count lines, words, and bytes.
// Implements srd005-wc: R1.1 (line count), R1.2 (word count),
// R1.3 (byte count), R1.4 (character count),
// R2.1 (-l flag), R2.2 (-w flag), R2.3 (-c flag), R2.4 (-m flag),
// R2.5 (total line), R2.6 (stdin fallback),
// R3.1 (error handling), R3.2 (exit codes),
// R3.3 (--total mode), R4.1 ("-" as stdin), R4.2 (binary input),
// R4.3 (empty input).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "wc"

// bufSize is the read buffer size for counting operations.
const bufSize = 32 * 1024

// totalMode controls when the "total" summary line is printed.
// R3.3: --total=auto|always|only|never.
type totalMode int

const (
	totalAuto   totalMode = iota // print total when >1 file (default)
	totalAlways                  // print total even for 1 file
	totalOnly                    // print only total, no per-file lines
	totalNever                   // never print total
)

// countResult holds per-file or aggregate counting results.
// R1, R2: fields correspond to wc output columns.
// R4.2: binary input is handled safely by byte-level processing.
// R4.3: zero-value countResult produces correct "0" counts for empty input.
type countResult struct {
	lines         int64 // R2.1: -l newline count
	words         int64 // R2.2: -w word count
	bytes         int64 // R2.3: -c byte count
	chars         int64 // R2.4: -m character count
	maxLineLength int64 // R2.5: -L longest line length
}

// config captures all parsed flag states for wc invocation.
// R2, R4: maps CLI flags to runtime configuration.
type config struct {
	showLines   bool      // -l
	showWords   bool      // -w
	showBytes   bool      // -c
	showChars   bool      // -m
	showMaxLine bool      // -L
	files0From  string    // --files0-from=FILE
	total       totalMode // R3.3: --total=auto|always|only|never
}

// defaultConfig returns true when no selection flags were specified.
// R1.1: default is lines, words, bytes.
func defaultConfig(cfg *config) bool {
	return !cfg.showLines && !cfg.showWords && !cfg.showBytes &&
		!cfg.showChars && !cfg.showMaxLine
}

// applyDefaults sets the default output columns when no flags are given.
// R1.1: default is lines, words, bytes.
func applyDefaults(cfg *config) {
	if defaultConfig(cfg) {
		cfg.showLines = true
		cfg.showWords = true
		cfg.showBytes = true
	}
}

// parseArgs parses command-line arguments and returns a config and file list.
// R2.1-R2.4: maps flag arguments to config struct.
// R2.3/R2.4: -c and -m are mutually exclusive; last flag wins.
// R4.1: "-" is treated as a filename argument meaning stdin.
func parseArgs() (config, []string) {
	var cfg config
	var files []string
	args := os.Args[1:]

	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		// R4.1: "-" is a filename meaning stdin, not a flag.
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		parseFlag(&cfg, arg)
	}
	applyDefaults(&cfg)
	return cfg, files
}

// parseFlag handles a single flag argument, setting config fields.
// R2.3/R2.4: -c and -m are mutually exclusive; last wins.
func parseFlag(cfg *config, arg string) {
	// Handle long flags.
	if strings.HasPrefix(arg, "--") {
		parseLongFlag(cfg, arg)
		return
	}
	// Short flags: iterate over each character after '-'.
	for _, ch := range arg[1:] {
		parseShortFlag(cfg, ch)
	}
}

// parseLongFlag handles --lines, --words, --bytes, --chars, --total flags.
// R3.3: --total=auto|always|only|never.
func parseLongFlag(cfg *config, arg string) {
	// R3.3: handle --total=VALUE.
	if strings.HasPrefix(arg, "--total") {
		parseTotalFlag(cfg, arg)
		return
	}
	switch arg {
	case "--lines":
		cfg.showLines = true
	case "--words":
		cfg.showWords = true
	case "--bytes":
		cfg.showBytes = true
		cfg.showChars = false // R2.3: -c and -m mutually exclusive
	case "--chars":
		cfg.showChars = true
		cfg.showBytes = false // R2.4: -m and -c mutually exclusive
	case "--max-line-length":
		cfg.showMaxLine = true
	}
}

// parseTotalFlag parses --total and --total=VALUE flags.
// R3.3: auto (default), always, only, never.
func parseTotalFlag(cfg *config, arg string) {
	// --total without =VALUE is equivalent to --total=always.
	if arg == "--total" {
		cfg.total = totalAlways
		return
	}
	_, value, found := strings.Cut(arg, "=")
	if !found {
		return
	}
	switch value {
	case "auto":
		cfg.total = totalAuto
	case "always":
		cfg.total = totalAlways
	case "only":
		cfg.total = totalOnly
	case "never":
		cfg.total = totalNever
	}
}

// parseShortFlag handles -l, -w, -c, -m, -L short flags.
func parseShortFlag(cfg *config, ch rune) {
	switch ch {
	case 'l':
		cfg.showLines = true
	case 'w':
		cfg.showWords = true
	case 'c':
		cfg.showBytes = true
		cfg.showChars = false // R2.3: last wins
	case 'm':
		cfg.showChars = true
		cfg.showBytes = false // R2.4: last wins
	case 'L':
		cfg.showMaxLine = true
	}
}

// countReader counts lines, words, bytes, and chars from r in a single pass.
// R1.1: lines are newline characters (\n).
// R1.2: words are maximal sequences of non-whitespace characters.
// R1.3: bytes are total bytes read.
// R1.4: chars are UTF-8 code points; invalid bytes each count as one character.
// R4.2: handles binary input safely via byte-level processing.
// R4.3: returns zero-value countResult for empty input.
func countReader(r io.Reader) (countResult, error) {
	reader := bufio.NewReaderSize(r, bufSize)
	var result countResult
	inWord := false

	for {
		buf, err := reader.Peek(bufSize)
		if len(buf) == 0 && err == io.EOF {
			break
		}
		if len(buf) == 0 && err != nil {
			return result, err
		}
		n := len(buf)
		// Discard what we peeked so the next Peek advances.
		_, _ = reader.Discard(n) // always succeeds after Peek

		result.bytes += int64(n)
		countChunk(&result, buf, &inWord)

		if err == io.EOF {
			break
		}
	}
	return result, nil
}

// countChunk updates result with counts from a single buffer chunk.
// Tracks word boundary state via inWord across chunk boundaries.
func countChunk(result *countResult, buf []byte, inWord *bool) {
	i := 0
	for i < len(buf) {
		b := buf[i]
		if b == '\n' {
			result.lines++
			result.chars++
			if *inWord {
				*inWord = false
			}
			i++
			continue
		}
		if isWSByte(b) {
			if *inWord {
				*inWord = false
			}
			result.chars++
			i++
			continue
		}
		// Non-whitespace byte: may start or continue a word.
		if !*inWord {
			*inWord = true
			result.words++
		}
		// R1.4: count characters as UTF-8 code points.
		if b < utf8.RuneSelf {
			// ASCII: one byte, one character.
			result.chars++
			i++
		} else {
			_, size := utf8.DecodeRune(buf[i:])
			result.chars++
			i += size
		}
	}
}

// isWSByte returns true if b is a C isspace() whitespace character
// (space, tab, newline, carriage return, form feed, vertical tab).
// Newline is handled separately by the caller, so this covers the rest.
func isWSByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\f' || b == '\v'
}

// countFile counts lines, words, bytes, chars, and max line length for a file.
// R1, R2: delegates to countReader for counting logic.
func countFile(name string) (countResult, error) {
	f, err := os.Open(name)
	if err != nil {
		return countResult{}, err
	}
	defer f.Close()
	return countReader(f)
}

// countStdin counts lines, words, bytes, chars, and max line length from stdin.
// R2.6: handles stdin as an unnamed input source.
func countStdin() (countResult, error) {
	return countReader(os.Stdin)
}

// selectedValues returns the values to display based on config flags.
// R2.6: output order is lines, words, chars-or-bytes, max-line-length.
func selectedValues(r countResult, cfg config) []int64 {
	var vals []int64
	if cfg.showLines {
		vals = append(vals, r.lines)
	}
	if cfg.showWords {
		vals = append(vals, r.words)
	}
	if cfg.showChars {
		vals = append(vals, r.chars)
	} else if cfg.showBytes {
		vals = append(vals, r.bytes)
	}
	if cfg.showMaxLine {
		vals = append(vals, r.maxLineLength)
	}
	return vals
}

// numberWidth returns the minimum column width for count fields.
// R3.1: GNU wc uses width 7 for multi-file output, 1 otherwise.
func numberWidth(nfiles int) int {
	if nfiles > 1 {
		return 7
	}
	return 1
}

// formatOutput formats a countResult as a display line.
// R3.1: right-aligns counts and appends the filename.
func formatOutput(r countResult, name string, cfg config, width int) string {
	vals := selectedValues(r, cfg)
	var sb strings.Builder
	for i, v := range vals {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%*d", width, v)
	}
	if name != "" {
		sb.WriteByte(' ')
		sb.WriteString(name)
	}
	return sb.String()
}

// addResult accumulates r into total.
func addResult(total *countResult, r countResult) {
	total.lines += r.lines
	total.words += r.words
	total.bytes += r.bytes
	total.chars += r.chars
	if r.maxLineLength > total.maxLineLength {
		total.maxLineLength = r.maxLineLength
	}
}

// shouldPrintTotal returns whether the total line should be printed.
// R3.3: respects --total mode.
func shouldPrintTotal(mode totalMode, nfiles int) bool {
	switch mode {
	case totalAlways, totalOnly:
		return true
	case totalNever:
		return false
	default:
		// totalAuto: print total when >1 file.
		return nfiles > 1
	}
}

// shouldPrintPerFile returns whether per-file lines should be printed.
// R3.3: --total=only suppresses per-file output.
func shouldPrintPerFile(mode totalMode) bool {
	return mode != totalOnly
}

// processFiles iterates over file arguments and accumulates results.
// R2.6: when no file arguments are given, reads from stdin.
func processFiles(files []string, cfg config) int {
	if len(files) == 0 {
		return processStdin(cfg)
	}
	return processNamedFiles(files, cfg)
}

// processStdin handles the no-arguments case: read from stdin.
func processStdin(cfg config) int {
	r, err := countStdin()
	if err != nil {
		reportError("", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, formatOutput(r, "", cfg, numberWidth(0)))
	return 0
}

// processNamedFiles processes a list of named file arguments.
// R2.5: prints a total line when two or more files are given.
// R3.1: prints errors to stderr and continues processing.
// R3.2: returns exit code 1 if any file produced an error.
// R3.3: respects --total mode for total line control.
func processNamedFiles(files []string, cfg config) int {
	width := numberWidth(len(files))
	exitCode := 0
	var total countResult
	printPerFile := shouldPrintPerFile(cfg.total)

	for _, name := range files {
		r, err := countOneFile(name)
		if err != nil {
			reportError(name, err)
			exitCode = 1
			continue
		}
		addResult(&total, r)
		if printPerFile {
			fmt.Fprintln(os.Stdout, formatOutput(r, name, cfg, width))
		}
	}

	if shouldPrintTotal(cfg.total, len(files)) {
		fmt.Fprintln(os.Stdout, formatOutput(total, "total", cfg, width))
	}
	return exitCode
}

// countOneFile counts a single file, handling "-" as stdin.
// R4.1: "-" means stdin, consistent with POSIX convention.
func countOneFile(name string) (countResult, error) {
	if name == "-" {
		return countStdin()
	}
	return countFile(name)
}

// reportError prints a GNU-compatible diagnostic to stderr.
// R3.1: format is "wc: FILENAME: REASON" using the OS error message.
func reportError(name string, err error) {
	// Unwrap os.PathError to get the underlying OS error message,
	// matching GNU wc format (e.g., "No such file or directory").
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	if name == "" {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, files := parseArgs()
	os.Exit(processFiles(files, cfg))
}
