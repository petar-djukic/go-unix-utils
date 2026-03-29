// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cut implements GNU cut: remove sections from each line of files.
//
// Implements prd026-cut R1.1, R1.2, R1.3, R1.4 (byte and character selection),
// R2.1, R2.2 (field selection with delimiter), R2.3 (only-delimited),
// R2.4 (output delimiter), R3.1, R3.2, R3.3 (complement mode),
// R4.1, R4.2, R4.3, R4.4 (exit codes, SIGPIPE).
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

// selMode distinguishes byte, character, and field selection.
type selMode int

const (
	modeNone   selMode = iota
	modeBytes          // -b
	modeChars          // -c
	modeFields         // -f
)

// cutRange represents a single range in a LIST specification.
// start and end are 1-indexed. end == 0 means "to end of line".
// start == 0 means "from beginning" (i.e., -M form).
type cutRange struct {
	start int
	end   int
}

// cutOptions holds parsed flag state.
type cutOptions struct {
	mode           selMode
	ranges         []cutRange
	delimiter      byte
	complement     bool   // R3.1: invert selection
	onlyDelimited  bool   // R2.3: suppress lines without delimiter
	outputDelim    string // R2.4: output delimiter string
	outputDelimSet bool   // whether --output-delimiter was explicitly set
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags and processes input files.
// R4.1: exit 0 on success. R4.2: exit 1 on error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", programName)
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	return processFiles(files, stdin, stdout, stderr, opts)
}

// processFiles iterates over files and applies cut to each.
// R4.2: errors on individual files do not stop processing.
// R4.3: write errors cause exit 1.
func processFiles(files []string, stdin io.Reader, stdout, stderr io.Writer, opts cutOptions) int {
	exitCode := 0
	bw := bufio.NewWriter(stdout)
	for _, name := range files {
		if err := cutFile(name, stdin, bw, opts); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error\n", programName)
		return 1
	}
	return exitCode
}

// cutFile processes a single input file or stdin.
func cutFile(name string, stdin io.Reader, w *bufio.Writer, opts cutOptions) error {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	return cutLines(r, w, opts)
}

// openInput returns a reader and optional closer for the given filename.
// "-" means stdin.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: No such file or directory", name)
	}
	return f, f, nil
}

// cutLines reads lines from r and writes selected portions to w.
func cutLines(r io.Reader, w *bufio.Writer, opts cutOptions) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		result, suppress := selectFromLine(line, opts)
		if suppress {
			continue
		}
		if _, err := w.WriteString(result); err != nil {
			return err
		}
		if _, err := w.WriteString("\n"); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// selectFromLine applies the selection mode to a single line.
// Returns the result string and whether the line should be suppressed.
func selectFromLine(line string, opts cutOptions) (string, bool) {
	switch opts.mode {
	case modeBytes, modeChars:
		// R1.2: under LC_ALL=C, -c and -b are equivalent.
		return selectBytes(line, opts), false
	case modeFields:
		return selectFields(line, opts)
	default:
		return line, false
	}
}

// selectBytes extracts selected byte positions from a line.
// R1.1: byte positions are 1-indexed.
// R1.3: newlines are not counted; passed through by caller.
// R1.4: out-of-range positions produce no output.
// R3.1: --complement inverts the selection.
func selectBytes(line string, opts cutOptions) string {
	positions := expandRanges(opts.ranges, len(line))
	if opts.complement {
		positions = complementPositions(positions, len(line))
	}
	var b strings.Builder
	if opts.outputDelimSet {
		return joinBytePositions(line, positions, opts.outputDelim)
	}
	for _, pos := range positions {
		if pos >= 1 && pos <= len(line) {
			b.WriteByte(line[pos-1])
		}
	}
	return b.String()
}

// joinBytePositions joins selected byte positions with an output delimiter.
// D2: when --output-delimiter is set in byte/char mode, it separates ranges.
func joinBytePositions(line string, positions []int, delim string) string {
	var b strings.Builder
	for i, pos := range positions {
		if i > 0 {
			b.WriteString(delim)
		}
		if pos >= 1 && pos <= len(line) {
			b.WriteByte(line[pos-1])
		}
	}
	return b.String()
}

// selectFields extracts selected fields from a line.
// R2.1: fields are delimited by the delimiter character.
// R2.2: default delimiter is tab; output delimiter matches input.
// R2.3: -s suppresses lines without delimiter.
// R2.4: --output-delimiter replaces delimiter in output.
// R3.3: --complement outputs non-selected fields.
func selectFields(line string, opts cutOptions) (string, bool) {
	delim := string(opts.delimiter)
	if !strings.Contains(line, delim) {
		// R2.3: -s suppresses lines without delimiter.
		if opts.onlyDelimited {
			return "", true
		}
		// Without -s, lines without delimiter are printed unchanged.
		return line, false
	}
	fields := strings.Split(line, delim)
	positions := expandRanges(opts.ranges, len(fields))
	if opts.complement {
		positions = complementPositions(positions, len(fields))
	}
	selected := make([]string, 0, len(positions))
	for _, pos := range positions {
		if pos >= 1 && pos <= len(fields) {
			selected = append(selected, fields[pos-1])
		}
	}
	outDelim := delim
	if opts.outputDelimSet {
		outDelim = opts.outputDelim
	}
	return strings.Join(selected, outDelim), false
}

// complementPositions returns positions from 1..maxLen not in the given set.
// R3.1: inverts the selection for bytes, characters, or fields.
func complementPositions(positions []int, maxLen int) []int {
	selected := make(map[int]bool, len(positions))
	for _, p := range positions {
		selected[p] = true
	}
	result := make([]int, 0, maxLen)
	for i := 1; i <= maxLen; i++ {
		if !selected[i] {
			result = append(result, i)
		}
	}
	return result
}

// expandRanges converts a list of cutRange into a sorted, deduplicated
// list of 1-indexed positions, capped at maxLen.
func expandRanges(ranges []cutRange, maxLen int) []int {
	seen := make(map[int]bool)
	for _, r := range ranges {
		start, end := normalizeRange(r, maxLen)
		for i := start; i <= end; i++ {
			seen[i] = true
		}
	}
	positions := make([]int, 0, len(seen))
	for pos := range seen {
		positions = append(positions, pos)
	}
	sort.Ints(positions)
	return positions
}

// normalizeRange resolves a cutRange into concrete start/end positions.
func normalizeRange(r cutRange, maxLen int) (int, int) {
	start := r.start
	end := r.end
	if start == 0 {
		start = 1
	}
	if end == 0 || end > maxLen {
		end = maxLen
	}
	if start > maxLen {
		start = maxLen + 1
	}
	return start, end
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (cutOptions, []string, error) {
	opts := cutOptions{delimiter: '\t'}
	var files []string
	var listStr string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (arg != "-" && (len(arg) == 0 || arg[0] != '-')) {
			files = append(files, arg)
			continue
		}
		if arg == "-" {
			files = append(files, arg)
			continue
		}
		var err error
		i, listStr, err = parseFlag(&opts, &flagsDone, args, i, listStr)
		if err != nil {
			return opts, nil, err
		}
	}
	return finalizeParse(opts, listStr, files)
}

// finalizeParse validates final state and parses the range list.
func finalizeParse(opts cutOptions, listStr string, files []string) (cutOptions, []string, error) {
	if opts.mode == modeNone {
		return opts, nil, fmt.Errorf(
			"you must specify a list of bytes, characters, or fields")
	}
	if listStr == "" {
		return opts, nil, fmt.Errorf(
			"you must specify a list of bytes, characters, or fields")
	}
	ranges, err := parseRangeList(listStr)
	if err != nil {
		return opts, nil, err
	}
	opts.ranges = ranges
	return opts, files, nil
}

// parseFlag handles a single flag argument starting at args[i].
func parseFlag(opts *cutOptions, flagsDone *bool, args []string, i int, listStr string) (int, string, error) {
	arg := args[i]
	switch {
	case arg == "--":
		*flagsDone = true
	case arg == "-b" || arg == "--bytes":
		return setModeWithList(opts, modeBytes, args, i)
	case strings.HasPrefix(arg, "-b") && len(arg) > 2:
		return setModeInline(opts, modeBytes, arg[2:], i)
	case strings.HasPrefix(arg, "--bytes="):
		return setModeInline(opts, modeBytes, arg[8:], i)
	case arg == "-c" || arg == "--characters":
		return setModeWithList(opts, modeChars, args, i)
	case strings.HasPrefix(arg, "-c") && len(arg) > 2:
		return setModeInline(opts, modeChars, arg[2:], i)
	case strings.HasPrefix(arg, "--characters="):
		return setModeInline(opts, modeChars, arg[13:], i)
	case arg == "-f" || arg == "--fields":
		return setModeWithList(opts, modeFields, args, i)
	case strings.HasPrefix(arg, "-f") && len(arg) > 2:
		return setModeInline(opts, modeFields, arg[2:], i)
	case strings.HasPrefix(arg, "--fields="):
		return setModeInline(opts, modeFields, arg[9:], i)
	case arg == "-d":
		return parseDelimNext(opts, args, i, listStr)
	case strings.HasPrefix(arg, "-d") && len(arg) > 2:
		return parseDelimInline(opts, arg[2:], i, listStr)
	case strings.HasPrefix(arg, "--delimiter="):
		return parseDelimInline(opts, arg[12:], i, listStr)
	case arg == "-s" || arg == "--only-delimited":
		// R2.3: suppress lines without delimiter in field mode.
		opts.onlyDelimited = true
	case arg == "--complement":
		// R3.1: invert the selection.
		opts.complement = true
	case arg == "--output-delimiter":
		return parseOutputDelimNext(opts, args, i, listStr)
	case strings.HasPrefix(arg, "--output-delimiter="):
		return parseOutputDelimInline(opts, arg, i, listStr)
	default:
		return i, listStr, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
	return i, listStr, nil
}

// setModeWithList sets the selection mode and reads LIST from the next arg.
func setModeWithList(opts *cutOptions, mode selMode, args []string, i int) (int, string, error) {
	opts.mode = mode
	i++
	if i >= len(args) {
		return i, "", fmt.Errorf("option requires an argument -- '%s'", args[i-1])
	}
	return i, args[i], nil
}

// setModeInline sets the selection mode with an inline LIST value.
func setModeInline(opts *cutOptions, mode selMode, list string, i int) (int, string, error) {
	opts.mode = mode
	return i, list, nil
}

// parseDelimNext reads the delimiter from the next argument.
func parseDelimNext(opts *cutOptions, args []string, i int, listStr string) (int, string, error) {
	i++
	if i >= len(args) {
		return i, listStr, fmt.Errorf("option requires an argument -- 'd'")
	}
	return parseDelimValue(opts, args[i], i, listStr)
}

// parseDelimInline reads the delimiter from an inline value.
func parseDelimInline(opts *cutOptions, val string, i int, listStr string) (int, string, error) {
	return parseDelimValue(opts, val, i, listStr)
}

// parseDelimValue validates and sets the delimiter.
func parseDelimValue(opts *cutOptions, val string, i int, listStr string) (int, string, error) {
	if len(val) != 1 {
		return i, listStr, fmt.Errorf(
			"the delimiter must be a single character")
	}
	opts.delimiter = val[0]
	return i, listStr, nil
}

// parseOutputDelimNext reads the output delimiter from the next argument.
// R2.4: --output-delimiter STRING.
func parseOutputDelimNext(opts *cutOptions, args []string, i int, listStr string) (int, string, error) {
	i++
	if i >= len(args) {
		return i, listStr, fmt.Errorf("option requires an argument -- 'output-delimiter'")
	}
	opts.outputDelim = args[i]
	opts.outputDelimSet = true
	return i, listStr, nil
}

// parseOutputDelimInline reads the output delimiter from --output-delimiter=VAL.
// R2.4: --output-delimiter STRING.
func parseOutputDelimInline(opts *cutOptions, arg string, i int, listStr string) (int, string, error) {
	val := arg[len("--output-delimiter="):]
	opts.outputDelim = val
	opts.outputDelimSet = true
	return i, listStr, nil
}

// parseRangeList parses a comma-separated LIST of range specifications.
// R1.4: supports N, N-M, N-, -M, and comma-separated combinations.
func parseRangeList(list string) ([]cutRange, error) {
	parts := strings.Split(list, ",")
	ranges := make([]cutRange, 0, len(parts))
	for _, part := range parts {
		r, err := parseSingleRange(part)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

// parseSingleRange parses one range element: N, N-M, N-, or -M.
func parseSingleRange(s string) (cutRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return cutRange{}, fmt.Errorf("invalid range: ''")
	}
	idx := strings.Index(s, "-")
	if idx < 0 {
		return parseSinglePosition(s)
	}
	if idx == 0 {
		return parseToEnd(s[1:])
	}
	if idx == len(s)-1 {
		return parseFromStart(s[:idx])
	}
	return parseFullRange(s[:idx], s[idx+1:])
}

// parseSinglePosition parses a single position N.
func parseSinglePosition(s string) (cutRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	return cutRange{start: n, end: n}, nil
}

// parseToEnd parses the -M form (from 1 to M).
func parseToEnd(s string) (cutRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	return cutRange{start: 1, end: n}, nil
}

// parseFromStart parses the N- form (from N to end).
func parseFromStart(s string) (cutRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	return cutRange{start: n, end: 0}, nil
}

// parseFullRange parses the N-M form.
func parseFullRange(startStr, endStr string) (cutRange, error) {
	start, err := strconv.Atoi(startStr)
	if err != nil || start <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	end, err := strconv.Atoi(endStr)
	if err != nil || end <= 0 {
		return cutRange{}, fmt.Errorf("invalid byte, character or field list")
	}
	if start > end {
		return cutRange{}, fmt.Errorf(
			"invalid decreasing range")
	}
	return cutRange{start: start, end: end}, nil
}
