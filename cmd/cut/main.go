// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cut: remove sections from lines.
// Implements srd026-cut R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// cutRange represents a half-open range [low, high) of 1-indexed positions.
type cutRange struct {
	low  int
	high int // 0 means unbounded (to end of line)
}

// parseRange parses a single range element like "N", "N-M", "N-", or "-M".
// R1.1: ranges are 1-indexed; -M means 1-M, N- means N to end.
func parseRange(s string) (cutRange, error) {
	if s == "" {
		return cutRange{}, fmt.Errorf("invalid range: empty")
	}
	dash := strings.IndexByte(s, '-')
	if dash < 0 {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
		return cutRange{low: n, high: n}, nil
	}
	return parseRangeWithDash(s, dash)
}

// parseRangeWithDash handles the N-M, N-, and -M forms.
func parseRangeWithDash(s string, dash int) (cutRange, error) {
	lo, hi := s[:dash], s[dash+1:]
	var low, high int
	var err error
	if lo == "" {
		low = 1
	} else {
		low, err = strconv.Atoi(lo)
		if err != nil || low <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
	}
	if hi == "" {
		high = 0 // unbounded
	} else {
		high, err = strconv.Atoi(hi)
		if err != nil || high <= 0 {
			return cutRange{}, fmt.Errorf("invalid byte, character or field list")
		}
	}
	if high != 0 && low > high {
		return cutRange{}, fmt.Errorf("invalid decreasing range")
	}
	return cutRange{low: low, high: high}, nil
}

// parseRangeList parses a comma-separated list of ranges.
func parseRangeList(s string) ([]cutRange, error) {
	if s == "" {
		return nil, fmt.Errorf("cut: you must specify a list of bytes, characters, or fields")
	}
	parts := strings.Split(s, ",")
	ranges := make([]cutRange, 0, len(parts))
	for _, p := range parts {
		r, err := parseRange(p)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return mergeRanges(ranges), nil
}

// mergeRanges sorts and merges overlapping ranges.
func mergeRanges(ranges []cutRange) []cutRange {
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].low < ranges[j].low
	})
	merged := []cutRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if last.high == 0 || (r.low <= last.high+1) {
			if r.high == 0 || (last.high != 0 && r.high > last.high) {
				last.high = r.high
			} else if r.high == 0 {
				last.high = 0
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// byteSelected returns true if 1-indexed position pos is in any range.
func byteSelected(pos int, ranges []cutRange) bool {
	for _, r := range ranges {
		if pos < r.low {
			return false // ranges are sorted
		}
		if r.high == 0 || pos <= r.high {
			return true
		}
	}
	return false
}

// cutMode identifies the selection mode.
type cutMode int

const (
	modeNone cutMode = iota
	modeByte
	modeChar
	modeField
)

// config holds the parsed command-line options.
type config struct {
	mode   cutMode
	ranges []cutRange
	files  []string
}

// extractByteFlag tries to extract -b LIST from the current argument.
func extractByteFlag(arg string, args []string, i int) (string, int, error) {
	return extractListFlag(arg, args, i, "-b", "--bytes")
}

// extractCharFlag tries to extract -c LIST from the current argument.
func extractCharFlag(arg string, args []string, i int) (string, int, error) {
	return extractListFlag(arg, args, i, "-c", "--characters")
}

// extractListFlag is a helper for extracting -X LIST or --long=LIST.
func extractListFlag(arg string, args []string, i int, short, long string) (string, int, error) {
	if strings.HasPrefix(arg, long+"=") {
		return arg[len(long)+1:], 0, nil
	}
	if arg == long {
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("option '%s' requires an argument", long)
		}
		return args[i+1], 1, nil
	}
	if strings.HasPrefix(arg, short) {
		rest := arg[len(short):]
		if rest != "" {
			return rest, 0, nil
		}
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("option requires an argument -- '%s'", short[1:])
		}
		return args[i+1], 1, nil
	}
	return "", 0, nil
}

// setMode sets the selection mode, returning an error on conflict.
func setMode(cfg *config, m cutMode, list string) error {
	if cfg.mode != modeNone {
		return fmt.Errorf("only one type of list may be specified")
	}
	cfg.mode = m
	ranges, err := parseRangeList(list)
	if err != nil {
		return err
	}
	cfg.ranges = ranges
	return nil
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (config, error) {
	var cfg config
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (arg != "-" && !strings.HasPrefix(arg, "-")) {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}
		skip, err := parseFlag(&cfg, arg, args, i)
		if err != nil {
			return config{}, err
		}
		i += skip
	}
	if cfg.mode == modeNone {
		return config{}, fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg, nil
}

// parseFlag handles a single flag argument, returning extra args consumed.
func parseFlag(cfg *config, arg string, args []string, i int) (int, error) {
	val, skip, err := extractByteFlag(arg, args, i)
	if err != nil {
		return 0, err
	}
	if val != "" {
		return skip, setMode(cfg, modeByte, val)
	}
	val, skip, err = extractCharFlag(arg, args, i)
	if err != nil {
		return 0, err
	}
	if val != "" {
		return skip, setMode(cfg, modeChar, val)
	}
	return 0, fmt.Errorf("invalid option -- '%s'", strings.TrimLeft(arg, "-"))
}

// openInput returns os.Stdin for "-", otherwise opens the named file.
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError extracts the underlying error for GNU-compatible messages.
func formatOpenError(name string, err error) error {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// cutBytes processes a single reader in byte/character mode.
// R1.1: extract specified byte positions.
// R1.3: newlines pass through; not counted as part of line.
// R1.4: short lines produce only existing bytes.
func cutBytes(r io.Reader, w *bufio.Writer, ranges []cutRange) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if werr := writeByteLine(w, line, ranges); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeByteLine writes the selected bytes from a single line.
// R1.3: newline is stripped before selection and always re-added in output.
// R1.4: positions beyond line length produce nothing.
func writeByteLine(w *bufio.Writer, line []byte, ranges []cutRange) error {
	content := line
	if len(content) > 0 && content[len(content)-1] == '\n' {
		content = content[:len(content)-1]
	}
	for pos := 1; pos <= len(content); pos++ {
		if byteSelected(pos, ranges) {
			if err := w.WriteByte(content[pos-1]); err != nil {
				return err
			}
		}
	}
	return w.WriteByte('\n')
}

// cutFile processes a single file.
func cutFile(name string, w *bufio.Writer, cfg config) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	// R1.2: -c is equivalent to -b under LC_ALL=C
	return cutBytes(r, w, cfg.ranges)
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %s\n", err)
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	for _, name := range cfg.files {
		if err := cutFile(name, w, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "cut: %s\n", err)
			exitCode = 1
		}
	}

	// best-effort flush; SIGPIPE handler covers broken pipe
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cut: write error\n")
		exitCode = 1
	}

	os.Exit(exitCode)
}
