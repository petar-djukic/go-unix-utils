// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cut: remove sections from lines.
// Implements srd026-cut R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4.
package main

import (
	"bufio"
	"bytes"
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
	mode           cutMode
	ranges         []cutRange
	complement     bool
	delimiter      byte
	delimSet       bool
	onlyDelimited  bool
	outputDelim    string
	outputDelimSet bool
	files          []string
}

// extractByteFlag tries to extract -b LIST from the current argument.
func extractByteFlag(arg string, args []string, i int) (string, int, error) {
	return extractListFlag(arg, args, i, "-b", "--bytes")
}

// extractCharFlag tries to extract -c LIST from the current argument.
func extractCharFlag(arg string, args []string, i int) (string, int, error) {
	return extractListFlag(arg, args, i, "-c", "--characters")
}

// extractFieldFlag tries to extract -f LIST from the current argument.
func extractFieldFlag(arg string, args []string, i int) (string, int, error) {
	return extractListFlag(arg, args, i, "-f", "--fields")
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

// extractDelimFlag tries to extract -d DELIM from the current argument.
// R2.2: delimiter must be exactly one byte.
func extractDelimFlag(arg string, args []string, i int) (string, int, error) {
	if strings.HasPrefix(arg, "--delimiter=") {
		return arg[len("--delimiter="):], 0, nil
	}
	if arg == "--delimiter" {
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("option '--delimiter' requires an argument")
		}
		return args[i+1], 1, nil
	}
	if strings.HasPrefix(arg, "-d") {
		rest := arg[2:]
		if rest != "" {
			return rest, 0, nil
		}
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("option requires an argument -- 'd'")
		}
		return args[i+1], 1, nil
	}
	return "", 0, nil
}

// setMode sets the selection mode, returning an error on conflict.
func setMode(cfg *config, m cutMode, list string) error {
	if cfg.mode != modeNone {
		return fmt.Errorf("only one list may be specified")
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
	cfg := config{delimiter: '\t'}
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
	return validateConfig(cfg)
}

// validateConfig checks for conflicting or missing flags.
func validateConfig(cfg config) (config, error) {
	if cfg.mode == modeNone {
		return config{}, fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}
	if cfg.delimSet && cfg.mode != modeField {
		return config{}, fmt.Errorf("an input delimiter may be specified only when operating on fields")
	}
	if cfg.onlyDelimited && cfg.mode != modeField {
		return config{}, fmt.Errorf("suppressing non-delimited lines makes sense\n\tonly when operating on fields")
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg, nil
}

// parseFlag handles a single flag argument, returning extra args consumed.
func parseFlag(cfg *config, arg string, args []string, i int) (int, error) {
	if arg == "--complement" {
		cfg.complement = true
		return 0, nil
	}
	if arg == "-s" || arg == "--only-delimited" {
		cfg.onlyDelimited = true
		return 0, nil
	}
	if strings.HasPrefix(arg, "--output-delimiter") {
		return parseOutputDelimFlag(cfg, arg, args, i)
	}
	return parseModeOrDelimFlag(cfg, arg, args, i)
}

// parseOutputDelimFlag handles --output-delimiter=STRING and --output-delimiter STRING.
// R2.4: sets the output delimiter for all modes.
func parseOutputDelimFlag(cfg *config, arg string, args []string, i int) (int, error) {
	const prefix = "--output-delimiter="
	if strings.HasPrefix(arg, prefix) {
		cfg.outputDelim = arg[len(prefix):]
		cfg.outputDelimSet = true
		return 0, nil
	}
	if arg == "--output-delimiter" {
		if i+1 >= len(args) {
			return 0, fmt.Errorf("option '--output-delimiter' requires an argument")
		}
		cfg.outputDelim = args[i+1]
		cfg.outputDelimSet = true
		return 1, nil
	}
	return 0, fmt.Errorf("invalid option -- '%s'", strings.TrimLeft(arg, "-"))
}

// parseModeOrDelimFlag handles -d, -b, -c, and -f flags.
func parseModeOrDelimFlag(cfg *config, arg string, args []string, i int) (int, error) {
	val, skip, err := extractDelimFlag(arg, args, i)
	if err != nil {
		return 0, err
	}
	if val != "" {
		if len(val) != 1 {
			return 0, fmt.Errorf("the delimiter must be a single character")
		}
		cfg.delimiter = val[0]
		cfg.delimSet = true
		return skip, nil
	}
	return parseModeFlag(cfg, arg, args, i)
}

// parseModeFlag handles the selection mode flags -b, -c, -f.
func parseModeFlag(cfg *config, arg string, args []string, i int) (int, error) {
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
	val, skip, err = extractFieldFlag(arg, args, i)
	if err != nil {
		return 0, err
	}
	if val != "" {
		return skip, setMode(cfg, modeField, val)
	}
	return 0, fmt.Errorf("invalid option -- '%s'", strings.TrimLeft(arg, "-"))
}

// openInput returns os.Stdin for "-", otherwise opens the named file.
// R4.2: file open failures return an error; processing continues for remaining files.
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
// Capitalizes the first letter to match GNU coreutils formatting.
func formatOpenError(name string, err error) error {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Errorf("%s: %s", name, capitalizeFirst(pe.Err.Error()))
	}
	return fmt.Errorf("%s: %s", name, err)
}

// capitalizeFirst uppercases the first byte of s for GNU-compatible messages.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// isSelected returns whether a 1-indexed position is selected,
// accounting for the complement flag.
// R3.1: --complement inverts the selected positions.
func isSelected(pos int, ranges []cutRange, complement bool) bool {
	sel := byteSelected(pos, ranges)
	if complement {
		return !sel
	}
	return sel
}

// cutBytes processes a single reader in byte/character mode.
// R1.1: extract specified byte positions.
// R1.3: newlines pass through; not counted as part of line.
// R1.4: short lines produce only existing bytes.
func cutBytes(r io.Reader, w *bufio.Writer, cfg config) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if werr := writeByteLine(w, line, cfg); werr != nil {
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
// R2.4: when outputDelimSet, inserts output delimiter between disjoint groups.
func writeByteLine(w *bufio.Writer, line []byte, cfg config) error {
	content, _ := stripNewline(line)
	if cfg.outputDelimSet {
		return writeByteLineDelim(w, content, cfg)
	}
	return writeByteLineRaw(w, content, cfg)
}

// writeByteLineRaw writes selected bytes with no separator.
func writeByteLineRaw(w *bufio.Writer, content []byte, cfg config) error {
	for pos := 1; pos <= len(content); pos++ {
		if isSelected(pos, cfg.ranges, cfg.complement) {
			if err := w.WriteByte(content[pos-1]); err != nil {
				return err
			}
		}
	}
	return w.WriteByte('\n')
}

// writeByteLineDelim writes selected bytes with output delimiter between groups.
func writeByteLineDelim(w *bufio.Writer, content []byte, cfg config) error {
	firstGroup := true
	prevSelected := false
	for pos := 1; pos <= len(content); pos++ {
		sel := isSelected(pos, cfg.ranges, cfg.complement)
		if sel {
			if !prevSelected && !firstGroup {
				if _, err := w.WriteString(cfg.outputDelim); err != nil {
					return err
				}
			}
			if err := w.WriteByte(content[pos-1]); err != nil {
				return err
			}
			firstGroup = false
		}
		prevSelected = sel
	}
	return w.WriteByte('\n')
}

// cutFields processes a single reader in field mode.
// R2.1: extract fields delimited by the delimiter character.
func cutFields(r io.Reader, w *bufio.Writer, cfg config) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if werr := writeFieldLine(w, line, cfg); werr != nil {
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

// writeFieldLine writes the selected fields from a single line.
// R2.3: lines without the delimiter are printed unchanged unless -s is set.
func writeFieldLine(w *bufio.Writer, line []byte, cfg config) error {
	content, hasNewline := stripNewline(line)
	if bytes.IndexByte(content, cfg.delimiter) < 0 {
		if cfg.onlyDelimited {
			return nil
		}
		if _, err := w.Write(content); err != nil {
			return err
		}
		return writeNewlineIf(w, hasNewline)
	}
	return writeSelectedFields(w, content, cfg, hasNewline)
}

// stripNewline removes a trailing newline and reports whether one was present.
func stripNewline(line []byte) ([]byte, bool) {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		return line[:len(line)-1], true
	}
	return line, false
}

// writeNewlineIf writes a newline byte if cond is true.
func writeNewlineIf(w *bufio.Writer, cond bool) error {
	if cond {
		return w.WriteByte('\n')
	}
	return nil
}

// writeSelectedFields writes the fields matching the range selection.
// R2.2: output delimiter defaults to the input delimiter.
// R2.4: uses outputDelim when explicitly set via --output-delimiter.
// R3.3: complement with -f outputs fields not in the list.
func writeSelectedFields(w *bufio.Writer, content []byte, cfg config, hasNewline bool) error {
	fields := bytes.Split(content, []byte{cfg.delimiter})
	outDelim := string(cfg.delimiter)
	if cfg.outputDelimSet {
		outDelim = cfg.outputDelim
	}
	first := true
	for i, field := range fields {
		pos := i + 1
		if !isSelected(pos, cfg.ranges, cfg.complement) {
			continue
		}
		if !first {
			if _, err := w.WriteString(outDelim); err != nil {
				return err
			}
		}
		if _, err := w.Write(field); err != nil {
			return err
		}
		first = false
	}
	return writeNewlineIf(w, hasNewline)
}

// cutFile processes a single file.
// R4.2: returns error on file open failure; caller continues with remaining files.
func cutFile(name string, w *bufio.Writer, cfg config) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	if cfg.mode == modeField {
		return cutFields(r, w, cfg)
	}
	// R1.2: -c is equivalent to -b under LC_ALL=C
	return cutBytes(r, w, cfg)
}

// R4.4: SIGPIPE handler installed at start.
// R4.1: exit 0 on success.
// R4.2: exit 1 on file open failure, processing continues for remaining files.
// R4.3: exit 1 on write error.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'cut --help' for more information.\n")
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
