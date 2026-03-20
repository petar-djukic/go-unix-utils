// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd026-cut R1.1–R1.4, R2.1–R2.4, R3.1–R3.3.
// R1.1: -b LIST extracts byte positions using range syntax.
// R1.2: -c LIST extracts character positions (equivalent to -b under LC_ALL=C).
// R1.3: Newlines pass through unchanged; not counted as line content.
// R1.4: Out-of-range positions produce no output bytes.
// R2.1: -f LIST extracts fields delimited by -d (default tab).
// R2.2: -d DELIM sets input field delimiter (single byte).
// R2.3: -s suppresses lines without the delimiter character.
// R2.4: --output-delimiter STRING replaces the input delimiter in output.
// R3.1: --complement inverts the selection (output what was NOT selected).
// R3.2: --complement is compatible with -b, -c, and -f.
// R3.3: With --complement and -f, unselected fields output in original order.
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

// cutMode distinguishes between byte/char and field selection.
type cutMode int

const (
	modeBytes  cutMode = iota // -b or -c
	modeFields                // -f
)

// byteRange represents an inclusive 1-indexed range of bytes/characters/fields.
type byteRange struct {
	low  int // 1-indexed inclusive start
	high int // 1-indexed inclusive end; math.MaxInt for open-ended
}

// cutConfig holds the parsed flags for this invocation.
type cutConfig struct {
	mode           cutMode
	ranges         []byteRange
	files          []string
	delimiter      byte   // R2.2: input delimiter (default '\t')
	outputDelim    string // R2.4: output delimiter; empty means use input delimiter
	outputDelimSet bool   // whether --output-delimiter was explicitly provided
	suppress       bool   // R2.3: suppress lines without delimiter
	complement     bool   // R3.1: invert the selection
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
		processReader(w, os.Stdin, cfg)
	} else {
		exitCode = processFiles(w, cfg)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cut: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFiles iterates over file arguments and processes each.
func processFiles(w *bufio.Writer, cfg cutConfig) int {
	exitCode := 0
	for _, name := range cfg.files {
		if err := processFile(w, name, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "cut: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a single file (or stdin for "-") and processes it.
func processFile(w *bufio.Writer, name string, cfg cutConfig) error {
	if name == "-" {
		processReader(w, os.Stdin, cfg)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	defer f.Close()
	processReader(w, f, cfg)
	return nil
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// processReader reads lines from r and dispatches to byte or field mode.
func processReader(w *bufio.Writer, r io.Reader, cfg cutConfig) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		switch cfg.mode {
		case modeBytes:
			cutBytesLine(w, line, cfg)
			w.WriteByte('\n')
		case modeFields:
			cutFields(w, line, cfg)
		}
	}
}

// cutBytesLine dispatches byte cutting, handling complement. R1.1, R3.1.
func cutBytesLine(w *bufio.Writer, line []byte, cfg cutConfig) {
	if cfg.complement {
		cutBytesComplement(w, line, cfg.ranges)
	} else {
		cutBytes(w, line, cfg.ranges)
	}
}

// cutBytes extracts the selected byte ranges from a single line. R1.1, R1.4.
func cutBytes(w *bufio.Writer, line []byte, ranges []byteRange) {
	lineLen := len(line)
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
		w.Write(line[lo:hi])
	}
}

// cutBytesComplement outputs bytes NOT in the selected ranges. R3.1.
func cutBytesComplement(w *bufio.Writer, line []byte, ranges []byteRange) {
	lineLen := len(line)
	selected := buildSelectedSet(ranges, lineLen)
	for i := 0; i < lineLen; i++ {
		if !selected[i] {
			w.WriteByte(line[i])
		}
	}
}

// buildSelectedSet returns a boolean slice marking selected 0-indexed positions.
func buildSelectedSet(ranges []byteRange, length int) []bool {
	selected := make([]bool, length)
	for _, r := range ranges {
		lo := r.low - 1
		hi := r.high
		if lo < 0 {
			lo = 0
		}
		if hi > length {
			hi = length
		}
		for i := lo; i < hi; i++ {
			selected[i] = true
		}
	}
	return selected
}

// cutFields extracts selected fields from a line. R2.1, R2.3, R3.1.
func cutFields(w *bufio.Writer, line []byte, cfg cutConfig) {
	delim := cfg.delimiter
	if !containsByte(line, delim) {
		if !cfg.suppress {
			w.Write(line)
			w.WriteByte('\n')
		}
		return
	}
	fields := splitFields(line, delim)
	outDelim := cfg.effectiveOutputDelim()
	if cfg.complement {
		writeComplementFields(w, fields, cfg.ranges, outDelim)
	} else {
		writeSelectedFields(w, fields, cfg.ranges, outDelim)
	}
	w.WriteByte('\n')
}

// containsByte checks whether b contains the byte c.
func containsByte(b []byte, c byte) bool {
	for _, v := range b {
		if v == c {
			return true
		}
	}
	return false
}

// splitFields splits line by single-byte delimiter into fields.
func splitFields(line []byte, delim byte) []string {
	return strings.Split(string(line), string(delim))
}

// writeSelectedFields writes the fields at selected positions. R2.1, R2.4.
func writeSelectedFields(
	w *bufio.Writer, fields []string, ranges []byteRange, outDelim string,
) {
	nFields := len(fields)
	first := true
	for _, r := range ranges {
		lo := r.low
		hi := r.high
		if hi > nFields {
			hi = nFields
		}
		for idx := lo; idx <= hi; idx++ {
			if idx < 1 || idx > nFields {
				continue
			}
			if !first {
				w.WriteString(outDelim)
			}
			w.WriteString(fields[idx-1])
			first = false
		}
	}
}

// writeComplementFields writes fields NOT in the range list. R3.1, R3.3.
func writeComplementFields(
	w *bufio.Writer, fields []string, ranges []byteRange, outDelim string,
) {
	nFields := len(fields)
	selected := buildFieldSelectedSet(ranges, nFields)
	first := true
	for i := 0; i < nFields; i++ {
		if selected[i] {
			continue
		}
		if !first {
			w.WriteString(outDelim)
		}
		w.WriteString(fields[i])
		first = false
	}
}

// buildFieldSelectedSet returns a boolean slice for selected 0-indexed fields.
func buildFieldSelectedSet(ranges []byteRange, nFields int) []bool {
	selected := make([]bool, nFields)
	for _, r := range ranges {
		lo := r.low - 1
		hi := r.high
		if lo < 0 {
			lo = 0
		}
		if hi > nFields {
			hi = nFields
		}
		for i := lo; i < hi; i++ {
			selected[i] = true
		}
	}
	return selected
}

// effectiveOutputDelim returns the output delimiter string. R2.2, R2.4.
func (c cutConfig) effectiveOutputDelim() string {
	if c.outputDelimSet {
		return c.outputDelim
	}
	return string(c.delimiter)
}

// parseArgs extracts config from command-line arguments.
func parseArgs(args []string) (cutConfig, error) {
	var cfg parsedFlags
	cfg.delimiter = '\t' // R2.2: default is tab
	parseAllFlags(args, &cfg)
	return buildConfig(cfg)
}

// parsedFlags holds intermediate flag parsing state.
type parsedFlags struct {
	byteList       string
	charList       string
	fieldList      string
	delimiter      byte
	delimSet       bool
	outputDelim    string
	outputDelimSet bool
	suppress       bool
	complement     bool
	files          []string
}

// parseAllFlags processes all command-line arguments into parsedFlags.
func parseAllFlags(args []string, cfg *parsedFlags) {
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || (!strings.HasPrefix(arg, "-") || arg == "-") {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		parseSingleFlag(arg, args, &i, cfg)
	}
}

// parseSingleFlag handles one flag argument. R1, R2, R3.
func parseSingleFlag(
	arg string, args []string, i *int, cfg *parsedFlags,
) {
	if val, ok := extractFlagVal(arg, args, i, "-b", "--bytes="); ok {
		cfg.byteList = val
		return
	}
	if val, ok := extractFlagVal(arg, args, i, "-c", "--characters="); ok {
		cfg.charList = val
		return
	}
	if val, ok := extractFlagVal(arg, args, i, "-f", "--fields="); ok {
		cfg.fieldList = val
		return
	}
	if val, ok := extractFlagVal(arg, args, i, "-d", "--delimiter="); ok {
		parseDelimiter(val, cfg)
		return
	}
	if val, ok := extractLongFlag(arg, "--output-delimiter="); ok {
		cfg.outputDelim = val
		cfg.outputDelimSet = true
		return
	}
	parseBooleanFlags(arg, cfg)
}

// parseBooleanFlags handles boolean flag arguments (-s, --complement). R2.3, R3.1.
func parseBooleanFlags(arg string, cfg *parsedFlags) {
	if arg == "-s" || arg == "--only-delimited" {
		cfg.suppress = true
		return
	}
	if arg == "--complement" {
		cfg.complement = true
	}
}

// parseDelimiter validates and sets the delimiter. R2.2.
func parseDelimiter(val string, cfg *parsedFlags) {
	if len(val) == 1 {
		cfg.delimiter = val[0]
		cfg.delimSet = true
	}
}

// extractLongFlag checks for --long=VALUE form only.
func extractLongFlag(arg, prefix string) (string, bool) {
	if val, ok := strings.CutPrefix(arg, prefix); ok {
		return val, true
	}
	return "", false
}

// extractFlagVal checks if arg matches -X or --long= form and returns value.
func extractFlagVal(
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

// buildConfig validates parsed flags and constructs cutConfig.
func buildConfig(cfg parsedFlags) (cutConfig, error) {
	listStr, mode, err := selectMode(cfg)
	if err != nil {
		return cutConfig{}, err
	}
	ranges, err := parseRangeList(listStr)
	if err != nil {
		return cutConfig{}, err
	}
	return cutConfig{
		mode:           mode,
		ranges:         ranges,
		files:          cfg.files,
		delimiter:      cfg.delimiter,
		outputDelim:    cfg.outputDelim,
		outputDelimSet: cfg.outputDelimSet,
		suppress:       cfg.suppress,
		complement:     cfg.complement,
	}, nil
}

// selectMode determines which mode (-b, -c, -f) was requested.
func selectMode(cfg parsedFlags) (string, cutMode, error) {
	if cfg.byteList != "" {
		return cfg.byteList, modeBytes, nil
	}
	if cfg.charList != "" {
		return cfg.charList, modeBytes, nil
	}
	if cfg.fieldList != "" {
		return cfg.fieldList, modeFields, nil
	}
	return "", 0, fmt.Errorf(
		"you must specify a list of bytes, characters, or fields")
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
		return byteRange{}, fmt.Errorf(
			"invalid byte/character position %q", s)
	}
	return byteRange{low: n, high: n}, nil
}

// parseEndRange parses "-M" as range [1,M].
func parseEndRange(s string) (byteRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return byteRange{}, fmt.Errorf(
			"invalid byte/character position %q", s)
	}
	return byteRange{low: 1, high: n}, nil
}

// parseStartRange parses "N-" as range [N,max].
func parseStartRange(s string) (byteRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return byteRange{}, fmt.Errorf(
			"invalid byte/character position %q", s)
	}
	return byteRange{low: n, high: math.MaxInt}, nil
}

// parseFullRange parses "N-M" as range [N,M].
func parseFullRange(lo, hi string) (byteRange, error) {
	n, err := strconv.Atoi(lo)
	if err != nil || n <= 0 {
		return byteRange{}, fmt.Errorf(
			"invalid byte/character position %q", lo)
	}
	m, err := strconv.Atoi(hi)
	if err != nil || m <= 0 {
		return byteRange{}, fmt.Errorf(
			"invalid byte/character position %q", hi)
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
