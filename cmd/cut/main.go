// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cut utility.
// Implements prd026-cut R1 (byte/char selection), R2 (field/delimiter),
// R3 (complement), R4 (exit codes, SIGPIPE).
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

const programName = "cut"

// selectionMode indicates which of -b, -c, -f was chosen.
type selectionMode int

const (
	modeNone  selectionMode = iota
	modeByte                // -b
	modeChar                // -c
	modeField               // -f
)

// cutRange represents a 1-based inclusive range [low, high].
// high == 0 means "to end of line."
type cutRange struct {
	low  int
	high int // 0 means unbounded (to end)
}

type options struct {
	mode            selectionMode
	ranges          []cutRange
	delimiter       byte
	outputDelimiter string
	outputDelimSet  bool
	suppress        bool
	complement      bool
	files           []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fatal(err.Error())
	}

	exitCode := 0
	if len(opts.files) == 0 {
		opts.files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	for _, file := range opts.files {
		if err := processFile(w, file, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	os.Exit(exitCode)
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, msg)
	os.Exit(1)
}

func parseArgs(args []string) (*options, error) {
	opts := &options{
		delimiter: '\t',
	}
	var rangeStr string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			opts.files = append(opts.files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		if arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("cut (go-unix-utils)")
			os.Exit(0)
		}
		if arg == "--complement" {
			opts.complement = true
			continue
		}
		if val, ok := strings.CutPrefix(arg, "--output-delimiter="); ok {
			opts.outputDelimiter = val
			opts.outputDelimSet = true
			continue
		}
		if arg == "--output-delimiter" {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("option '--output-delimiter' requires an argument")
			}
			opts.outputDelimiter = args[i]
			opts.outputDelimSet = true
			continue
		}

		// Short flags: can be combined or have values attached
		j := 1
		for j < len(arg) {
			ch := arg[j]
			switch ch {
			case 'b':
				if opts.mode != modeNone && opts.mode != modeByte {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				opts.mode = modeByte
				rangeStr, i = consumeValue(rangeStr, arg, j, args, i)
				j = len(arg)
			case 'c':
				if opts.mode != modeNone && opts.mode != modeChar {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				opts.mode = modeChar
				rangeStr, i = consumeValue(rangeStr, arg, j, args, i)
				j = len(arg)
			case 'f':
				if opts.mode != modeNone && opts.mode != modeField {
					return nil, fmt.Errorf("only one type of list may be specified")
				}
				opts.mode = modeField
				rangeStr, i = consumeValue(rangeStr, arg, j, args, i)
				j = len(arg)
			case 'd':
				rest := arg[j+1:]
				if rest != "" {
					if len(rest) != 1 {
						return nil, fmt.Errorf("the delimiter must be a single character")
					}
					opts.delimiter = rest[0]
					j = len(arg)
				} else {
					i++
					if i >= len(args) {
						return nil, fmt.Errorf("option requires an argument -- 'd'")
					}
					dval := args[i]
					if len(dval) != 1 {
						return nil, fmt.Errorf("the delimiter must be a single character")
					}
					opts.delimiter = dval[0]
					j = len(arg)
				}
			case 's':
				opts.suppress = true
				j++
			default:
				return nil, fmt.Errorf("invalid option -- '%c'", ch)
			}
		}
	}

	if opts.mode == modeNone {
		return nil, fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}

	if opts.mode != modeField && opts.suppress {
		return nil, fmt.Errorf("suppressing non-delimited lines makes sense\n\tonly when operating on fields")
	}

	if !opts.outputDelimSet {
		opts.outputDelimiter = string(opts.delimiter)
	}

	var parseErr error
	opts.ranges, parseErr = parseRangeList(rangeStr)
	if parseErr != nil {
		return nil, parseErr
	}

	return opts, nil
}

// consumeValue reads the value for a flag (-b, -c, -f) either from the
// remainder of the current arg or from the next arg.
func consumeValue(rangeStr, arg string, j int, args []string, i int) (string, int) {
	rest := arg[j+1:]
	if rest != "" {
		return appendRange(rangeStr, rest), i
	}
	i++
	if i < len(args) {
		return appendRange(rangeStr, args[i]), i
	}
	return rangeStr, i
}

func appendRange(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "," + addition
}

func parseRangeList(s string) ([]cutRange, error) {
	if s == "" {
		return nil, fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}

	parts := strings.Split(s, ",")
	var ranges []cutRange

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		r, err := parseSingleRange(part)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}

	if len(ranges) == 0 {
		return nil, fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}

	ranges = mergeRanges(ranges)
	return ranges, nil
}

func parseSingleRange(s string) (cutRange, error) {
	left, right, hasDash := strings.Cut(s, "-")

	if !hasDash {
		// Single number: N
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
		return cutRange{low: n, high: n}, nil
	}

	if left == "" && right == "" {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}

	if left == "" {
		// -M: from 1 to M
		m, err := strconv.Atoi(right)
		if err != nil || m <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
		return cutRange{low: 1, high: m}, nil
	}

	if right == "" {
		// N-: from N to end
		n, err := strconv.Atoi(left)
		if err != nil || n <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
		return cutRange{low: n, high: 0}, nil
	}

	// N-M
	n, err := strconv.Atoi(left)
	if err != nil || n <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	m, err2 := strconv.Atoi(right)
	if err2 != nil || m <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	if n > m {
		return cutRange{}, fmt.Errorf("invalid decreasing range")
	}
	return cutRange{low: n, high: m}, nil
}

// mergeRanges sorts and merges overlapping ranges.
func mergeRanges(ranges []cutRange) []cutRange {
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].low < ranges[j].low
	})

	var merged []cutRange
	for _, r := range ranges {
		if len(merged) == 0 {
			merged = append(merged, r)
			continue
		}
		last := &merged[len(merged)-1]
		if last.high == 0 {
			// Already covers to the end
			continue
		}
		if r.low <= last.high+1 {
			if r.high == 0 {
				last.high = 0
			} else if r.high > last.high {
				last.high = r.high
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// isSelected returns true if 1-based position pos is in the range list.
func isSelected(ranges []cutRange, pos int) bool {
	for _, r := range ranges {
		if r.high == 0 {
			if pos >= r.low {
				return true
			}
		} else if pos >= r.low && pos <= r.high {
			return true
		}
	}
	return false
}

func processFile(w *bufio.Writer, filename string, opts *options) error {
	var r io.Reader
	if filename == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close() // best-effort close
		r = f
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		switch opts.mode {
		case modeByte, modeChar:
			fmt.Fprintln(w, cutBytes(line, opts))
		case modeField:
			out, print := cutFields(line, opts)
			if print {
				fmt.Fprintln(w, out)
			}
		}
	}
	return scanner.Err()
}

func cutBytes(line string, opts *options) string {
	n := len(line)
	var buf strings.Builder
	first := true
	for i := range n {
		pos := i + 1 // 1-based
		sel := isSelected(opts.ranges, pos)
		if opts.complement {
			sel = !sel
		}
		if sel {
			if opts.outputDelimSet && !first {
				buf.WriteString(opts.outputDelimiter)
			}
			buf.WriteByte(line[i])
			first = false
		}
	}
	return buf.String()
}

// cutFields processes a line in field mode. Returns the output string and
// whether the line should be printed (false when -s suppresses it).
func cutFields(line string, opts *options) (string, bool) {
	delimStr := string(opts.delimiter)
	if !strings.Contains(line, delimStr) {
		if opts.suppress {
			// R2.3: suppress lines without delimiter
			return "", false
		}
		// Lines without delimiter are printed unchanged
		return line, true
	}

	fields := strings.Split(line, delimStr)
	n := len(fields)

	var selected []string
	for i := range n {
		pos := i + 1 // 1-based
		sel := isSelected(opts.ranges, pos)
		if opts.complement {
			sel = !sel
		}
		if sel {
			selected = append(selected, fields[i])
		}
	}

	return strings.Join(selected, opts.outputDelimiter), true
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %s OPTION... [FILE]...
Print selected parts of lines from each FILE to standard output.

With no FILE, or when FILE is -, read standard input.

  -b LIST        select only these bytes
  -c LIST        select only these characters
  -d DELIM       use DELIM instead of TAB for field delimiter
  -f LIST        select only these fields
  -s             do not print lines not containing delimiters
  --complement   complement the set of selected bytes, characters or fields
  --output-delimiter=STRING  use STRING as the output delimiter
  --help         display this help and exit
  --version      output version information and exit
`, programName)
}
