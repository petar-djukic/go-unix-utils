// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd026-cut R1.1-R1.4, R2.1-R2.4, R3.1-R3.3, R4.1-R4.4: cmd/cut
// removes sections from each line of input. This file covers byte selection (-b),
// character selection (-c), field selection (-f) with delimiter (-d), suppress
// (-s), complement, and output-delimiter support. R3.1: reads stdin when no
// FILE operands are given. R3.2: processes multiple files sequentially. R3.3:
// a FILE operand of '-' reads standard input. R4.1-R4.4: --version prints
// version info and exits 0, --help prints usage and exits 0, error messages for
// invalid options and missing flags to stderr with exit 1, edge case handling
// for empty input, single-char delimiter, and multi-byte output-delimiter.
// Installs SIGPIPE handler.
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
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU cut format.
const progName = "cut"

// selMode represents the selection mode: bytes, characters, or fields.
type selMode int

const (
	modeNone selMode = iota
	modeBytes
	modeChars
	modeFields
)

// interval represents a 1-based inclusive range [start, end].
// end == -1 means "to end of line".
type interval struct {
	start int // 1-based
	end   int // 1-based, or -1 for open-ended
}

func main() {
	sys.InstallSIGPIPEHandler()

	mode := modeNone
	var listStr string
	delimiter := "\t"
	outputDelimiter := ""
	outputDelimiterSet := false
	suppress := false
	complement := false
	args := os.Args[1:]
	var files []string
	modeCount := 0

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// R4.4: --help prints usage and exits 0.
		if arg == "--help" {
			fmt.Fprintf(os.Stdout,
				"Usage: %s OPTION... [FILE]...\n"+
					"Print selected parts of lines from each FILE to standard output.\n\n"+
					"With no FILE, or when FILE is -, read standard input.\n\n"+
					"  -b, --bytes=LIST        select only these bytes\n"+
					"  -c, --characters=LIST   select only these characters\n"+
					"  -d, --delimiter=DELIM   use DELIM instead of TAB for field delimiter\n"+
					"  -f, --fields=LIST       select only these fields\n"+
					"  -s, --only-delimited    do not print lines not containing delimiters\n"+
					"      --complement        complement the set of selected bytes, characters\n"+
					"                            or fields\n"+
					"      --output-delimiter=STRING  use STRING as the output delimiter\n"+
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
		if arg == "--complement" {
			complement = true
			continue
		}
		if arg == "--only-delimited" {
			suppress = true
			continue
		}
		if strings.HasPrefix(arg, "--output-delimiter=") {
			outputDelimiter = arg[len("--output-delimiter="):]
			outputDelimiterSet = true
			continue
		}
		if strings.HasPrefix(arg, "--bytes=") {
			mode = modeBytes
			modeCount++
			listStr = arg[len("--bytes="):]
			continue
		}
		if arg == "--bytes" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'b'\n", progName)
				os.Exit(1)
			}
			i++
			mode = modeBytes
			modeCount++
			listStr = args[i]
			continue
		}
		if strings.HasPrefix(arg, "--characters=") {
			mode = modeChars
			modeCount++
			listStr = arg[len("--characters="):]
			continue
		}
		if arg == "--characters" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'c'\n", progName)
				os.Exit(1)
			}
			i++
			mode = modeChars
			modeCount++
			listStr = args[i]
			continue
		}
		if strings.HasPrefix(arg, "--fields=") {
			mode = modeFields
			modeCount++
			listStr = arg[len("--fields="):]
			continue
		}
		if arg == "--fields" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'f'\n", progName)
				os.Exit(1)
			}
			i++
			mode = modeFields
			modeCount++
			listStr = args[i]
			continue
		}
		if strings.HasPrefix(arg, "--delimiter=") {
			delimiter = arg[len("--delimiter="):]
			if len(delimiter) != 1 {
				fmt.Fprintf(os.Stderr, "%s: the delimiter must be a single character\n", progName)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
			continue
		}
		if arg == "--delimiter" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'd'\n", progName)
				os.Exit(1)
			}
			i++
			delimiter = args[i]
			if len(delimiter) != 1 {
				fmt.Fprintf(os.Stderr, "%s: the delimiter must be a single character\n", progName)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
			continue
		}

		// Short options: may be combined or have values attached.
		flags := arg[1:]
		for j := 0; j < len(flags); j++ {
			switch flags[j] {
			case 'b':
				mode = modeBytes
				modeCount++
				val := flags[j+1:]
				if val == "" {
					if i+1 >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'b'\n", progName)
						os.Exit(1)
					}
					i++
					val = args[i]
				}
				listStr = val
				j = len(flags) // consumed rest
			case 'c':
				mode = modeChars
				modeCount++
				val := flags[j+1:]
				if val == "" {
					if i+1 >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'c'\n", progName)
						os.Exit(1)
					}
					i++
					val = args[i]
				}
				listStr = val
				j = len(flags)
			case 'f':
				mode = modeFields
				modeCount++
				val := flags[j+1:]
				if val == "" {
					if i+1 >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'f'\n", progName)
						os.Exit(1)
					}
					i++
					val = args[i]
				}
				listStr = val
				j = len(flags)
			case 'd':
				val := flags[j+1:]
				if val == "" {
					if i+1 >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'd'\n", progName)
						os.Exit(1)
					}
					i++
					val = args[i]
				}
				if len(val) != 1 {
					fmt.Fprintf(os.Stderr, "%s: the delimiter must be a single character\n", progName)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
					os.Exit(1)
				}
				delimiter = val
				j = len(flags)
			case 's':
				suppress = true
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
		}
	}

	// R1.4: exactly one of -b, -c, or -f must be specified.
	if modeCount == 0 {
		fmt.Fprintf(os.Stderr, "%s: you must specify a list of bytes, characters, or fields\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
	if modeCount > 1 {
		fmt.Fprintf(os.Stderr, "%s: only one list may be specified\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	intervals, err := parseRangeList(listStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// Default output delimiter for field mode is the input delimiter.
	if !outputDelimiterSet {
		outputDelimiter = delimiter
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	if len(files) == 0 {
		if err := cutReader(os.Stdin, w, mode, intervals, complement, delimiter, outputDelimiter, outputDelimiterSet, suppress); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		for _, name := range files {
			if name == "-" {
				if err := cutReader(os.Stdin, w, mode, intervals, complement, delimiter, outputDelimiter, outputDelimiterSet, suppress); err != nil {
					fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
					exitCode = 1
				}
				continue
			}
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
				exitCode = 1
				continue
			}
			if err := cutReader(f, w, mode, intervals, complement, delimiter, outputDelimiter, outputDelimiterSet, suppress); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
				exitCode = 1
			}
			f.Close() // best-effort close; read errors already reported
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseRangeList parses a comma-separated list of range specs into merged intervals.
// Each spec is one of: N, N-M, N-, -M (1-based inclusive).
func parseRangeList(s string) ([]interval, error) {
	parts := strings.Split(s, ",")
	var intervals []interval

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		iv, err := parseRange(p)
		if err != nil {
			return nil, err
		}
		intervals = append(intervals, iv)
	}

	if len(intervals) == 0 {
		return nil, fmt.Errorf("byte/character positions are numbered from 1")
	}

	return mergeIntervals(intervals), nil
}

// parseRange parses a single range spec: N, N-M, N-, -M.
// Error messages match GNU cut format for differential testing.
func parseRange(s string) (interval, error) {
	left, right, hasDash := strings.Cut(s, "-")
	if !hasDash {
		// Single value: N
		n, err := strconv.Atoi(s)
		if err != nil {
			return interval{}, fmt.Errorf("invalid byte/character position '%s'", s)
		}
		if n <= 0 {
			return interval{}, fmt.Errorf("byte/character positions are numbered from 1")
		}
		return interval{start: n, end: n}, nil
	}

	if left == "" && right == "" {
		return interval{}, fmt.Errorf("invalid range with no endpoint: -")
	}

	if left == "" {
		// -M: from 1 to M
		m, err := strconv.Atoi(right)
		if err != nil {
			return interval{}, fmt.Errorf("invalid byte/character position '%s'", right)
		}
		if m <= 0 {
			return interval{}, fmt.Errorf("byte/character positions are numbered from 1")
		}
		return interval{start: 1, end: m}, nil
	}

	if right == "" {
		// N-: from N to end
		n, err := strconv.Atoi(left)
		if err != nil {
			return interval{}, fmt.Errorf("invalid byte/character position '%s'", left)
		}
		if n <= 0 {
			return interval{}, fmt.Errorf("byte/character positions are numbered from 1")
		}
		return interval{start: n, end: -1}, nil
	}

	// N-M
	n, err := strconv.Atoi(left)
	if err != nil {
		return interval{}, fmt.Errorf("invalid byte/character position '%s'", left)
	}
	if n <= 0 {
		return interval{}, fmt.Errorf("byte/character positions are numbered from 1")
	}
	m, err := strconv.Atoi(right)
	if err != nil {
		return interval{}, fmt.Errorf("invalid byte/character position '%s'", right)
	}
	if m <= 0 {
		return interval{}, fmt.Errorf("byte/character positions are numbered from 1")
	}
	if n > m {
		return interval{}, fmt.Errorf("invalid decreasing range")
	}
	return interval{start: n, end: m}, nil
}

// mergeIntervals sorts and merges overlapping intervals. An end of -1 means
// open-ended (to end of line).
func mergeIntervals(ivs []interval) []interval {
	if len(ivs) == 0 {
		return ivs
	}

	sort.Slice(ivs, func(i, j int) bool {
		if ivs[i].start != ivs[j].start {
			return ivs[i].start < ivs[j].start
		}
		// Open-ended sorts after bounded.
		if ivs[i].end == -1 {
			return false
		}
		if ivs[j].end == -1 {
			return true
		}
		return ivs[i].end < ivs[j].end
	})

	merged := []interval{ivs[0]}
	for _, iv := range ivs[1:] {
		last := &merged[len(merged)-1]
		if last.end == -1 {
			// Already covers everything from last.start onwards.
			continue
		}
		if iv.start <= last.end+1 {
			// Overlapping or adjacent — merge.
			if iv.end == -1 {
				last.end = -1
			} else if iv.end > last.end {
				last.end = iv.end
			}
		} else {
			merged = append(merged, iv)
		}
	}

	return merged
}

// cutReader reads from r and writes selected portions of each line to w.
func cutReader(r io.Reader, w *bufio.Writer, mode selMode, intervals []interval, complement bool, delim, outDelim string, outDelimSet bool, suppress bool) error {
	br := bufio.NewReader(r)

	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
			content := line
			if hasNewline {
				content = line[:len(line)-1]
			}

			var out string
			switch mode {
			case modeBytes, modeChars:
				// R1.2, R1.3: under LC_ALL=C, -c and -b are equivalent.
				// R4.4: when --output-delimiter is set, insert it between disjoint ranges.
				out = selectBytes(content, intervals, complement, outDelim, outDelimSet)
			case modeFields:
				var skip bool
				out, skip = selectFields(content, intervals, complement, delim, outDelim, suppress)
				if skip {
					if err != nil {
						if err == io.EOF {
							return nil
						}
						return err
					}
					continue
				}
			}

			if _, werr := w.WriteString(out); werr != nil {
				return werr
			}
			if werr := w.WriteByte('\n'); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// selectBytes extracts selected byte positions from a line.
// R1.1: 1-based byte selection. R1.4: out-of-range positions produce no output.
// R1.3: newlines are not part of the line; they are handled by the caller.
// R4.4: when outDelimSet is true, outDelim is inserted between disjoint ranges.
func selectBytes(line string, intervals []interval, complement bool, outDelim string, outDelimSet bool) string {
	n := len(line)
	if n == 0 {
		return ""
	}

	if complement {
		return selectBytesComplement(line, intervals)
	}

	var buf strings.Builder
	first := true
	for _, iv := range intervals {
		start := iv.start - 1 // convert to 0-based
		if start >= n {
			continue
		}
		end := iv.end
		if end == -1 || end > n {
			end = n
		}
		if outDelimSet {
			if !first {
				buf.WriteString(outDelim)
			}
			first = false
		}
		buf.WriteString(line[start:end])
	}
	return buf.String()
}

// selectBytesComplement outputs bytes NOT in the selected intervals.
func selectBytesComplement(line string, intervals []interval) string {
	n := len(line)
	selected := make([]bool, n)
	for _, iv := range intervals {
		start := iv.start - 1
		end := iv.end
		if end == -1 || end > n {
			end = n
		}
		if start < 0 {
			start = 0
		}
		for k := start; k < end; k++ {
			selected[k] = true
		}
	}
	var buf strings.Builder
	for k := range n {
		if !selected[k] {
			buf.WriteByte(line[k])
		}
	}
	return buf.String()
}

// selectFields extracts selected fields from a line.
// R2.1-R2.4: field-based selection with delimiter, output delimiter, and suppress.
// Returns the output string and whether the line should be skipped entirely.
func selectFields(line string, intervals []interval, complement bool, delim, outDelim string, suppress bool) (string, bool) {
	if !strings.Contains(line, delim) {
		// R2.3: line contains no delimiter.
		if suppress {
			return "", true
		}
		return line, false
	}

	fields := strings.Split(line, delim)
	numFields := len(fields)

	if complement {
		return selectFieldsComplement(fields, numFields, intervals, outDelim), false
	}

	var parts []string
	for _, iv := range intervals {
		start := iv.start - 1
		end := iv.end
		if end == -1 || end > numFields {
			end = numFields
		}
		if start >= numFields {
			continue
		}
		for k := start; k < end; k++ {
			parts = append(parts, fields[k])
		}
	}

	return strings.Join(parts, outDelim), false
}

// selectFieldsComplement outputs fields NOT in the selected intervals.
func selectFieldsComplement(fields []string, numFields int, intervals []interval, outDelim string) string {
	selected := make([]bool, numFields)
	for _, iv := range intervals {
		start := iv.start - 1
		end := iv.end
		if end == -1 || end > numFields {
			end = numFields
		}
		if start < 0 {
			start = 0
		}
		for k := start; k < end; k++ {
			selected[k] = true
		}
	}
	var parts []string
	for k := range numFields {
		if !selected[k] {
			parts = append(parts, fields[k])
		}
	}
	return strings.Join(parts, outDelim)
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
