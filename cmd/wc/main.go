// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the wc utility: count lines, words, and bytes.
//
// Implements: prd005-wc (R1-R6)
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// totalMode controls when the "total" summary line is printed.
// Per prd005-wc R3.3.
type totalMode int

const (
	totalAuto   totalMode = iota // Print total when more than one file.
	totalAlways                  // Print total even for one file.
	totalOnly                    // Print only the total, no per-file lines.
	totalNever                   // Never print total.
)

// options holds the parsed command-line flags for wc.
type options struct {
	printLines     bool   // -l: count newline characters.
	printWords     bool   // -w: count words.
	printBytes     bool   // -c: count bytes.
	printChars     bool   // -m: count characters (runes).
	printMaxLine   bool   // -L: print length of longest line.
	total          totalMode
	files0From     string // --files0-from=FILE
	hasFiles0From  bool
	defaultMode    bool // True when no counting flags were given.
}

// counts holds the counting results for a single input.
type counts struct {
	lines      int64
	words      int64
	bytesCount int64
	chars      int64
	maxLine    int64
}

func main() {
	// SIGPIPE handling: exit 0 silently on broken pipe.
	// Per prd005-wc R6.3 and design decision D1, matching cmd/cat pattern.
	sigpipeCh := make(chan os.Signal, 1)
	signal.Notify(sigpipeCh, syscall.SIGPIPE)
	go func() {
		<-sigpipeCh
		os.Exit(0)
	}()

	opts, files, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "wc: %s\n", err)
		os.Exit(1)
	}

	// Per prd005-wc R4.4: --files0-from reads NUL-delimited filenames.
	if opts.hasFiles0From {
		extraFiles, err := readFiles0From(opts.files0From)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %s\n", err)
			os.Exit(1)
		}
		files = append(files, extraFiles...)
	}

	// Per prd005-wc R1.2: read from stdin when no file arguments are given.
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	writer := bufio.NewWriter(os.Stdout)

	type fileResult struct {
		label string
		c     counts
		ok    bool
	}

	results := make([]fileResult, len(files))
	var totalCounts counts
	successCount := 0

	for i, file := range files {
		// Per prd005-wc R1.3: for stdin input, no filename is printed.
		// GNU wc omits the filename label for stdin regardless of whether
		// it was implied (no args) or explicit ("-" argument).
		label := file
		if file == "-" {
			label = ""
		}
		c, err := countFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %s\n", err)
			exitCode = 1
			results[i] = fileResult{label: label, ok: false}
			continue
		}
		results[i] = fileResult{label: label, c: c, ok: true}
		successCount++
		totalCounts.lines += c.lines
		totalCounts.words += c.words
		totalCounts.bytesCount += c.bytesCount
		totalCounts.chars += c.chars
		if c.maxLine > totalCounts.maxLine {
			totalCounts.maxLine = c.maxLine
		}
	}

	// Determine if we need a total line, to include it in column width calculation.
	// Per prd005-wc R3.3: total considers total file count, not success count.
	showTotal := shouldShowTotal(opts.total, len(files))

	// Compute column width from the maximum count value across all entries.
	// Per design decision D5.
	successCounts := make([]counts, 0, successCount)
	for _, r := range results {
		if r.ok {
			successCounts = append(successCounts, r.c)
		}
	}
	colWidth := computeColumnWidth(successCounts, totalCounts, showTotal, &opts)

	// Print per-file lines unless --total=only.
	// Per prd005-wc R6.2: print counts only for successfully read files.
	if opts.total != totalOnly {
		for _, r := range results {
			if !r.ok {
				continue
			}
			printCounts(writer, r.c, r.label, colWidth, &opts)
		}
	}

	// Print total line if needed. Per prd005-wc R1.4, R3.2.
	if showTotal {
		printCounts(writer, totalCounts, "total", colWidth, &opts)
	}

	if err := writer.Flush(); err != nil {
		// Per prd005-wc R6.3: exit 1 on stdout write error.
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// parseFlags parses wc command-line arguments using manual flag parsing to
// support combined short flags (e.g., -lwc) and GNU-style long options
// (--total, --files0-from), matching the pattern in cmd/cat/main.go.
// Per design decision D2.
func parseFlags(args []string) (options, []string, error) {
	var opts options
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "-" {
			// Per prd005-wc R4.1: "-" means stdin, not a flag.
			files = append(files, arg)
			continue
		}

		// Handle GNU-style long options.
		if strings.HasPrefix(arg, "--") {
			if err := parseLongOption(arg, &opts); err != nil {
				return options{}, nil, err
			}
			continue
		}

		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
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
					opts.printMaxLine = true
				default:
					return options{}, nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
		} else {
			files = append(files, arg)
		}
	}

	// Per prd005-wc R1.1: when no flags are given, default to lines, words, bytes.
	if !opts.printLines && !opts.printWords && !opts.printBytes && !opts.printChars && !opts.printMaxLine {
		opts.printLines = true
		opts.printWords = true
		opts.printBytes = true
		opts.defaultMode = true
	}

	return opts, files, nil
}

// parseLongOption handles --total and --files0-from long options.
func parseLongOption(arg string, opts *options) error {
	if arg == "--total" || strings.HasPrefix(arg, "--total=") {
		val := "auto"
		if strings.HasPrefix(arg, "--total=") {
			val = arg[len("--total="):]
		}
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
			return fmt.Errorf("invalid argument '%s' for '--total'", val)
		}
		return nil
	}

	if arg == "--files0-from" || strings.HasPrefix(arg, "--files0-from=") {
		if strings.HasPrefix(arg, "--files0-from=") {
			opts.files0From = arg[len("--files0-from="):]
		} else {
			return fmt.Errorf("option '--files0-from' requires an argument")
		}
		opts.hasFiles0From = true
		return nil
	}

	return fmt.Errorf("unrecognized option '%s'", arg)
}

// readFiles0From reads NUL-delimited filenames from the given file.
// Per prd005-wc R4.4: when FILE is "-", filenames are read from stdin.
func readFiles0From(file string) ([]string, error) {
	var r io.Reader
	if file == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(file)
		if err != nil {
			if pathErr, ok := err.(*os.PathError); ok {
				return nil, fmt.Errorf("%s: %s", file, pathErr.Err)
			}
			return nil, err
		}
		defer f.Close()
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading files from '%s': %w", file, err)
	}

	var files []string
	for _, name := range bytes.Split(data, []byte{0}) {
		s := string(name)
		if s != "" {
			files = append(files, s)
		}
	}
	return files, nil
}

// countFile opens and counts a single file (or stdin for "-").
// Per prd005-wc R1.2.
func countFile(file string) (counts, error) {
	var r io.Reader
	if file == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(file)
		if err != nil {
			if pathErr, ok := err.(*os.PathError); ok {
				return counts{}, fmt.Errorf("%s: %s", file, pathErr.Err)
			}
			return counts{}, err
		}
		defer f.Close()
		r = f
	}

	return countReader(r)
}

// countReader counts lines, words, bytes, characters, and max line length from
// a reader in a single pass.
//
// Per prd005-wc R2.1: -l counts newline characters (0x0A occurrences).
// Per prd005-wc R2.2: -w counts words (maximal sequences of non-whitespace).
// Per prd005-wc R2.3: -c counts bytes.
// Per prd005-wc R2.4: -m counts characters (runes); invalid UTF-8 bytes count as one character each.
// Per prd005-wc R2.5: -L measures longest line with tab expansion to next multiple of 8.
// Per design decision D3: count 0x0A bytes, not logical lines.
// Per design decision D4: use utf8 rune counting; invalid bytes count as one character.
func countReader(r io.Reader) (counts, error) {
	var c counts
	buf := make([]byte, 32*1024)
	var inWord bool
	var lineLen int64

	// For character counting, we need to handle partial UTF-8 sequences
	// that may span buffer boundaries.
	var partial [utf8.UTFMax]byte
	var partialLen int

	for {
		n, readErr := r.Read(buf)
		data := buf[:n]

		// If we have leftover partial bytes from the previous read,
		// prepend them to the current data for character counting.
		if partialLen > 0 && n > 0 {
			// Try to complete the partial rune.
			needed := runeIncompleteBytes(partial[:partialLen])
			if needed > 0 && needed <= n {
				combined := make([]byte, partialLen+needed)
				copy(combined, partial[:partialLen])
				copy(combined[partialLen:], data[:needed])
				runeCount := countRunes(combined)
				c.chars += int64(runeCount)
				// Process the rest of the data for chars below,
				// but skip the bytes we already consumed.
				data = data[needed:]
				partialLen = 0
			} else if needed > n {
				// Still not enough bytes, accumulate.
				copy(partial[partialLen:], data[:n])
				partialLen += n
				c.bytesCount += int64(n)
				// Process all original bytes for lines/words/maxline.
				processLinesWordsMaxline(buf[:n], &c, &inWord, &lineLen)
				if readErr == io.EOF {
					// Count remaining partial as individual characters.
					c.chars += int64(partialLen)
					updateMaxLine(&c, lineLen)
					return c, nil
				}
				if readErr != nil {
					return c, readErr
				}
				continue
			} else {
				// needed == 0 means partial was actually complete or invalid.
				c.chars += int64(countRunes(partial[:partialLen]))
				partialLen = 0
			}
		} else if partialLen > 0 && n == 0 {
			// EOF with leftover partial bytes: each counts as one character.
			c.chars += int64(partialLen)
			partialLen = 0
		}

		c.bytesCount += int64(len(data))

		// Count characters, handling partial rune at end of buffer.
		if len(data) > 0 {
			// Check if the last bytes form an incomplete UTF-8 sequence.
			trailingIncomplete := trailingIncompleteRune(data)
			if trailingIncomplete > 0 {
				charData := data[:len(data)-trailingIncomplete]
				c.chars += int64(countRunes(charData))
				copy(partial[:], data[len(data)-trailingIncomplete:])
				partialLen = trailingIncomplete
			} else {
				c.chars += int64(countRunes(data))
			}
		}

		// Process lines, words, and max line length from the original buffer chunk.
		processLinesWordsMaxline(buf[:n], &c, &inWord, &lineLen)

		if readErr == io.EOF {
			// Update max line for the final unterminated line.
			updateMaxLine(&c, lineLen)
			return c, nil
		}
		if readErr != nil {
			return c, readErr
		}
	}
}

// processLinesWordsMaxline processes a data chunk for line counting, word
// counting, and max line length computation.
func processLinesWordsMaxline(data []byte, c *counts, inWord *bool, lineLen *int64) {
	for _, b := range data {
		// Per design decision D3: count 0x0A bytes for -l.
		if b == '\n' {
			c.lines++
			updateMaxLine(c, *lineLen)
			*lineLen = 0
			*inWord = false
			continue
		}

		// Tab expansion for -L: advance to next multiple of 8.
		// Per prd005-wc R2.5.
		if b == '\t' {
			*lineLen = (*lineLen/8 + 1) * 8
		} else {
			*lineLen++
		}

		// Word counting: whitespace detection.
		// Per prd005-wc R2.2: word is a maximal sequence of non-whitespace.
		if unicode.IsSpace(rune(b)) {
			*inWord = false
		} else {
			if !*inWord {
				c.words++
				*inWord = true
			}
		}
	}
}

// updateMaxLine updates the max line length if the current line is longer.
func updateMaxLine(c *counts, lineLen int64) {
	if lineLen > c.maxLine {
		c.maxLine = lineLen
	}
}

// countRunes counts the number of runes in data. Invalid UTF-8 bytes each
// count as one character per prd005-wc R2.4.
func countRunes(data []byte) int {
	count := 0
	for len(data) > 0 {
		_, size := utf8.DecodeRune(data)
		count++
		data = data[size:]
	}
	return count
}

// trailingIncompleteRune returns the number of trailing bytes in data that
// form an incomplete UTF-8 sequence. Returns 0 if the data ends on a complete
// rune boundary.
func trailingIncompleteRune(data []byte) int {
	if len(data) == 0 {
		return 0
	}

	// Check from the end of the buffer for the start of a multi-byte sequence.
	for i := 1; i <= utf8.UTFMax && i <= len(data); i++ {
		b := data[len(data)-i]
		if b < 0x80 {
			// ASCII byte: complete.
			return 0
		}
		if b >= 0xC0 {
			// This is a leading byte. Check if the sequence is complete.
			var expectedLen int
			if b < 0xE0 {
				expectedLen = 2
			} else if b < 0xF0 {
				expectedLen = 3
			} else {
				expectedLen = 4
			}
			if i < expectedLen {
				return i
			}
			return 0
		}
		// Continuation byte (0x80-0xBF): keep scanning backwards.
	}
	return 0
}

// runeIncompleteBytes returns the number of additional bytes needed to complete
// a partial UTF-8 sequence. Returns 0 if the partial is complete or invalid.
func runeIncompleteBytes(partial []byte) int {
	if len(partial) == 0 {
		return 0
	}
	r, size := utf8.DecodeRune(partial)
	if r != utf8.RuneError || size != 1 || len(partial) == 1 {
		// Complete rune or single invalid byte.
		if size >= len(partial) {
			return 0
		}
	}
	// Determine expected length from the leading byte.
	b := partial[0]
	var expectedLen int
	if b < 0xC0 {
		return 0 // Invalid leading byte.
	} else if b < 0xE0 {
		expectedLen = 2
	} else if b < 0xF0 {
		expectedLen = 3
	} else if b < 0xF8 {
		expectedLen = 4
	} else {
		return 0 // Invalid.
	}
	if len(partial) >= expectedLen {
		return 0 // Already have enough bytes.
	}
	return expectedLen - len(partial)
}

// shouldShowTotal determines whether to print a total line.
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

// computeColumnWidth determines the column width needed to right-align all
// count values. Per design decision D5.
func computeColumnWidth(allCounts []counts, total counts, showTotal bool, opts *options) int {
	maxVal := int64(0)
	for _, c := range allCounts {
		updateMaxVal(&maxVal, c, opts)
	}
	if showTotal {
		updateMaxVal(&maxVal, total, opts)
	}
	width := digitCount(maxVal)
	if width < 1 {
		width = 1
	}
	return width
}

// updateMaxVal updates maxVal with the largest printable count from c.
func updateMaxVal(maxVal *int64, c counts, opts *options) {
	if opts.printLines && c.lines > *maxVal {
		*maxVal = c.lines
	}
	if opts.printWords && c.words > *maxVal {
		*maxVal = c.words
	}
	if opts.printChars && c.chars > *maxVal {
		*maxVal = c.chars
	} else if opts.printBytes && !opts.printChars && c.bytesCount > *maxVal {
		*maxVal = c.bytesCount
	}
	if opts.printMaxLine && c.maxLine > *maxVal {
		*maxVal = c.maxLine
	}
}

// digitCount returns the number of digits in a non-negative integer.
func digitCount(n int64) int {
	if n == 0 {
		return 1
	}
	count := 0
	for n > 0 {
		count++
		n /= 10
	}
	return count
}

// printCounts writes a formatted count line to the writer.
// Per prd005-wc R2.6: counts are printed in fixed order: lines, words,
// chars-or-bytes, max-line-length.
// Per prd005-wc R3.1: right-aligned in columns.
func printCounts(w *bufio.Writer, c counts, label string, colWidth int, opts *options) {
	first := true
	writeCount := func(val int64) {
		if first {
			fmt.Fprintf(w, "%*d", colWidth, val)
			first = false
		} else {
			fmt.Fprintf(w, " %*d", colWidth, val)
		}
	}

	if opts.printLines {
		writeCount(c.lines)
	}
	if opts.printWords {
		writeCount(c.words)
	}
	// Per prd005-wc R2.3: -m takes precedence over -c in output when both given.
	if opts.printChars {
		writeCount(c.chars)
	} else if opts.printBytes {
		writeCount(c.bytesCount)
	}
	if opts.printMaxLine {
		writeCount(c.maxLine)
	}

	if label != "" {
		fmt.Fprintf(w, " %s", label)
	}
	w.WriteByte('\n')
}
