// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd026-cut R1.1–R1.4: byte and character selection from lines.
// R1.1: -b LIST extracts byte positions using range syntax.
// R1.2: -c LIST extracts character positions (equivalent to -b under LC_ALL=C).
// R1.3: Newlines pass through unchanged; not counted as line content.
// R1.4: Out-of-range positions produce no output bytes.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// byteRange represents an inclusive 1-indexed range of bytes/characters.
type byteRange struct {
	low  int // 1-indexed inclusive start
	high int // 1-indexed inclusive end; math.MaxInt for open-ended
}

// cutConfig holds the parsed flags for this invocation.
type cutConfig struct {
	ranges []byteRange
	files  []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(cfg))
}

// run processes all input sources and returns the exit code.
func run(cfg cutConfig) int {
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	if len(cfg.files) == 0 {
		processReader(w, os.Stdin, cfg.ranges)
	} else {
		exitCode = processFiles(w, cfg.files, cfg.ranges)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cut: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFiles iterates over file arguments and processes each.
func processFiles(w *bufio.Writer, files []string, ranges []byteRange) int {
	exitCode := 0
	for _, name := range files {
		if err := processFile(w, name, ranges); err != nil {
			fmt.Fprintf(os.Stderr, "cut: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a single file (or stdin for "-") and processes it.
func processFile(w *bufio.Writer, name string, ranges []byteRange) error {
	if name == "-" {
		processReader(w, os.Stdin, ranges)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	defer f.Close()
	processReader(w, f, ranges)
	return nil
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// processReader reads lines from r and extracts selected bytes. R1.3, R1.4.
func processReader(w *bufio.Writer, r io.Reader, ranges []byteRange) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		cutBytes(w, line, ranges)
		w.WriteByte('\n')
	}
}

// cutBytes extracts the selected byte ranges from a single line. R1.1, R1.4.
func cutBytes(w *bufio.Writer, line []byte, ranges []byteRange) {
	lineLen := len(line)
	first := true
	for _, r := range ranges {
		lo := r.low - 1 // convert to 0-indexed
		hi := r.high     // exclusive upper bound in 0-indexed
		if hi > lineLen {
			hi = lineLen
		}
		if lo >= lineLen {
			continue
		}
		if lo < 0 {
			lo = 0
		}
		if !first {
			// For byte/char mode, no separator between ranges
		}
		w.Write(line[lo:hi])
		first = false
	}
}

// parseArgs extracts config from command-line arguments.
func parseArgs(args []string) (cutConfig, error) {
	var listStr string
	var files []string
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || (!strings.HasPrefix(arg, "-") || arg == "-") {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		val, advanced := extractFlagValue(arg, args, &i, "-b", "--bytes=")
		if advanced {
			listStr = val
			continue
		}
		val, advanced = extractFlagValue(arg, args, &i, "-c", "--characters=")
		if advanced {
			listStr = val
			continue
		}
		// Unknown flags are ignored for forward compatibility
		files = append(files, arg)
	}
	if listStr == "" {
		return cutConfig{}, fmt.Errorf(
			"you must specify a list of bytes, characters, or fields")
	}
	ranges, err := parseRangeList(listStr)
	if err != nil {
		return cutConfig{}, err
	}
	return cutConfig{ranges: ranges, files: files}, nil
}

// extractFlagValue checks if arg matches -X or --long= form and returns value.
func extractFlagValue(
	arg string, args []string, i *int, short, long string,
) (string, bool) {
	if val, ok := strings.CutPrefix(arg, long); ok {
		return val, true
	}
	if strings.HasPrefix(arg, short) {
		if len(arg) > len(short) {
			return arg[len(short):], true
		}
		if *i+1 < len(args) {
			*i++
			return args[*i], true
		}
		return "", true
	}
	return "", false
}

// parseRangeList parses a comma-separated range list (e.g., "1,3-5,7-").
func parseRangeList(list string) ([]byteRange, error) {
	parts := strings.Split(list, ",")
	var ranges []byteRange
	for _, p := range parts {
		r, err := parseSingleRange(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	ranges = mergeRanges(ranges)
	return ranges, nil
}

// parseSingleRange parses one range element: N, N-M, N-, or -M.
func parseSingleRange(s string) (byteRange, error) {
	if s == "" {
		return byteRange{}, fmt.Errorf("invalid range: empty")
	}
	dashIdx := strings.Index(s, "-")
	if dashIdx < 0 {
		return parseSingleValue(s)
	}
	if dashIdx == 0 {
		return parseEndRange(s[1:])
	}
	if dashIdx == len(s)-1 {
		return parseStartRange(s[:dashIdx])
	}
	return parseFullRange(s[:dashIdx], s[dashIdx+1:])
}

// parseSingleValue parses "N" as range [N,N].
func parseSingleValue(s string) (byteRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return byteRange{}, fmt.Errorf("invalid byte/character position %q", s)
	}
	return byteRange{low: n, high: n}, nil
}

// parseEndRange parses "-M" as range [1,M].
func parseEndRange(s string) (byteRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return byteRange{}, fmt.Errorf("invalid byte/character position %q", s)
	}
	return byteRange{low: 1, high: n}, nil
}

// parseStartRange parses "N-" as range [N,max].
func parseStartRange(s string) (byteRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return byteRange{}, fmt.Errorf("invalid byte/character position %q", s)
	}
	return byteRange{low: n, high: math.MaxInt}, nil
}

// parseFullRange parses "N-M" as range [N,M].
func parseFullRange(lo, hi string) (byteRange, error) {
	n, err := strconv.Atoi(lo)
	if err != nil || n <= 0 {
		return byteRange{}, fmt.Errorf("invalid byte/character position %q", lo)
	}
	m, err := strconv.Atoi(hi)
	if err != nil || m <= 0 {
		return byteRange{}, fmt.Errorf("invalid byte/character position %q", hi)
	}
	if n > m {
		return byteRange{}, fmt.Errorf(
			"invalid decreasing range: %d-%d", n, m)
	}
	return byteRange{low: n, high: m}, nil
}

// mergeRanges sorts and merges overlapping ranges into a minimal set.
func mergeRanges(ranges []byteRange) []byteRange {
	if len(ranges) <= 1 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].low < ranges[j].low
	})
	merged := []byteRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.low <= last.high+1 {
			if r.high > last.high {
				last.high = r.high
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}
