// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU cut: remove sections from each line of files.
// Implements prd026-cut R1-R4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// cutMode represents the selection mode: bytes, characters, or fields.
type cutMode int

const (
	modeNone   cutMode = iota
	modeBytes          // -b
	modeChars          // -c
	modeFields         // -f
)

// cutRange represents a single range element in a LIST specification.
// Both low and high are 1-indexed inclusive. A value of 0 for high means "to end of line".
type cutRange struct {
	low  int
	high int // 0 means unbounded (N-)
}

// cutOptions holds the parsed command-line flags for cut.
type cutOptions struct {
	mode            cutMode
	ranges          []cutRange
	delimiter       byte
	outputDelimiter string
	outputDelimSet  bool // whether --output-delimiter was explicitly set
	suppress        bool // -s: suppress lines without delimiter
	complement      bool // --complement: invert selection
	zeroTerminated  bool // -z: use NUL as line terminator
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	if opts.mode == modeNone {
		fmt.Fprintf(os.Stderr, "cut: you must specify a list of bytes, characters, or fields\n")
		fmt.Fprintf(os.Stderr, "Try 'cut --help' for more information.\n")
		os.Exit(1)
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush

	exitCode := 0
	for _, file := range files {
		var r io.Reader
		if file == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cut: %s: No such file or directory\n", file)
				exitCode = 1
				continue
			}
			defer f.Close() // best-effort cleanup
			r = f
		}

		if err := cutReader(r, w, opts); err != nil {
			fmt.Fprintf(os.Stderr, "cut: write error: %v\n", err)
			exitCode = 1
			break
		}
	}

	w.Flush() // best-effort
	os.Exit(exitCode)
}

// cutReader reads from r and writes cut output to w.
func cutReader(r io.Reader, w *bufio.Writer, opts cutOptions) error {
	br := bufio.NewReader(r)
	terminator := byte('\n')
	if opts.zeroTerminated {
		terminator = 0
	}

	for {
		line, err := readLine(br, terminator)
		if err != nil && err != io.EOF {
			return err
		}
		if err == io.EOF && len(line) == 0 {
			return nil
		}

		// Remove trailing terminator for processing.
		hadTerminator := false
		if len(line) > 0 && line[len(line)-1] == terminator {
			line = line[:len(line)-1]
			hadTerminator = true
		}

		var output []byte
		suppressed := false
		switch opts.mode {
		case modeBytes, modeChars:
			// R1.1, R1.2: Under LC_ALL=C, -b and -c are equivalent.
			output = cutBytes(line, opts)
		case modeFields:
			output, suppressed = cutFields(line, opts)
		}

		if !suppressed {
			if _, werr := w.Write(output); werr != nil {
				return werr
			}
			if hadTerminator {
				if werr := w.WriteByte(terminator); werr != nil {
					return werr
				}
			}
		}

		if err == io.EOF {
			return nil
		}
	}
}

// readLine reads bytes until the given terminator or EOF.
func readLine(br *bufio.Reader, terminator byte) ([]byte, error) {
	var line []byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			return line, err
		}
		line = append(line, b)
		if b == terminator {
			return line, nil
		}
	}
}

// cutBytes extracts the selected byte positions from line.
func cutBytes(line []byte, opts cutOptions) []byte {
	selected := buildSelectedSet(len(line), opts.ranges, opts.complement)
	if opts.outputDelimSet {
		// For byte/char mode with --output-delimiter, GNU cut inserts the
		// output delimiter between each range of selected bytes.
		return cutBytesWithOutputDelim(line, opts)
	}
	var result []byte
	for i, b := range line {
		if selected[i] {
			result = append(result, b)
		}
	}
	return result
}

// cutBytesWithOutputDelim handles -b/-c with --output-delimiter set.
// GNU cut outputs the output delimiter between distinct ranges.
func cutBytesWithOutputDelim(line []byte, opts cutOptions) []byte {
	// Merge and sort ranges, then extract each range's bytes, joining with output delimiter.
	merged := mergeRanges(opts.ranges, len(line))
	if opts.complement {
		merged = complementMergedRanges(merged, len(line))
	}

	var parts [][]byte
	for _, r := range merged {
		low := r.low - 1 // convert to 0-based
		high := r.high
		high = min(high, len(line))
		if low < len(line) {
			parts = append(parts, line[low:high])
		}
	}

	var result []byte
	for i, p := range parts {
		if i > 0 {
			result = append(result, []byte(opts.outputDelimiter)...)
		}
		result = append(result, p...)
	}
	return result
}

// mergeRanges normalizes and merges overlapping ranges, capping at maxLen.
// Returns sorted, merged ranges with 1-based inclusive bounds.
func mergeRanges(ranges []cutRange, maxLen int) []cutRange {
	// Normalize: convert 0-high (unbounded) to maxLen, cap values.
	var normalized []cutRange
	for _, r := range ranges {
		low := r.low
		high := r.high
		if high == 0 || high > maxLen {
			high = maxLen
		}
		if low > maxLen {
			continue
		}
		normalized = append(normalized, cutRange{low: low, high: high})
	}

	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].low != normalized[j].low {
			return normalized[i].low < normalized[j].low
		}
		return normalized[i].high < normalized[j].high
	})

	var merged []cutRange
	for _, r := range normalized {
		if len(merged) > 0 && r.low <= merged[len(merged)-1].high+1 {
			if r.high > merged[len(merged)-1].high {
				merged[len(merged)-1].high = r.high
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// complementMergedRanges returns the complement of merged ranges within [1, maxLen].
func complementMergedRanges(merged []cutRange, maxLen int) []cutRange {
	var result []cutRange
	prev := 0
	for _, r := range merged {
		if r.low > prev+1 {
			result = append(result, cutRange{low: prev + 1, high: r.low - 1})
		}
		prev = r.high
	}
	if prev < maxLen {
		result = append(result, cutRange{low: prev + 1, high: maxLen})
	}
	return result
}

// buildSelectedSet returns a boolean slice indicating which 0-based positions are selected.
func buildSelectedSet(length int, ranges []cutRange, complement bool) []bool {
	selected := make([]bool, length)
	for _, r := range ranges {
		low := r.low - 1 // convert to 0-based
		high := r.high
		if high == 0 || high > length {
			high = length
		}
		for i := max(low, 0); i < high; i++ {
			selected[i] = true
		}
	}
	if complement {
		for i := range selected {
			selected[i] = !selected[i]
		}
	}
	return selected
}

// cutFields extracts the selected fields from line.
// The second return value is true if the line was suppressed by -s.
func cutFields(line []byte, opts cutOptions) ([]byte, bool) {
	delim := opts.delimiter
	lineStr := string(line)

	// R2.3: If line contains no delimiter, print unchanged (unless -s).
	if !strings.ContainsRune(lineStr, rune(delim)) {
		if opts.suppress {
			return nil, true
		}
		return line, false
	}

	fields := strings.Split(lineStr, string(delim))
	selected := buildSelectedSet(len(fields), opts.ranges, opts.complement)

	outDelim := string(delim)
	if opts.outputDelimSet {
		outDelim = opts.outputDelimiter
	}

	var parts []string
	for i, f := range fields {
		if selected[i] {
			parts = append(parts, f)
		}
	}

	return []byte(strings.Join(parts, outDelim)), false
}

// parseArgs parses cut command-line flags manually to match GNU cut behavior.
func parseArgs(args []string) (cutOptions, []string) {
	opts := cutOptions{
		delimiter: '\t', // R2.2: default delimiter is tab
	}

	var files []string
	i := 0

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--bytes=") {
			if opts.mode != modeNone && opts.mode != modeBytes {
				fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
				os.Exit(1)
			}
			opts.mode = modeBytes
			opts.ranges = parseRangeList(arg[len("--bytes="):])
			i++
			continue
		}
		if arg == "--bytes" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "cut: option '--bytes' requires an argument\n")
				os.Exit(1)
			}
			if opts.mode != modeNone && opts.mode != modeBytes {
				fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
				os.Exit(1)
			}
			opts.mode = modeBytes
			opts.ranges = parseRangeList(args[i])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--characters=") {
			if opts.mode != modeNone && opts.mode != modeChars {
				fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
				os.Exit(1)
			}
			opts.mode = modeChars
			opts.ranges = parseRangeList(arg[len("--characters="):])
			i++
			continue
		}
		if arg == "--characters" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "cut: option '--characters' requires an argument\n")
				os.Exit(1)
			}
			if opts.mode != modeNone && opts.mode != modeChars {
				fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
				os.Exit(1)
			}
			opts.mode = modeChars
			opts.ranges = parseRangeList(args[i])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--fields=") {
			if opts.mode != modeNone && opts.mode != modeFields {
				fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
				os.Exit(1)
			}
			opts.mode = modeFields
			opts.ranges = parseRangeList(arg[len("--fields="):])
			i++
			continue
		}
		if arg == "--fields" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "cut: option '--fields' requires an argument\n")
				os.Exit(1)
			}
			if opts.mode != modeNone && opts.mode != modeFields {
				fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
				os.Exit(1)
			}
			opts.mode = modeFields
			opts.ranges = parseRangeList(args[i])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--delimiter=") {
			val := arg[len("--delimiter="):]
			if len(val) != 1 {
				fmt.Fprintf(os.Stderr, "cut: the delimiter must be a single character\n")
				os.Exit(1)
			}
			opts.delimiter = val[0]
			i++
			continue
		}
		if arg == "--delimiter" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "cut: option '--delimiter' requires an argument\n")
				os.Exit(1)
			}
			val := args[i]
			if len(val) != 1 {
				fmt.Fprintf(os.Stderr, "cut: the delimiter must be a single character\n")
				os.Exit(1)
			}
			opts.delimiter = val[0]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--output-delimiter=") {
			opts.outputDelimiter = arg[len("--output-delimiter="):]
			opts.outputDelimSet = true
			i++
			continue
		}
		if arg == "--output-delimiter" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "cut: option '--output-delimiter' requires an argument\n")
				os.Exit(1)
			}
			opts.outputDelimiter = args[i]
			opts.outputDelimSet = true
			i++
			continue
		}
		if arg == "--complement" {
			opts.complement = true
			i++
			continue
		}
		if arg == "--only-delimited" {
			opts.suppress = true
			i++
			continue
		}
		if arg == "--zero-terminated" {
			opts.zeroTerminated = true
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'b':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "cut: option requires an argument -- 'b'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					if opts.mode != modeNone && opts.mode != modeBytes {
						fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
						os.Exit(1)
					}
					opts.mode = modeBytes
					opts.ranges = parseRangeList(val)
					j = len(arg)
				case 'c':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "cut: option requires an argument -- 'c'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					if opts.mode != modeNone && opts.mode != modeChars {
						fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
						os.Exit(1)
					}
					opts.mode = modeChars
					opts.ranges = parseRangeList(val)
					j = len(arg)
				case 'f':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "cut: option requires an argument -- 'f'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					if opts.mode != modeNone && opts.mode != modeFields {
						fmt.Fprintf(os.Stderr, "cut: only one type of list may be specified\n")
						os.Exit(1)
					}
					opts.mode = modeFields
					opts.ranges = parseRangeList(val)
					j = len(arg)
				case 'd':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "cut: option requires an argument -- 'd'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					if len(val) != 1 {
						fmt.Fprintf(os.Stderr, "cut: the delimiter must be a single character\n")
						os.Exit(1)
					}
					opts.delimiter = val[0]
					j = len(arg)
				case 's':
					opts.suppress = true
					j++
				case 'z':
					opts.zeroTerminated = true
					j++
				default:
					fmt.Fprintf(os.Stderr, "cut: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			i++
			continue
		}

		// Not a flag; treat as file argument.
		break
	}

	files = append(files, args[i:]...)
	return opts, files
}

// parseRangeList parses a comma-separated LIST of range specifications.
// Each element is N, N-M, N-, or -M. Positions are 1-indexed.
func parseRangeList(s string) []cutRange {
	parts := strings.Split(s, ",")
	var ranges []cutRange
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		r := parseRange(p)
		ranges = append(ranges, r)
	}
	if len(ranges) == 0 {
		fmt.Fprintf(os.Stderr, "cut: invalid range specification: '%s'\n", s)
		os.Exit(1)
	}
	return ranges
}

// parseRange parses a single range element: N, N-M, N-, or -M.
func parseRange(s string) cutRange {
	if lowStr, highStr, found := strings.Cut(s, "-"); found {

		if lowStr == "" && highStr == "" {
			fmt.Fprintf(os.Stderr, "cut: invalid range with no endpoint: -\n")
			os.Exit(1)
		}

		var low, high int

		if lowStr == "" {
			// -M: from 1 to M
			low = 1
		} else {
			n, err := strconv.Atoi(lowStr)
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "cut: invalid byte, character or field list\n")
				os.Exit(1)
			}
			low = n
		}

		if highStr == "" {
			// N-: from N to end (0 means unbounded)
			high = 0
		} else {
			n, err := strconv.Atoi(highStr)
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "cut: invalid byte, character or field list\n")
				os.Exit(1)
			}
			high = n
		}

		if high != 0 && low > high {
			fmt.Fprintf(os.Stderr, "cut: invalid decreasing range\n")
			os.Exit(1)
		}

		return cutRange{low: low, high: high}
	}

	// Single value N.
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr, "cut: invalid byte, character or field list\n")
		os.Exit(1)
	}
	return cutRange{low: n, high: n}
}
