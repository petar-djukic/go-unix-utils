// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cut utility for removing sections from lines.
//
// Implements prd026-cut: byte selection (R1), field selection (R2),
// complement mode (R3), exit codes and SIGPIPE (R4).
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// selectionMode selects between byte, character, and field operation.
type selectionMode int

const (
	modeNone selectionMode = iota
	modeBytes
	modeChars
	modeFields
)

// config holds the parsed command-line options.
type config struct {
	mode            selectionMode
	ranges          []cutRange
	delimiter       byte
	outputDelimiter string
	outputDelimSet  bool
	suppress        bool
	complement      bool
	files           []string
}

// cutRange represents a single range in a LIST specification.
// Both low and high are 1-indexed and inclusive. A value of 0 for high means
// "to end of line."
type cutRange struct {
	low  int
	high int // 0 means unbounded (to end)
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %v\n", err)
		os.Exit(1)
	}

	if cfg.mode == modeNone {
		fmt.Fprintf(os.Stderr, "cut: you must specify a list of bytes, characters, or fields\n")
		os.Exit(1)
	}

	exitCode := run(cfg)
	os.Exit(exitCode)
}

func run(cfg config) int {
	exitCode := 0
	inputs := cfg.files
	if len(inputs) == 0 {
		inputs = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	for _, name := range inputs {
		if err := processFile(w, name, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "cut: %v\n", err)
			exitCode = 1
		}
	}

	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

func processFile(w *bufio.Writer, name string, cfg config) error {
	var f *os.File
	if name == "-" {
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(name)
		if err != nil {
			return fmt.Errorf("%s: No such file or directory", name)
		}
		defer f.Close()
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		switch cfg.mode {
		case modeBytes, modeChars:
			if _, err := fmt.Fprintln(w, cutBytes(line, cfg)); err != nil {
				return err
			}
		case modeFields:
			output, suppress := cutFields(line, cfg)
			if suppress {
				continue
			}
			if _, err := fmt.Fprintln(w, output); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// cutBytes extracts selected byte/character positions from a line. R1.1, R1.2.
func cutBytes(line string, cfg config) string {
	n := len(line)
	selected := selectedPositions(n, cfg.ranges, cfg.complement)

	var buf strings.Builder
	outDelim := ""
	if cfg.outputDelimSet {
		outDelim = cfg.outputDelimiter
	}

	first := true
	for i := range n {
		if selected[i] {
			if cfg.outputDelimSet && !first {
				buf.WriteString(outDelim)
			}
			buf.WriteByte(line[i])
			first = false
		}
	}
	return buf.String()
}

// cutFields extracts selected fields from a line. R2.1, R2.3.
// Returns the output string and whether the line should be suppressed.
func cutFields(line string, cfg config) (string, bool) {
	delim := string(cfg.delimiter)
	if !strings.Contains(line, delim) {
		if cfg.suppress {
			return "", true
		}
		// R2.3: without -s, print unchanged.
		return line, false
	}

	fields := strings.Split(line, delim)
	n := len(fields)
	selected := selectedPositions(n, cfg.ranges, cfg.complement)

	outDelim := delim
	if cfg.outputDelimSet {
		outDelim = cfg.outputDelimiter
	}

	var result []string
	for i := range n {
		if selected[i] {
			result = append(result, fields[i])
		}
	}
	return strings.Join(result, outDelim), false
}

// selectedPositions returns a boolean slice of length n indicating which
// 0-indexed positions are selected by the given ranges. If complement is true,
// the selection is inverted. R3.1.
func selectedPositions(n int, ranges []cutRange, complement bool) []bool {
	sel := make([]bool, n)
	for _, r := range ranges {
		low := max(r.low-1, 0) // convert to 0-indexed
		high := r.high - 1
		if r.high == 0 {
			high = n - 1
		}
		if high >= n {
			high = n - 1
		}
		for i := low; i <= high; i++ {
			sel[i] = true
		}
	}
	if complement {
		for i := range sel {
			sel[i] = !sel[i]
		}
	}
	return sel
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	cfg := config{
		delimiter: '\t',
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			cfg.files = append(cfg.files, args[i:]...)
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--output-delimiter=") {
			cfg.outputDelimiter = arg[len("--output-delimiter="):]
			cfg.outputDelimSet = true
			i++
			continue
		}
		if arg == "--output-delimiter" {
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("option '--output-delimiter' requires an argument")
			}
			cfg.outputDelimiter = args[i]
			cfg.outputDelimSet = true
			i++
			continue
		}
		if arg == "--complement" {
			cfg.complement = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--bytes=") {
			cfg.mode = modeBytes
			r, err := parseRangeList(arg[len("--bytes="):])
			if err != nil {
				return cfg, err
			}
			cfg.ranges = r
			i++
			continue
		}
		if strings.HasPrefix(arg, "--characters=") {
			cfg.mode = modeChars
			r, err := parseRangeList(arg[len("--characters="):])
			if err != nil {
				return cfg, err
			}
			cfg.ranges = r
			i++
			continue
		}
		if strings.HasPrefix(arg, "--fields=") {
			cfg.mode = modeFields
			r, err := parseRangeList(arg[len("--fields="):])
			if err != nil {
				return cfg, err
			}
			cfg.ranges = r
			i++
			continue
		}
		if strings.HasPrefix(arg, "--delimiter=") {
			val := arg[len("--delimiter="):]
			if len(val) != 1 {
				return cfg, fmt.Errorf("the delimiter must be a single character")
			}
			cfg.delimiter = val[0]
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'b':
					cfg.mode = modeBytes
					rest := arg[j+1:]
					if rest != "" {
						r, err := parseRangeList(rest)
						if err != nil {
							return cfg, err
						}
						cfg.ranges = r
					} else {
						i++
						if i >= len(args) {
							return cfg, fmt.Errorf("option requires an argument -- 'b'")
						}
						r, err := parseRangeList(args[i])
						if err != nil {
							return cfg, err
						}
						cfg.ranges = r
					}
					j = len(arg)
				case 'c':
					cfg.mode = modeChars
					rest := arg[j+1:]
					if rest != "" {
						r, err := parseRangeList(rest)
						if err != nil {
							return cfg, err
						}
						cfg.ranges = r
					} else {
						i++
						if i >= len(args) {
							return cfg, fmt.Errorf("option requires an argument -- 'c'")
						}
						r, err := parseRangeList(args[i])
						if err != nil {
							return cfg, err
						}
						cfg.ranges = r
					}
					j = len(arg)
				case 'f':
					cfg.mode = modeFields
					rest := arg[j+1:]
					if rest != "" {
						r, err := parseRangeList(rest)
						if err != nil {
							return cfg, err
						}
						cfg.ranges = r
					} else {
						i++
						if i >= len(args) {
							return cfg, fmt.Errorf("option requires an argument -- 'f'")
						}
						r, err := parseRangeList(args[i])
						if err != nil {
							return cfg, err
						}
						cfg.ranges = r
					}
					j = len(arg)
				case 'd':
					rest := arg[j+1:]
					if rest != "" {
						if len(rest) != 1 {
							return cfg, fmt.Errorf("the delimiter must be a single character")
						}
						cfg.delimiter = rest[0]
					} else {
						i++
						if i >= len(args) {
							return cfg, fmt.Errorf("option requires an argument -- 'd'")
						}
						val := args[i]
						if len(val) != 1 {
							return cfg, fmt.Errorf("the delimiter must be a single character")
						}
						cfg.delimiter = val[0]
					}
					j = len(arg)
				case 's':
					cfg.suppress = true
					j++
				default:
					return cfg, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			i++
			continue
		}

		// File argument.
		cfg.files = append(cfg.files, arg)
		i++
	}

	return cfg, nil
}

// parseRangeList parses a comma-separated list of range specifications.
// Each element can be N, N-M, N-, or -M (all 1-indexed).
func parseRangeList(s string) ([]cutRange, error) {
	if s == "" {
		return nil, fmt.Errorf("invalid range: ''")
	}

	parts := strings.Split(s, ",")
	var ranges []cutRange
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		r, err := parseSingleRange(p)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}

	// Sort and merge ranges for efficiency.
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].low < ranges[j].low
	})

	return ranges, nil
}

// parseSingleRange parses a single range element: N, N-M, N-, or -M.
func parseSingleRange(s string) (cutRange, error) {
	left, right, hasDash := strings.Cut(s, "-")
	if !hasDash {
		// Simple number N.
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
		return cutRange{low: n, high: n}, nil
	}

	if left == "" && right == "" {
		return cutRange{}, fmt.Errorf("invalid range with no endpoint: -")
	}

	if left == "" {
		// -M form.
		m, err := strconv.Atoi(right)
		if err != nil || m <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
		return cutRange{low: 1, high: m}, nil
	}

	if right == "" {
		// N- form.
		n, err := strconv.Atoi(left)
		if err != nil || n <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
		return cutRange{low: n, high: 0}, nil // 0 means unbounded
	}

	// N-M form.
	n, err := strconv.Atoi(left)
	if err != nil || n <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	m, err := strconv.Atoi(right)
	if err != nil || m <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	return cutRange{low: n, high: m}, nil
}
