// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sort implements GNU sort: sort lines of text files.
//
// Implements prd053-sort R1.1-R1.7, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sortMode selects the comparison algorithm.
type sortMode int

const (
	modeLexicographic sortMode = iota
	modeNumeric                // R2.1: -n
	modeHumanNumeric           // R2.2: -h
	modeMonth                  // R2.3: -M
	modeVersion                // R2.4: -V
)

// monthRank maps three-letter uppercase month abbreviations to sort rank.
// R2.3: unknown strings sort before JAN (rank 0).
var monthRank = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

// humanSuffix maps SI suffix characters to their power-of-1024 exponent.
var humanSuffix = map[byte]float64{
	'K': 1, 'k': 1, 'M': 2, 'G': 3, 'T': 4,
	'P': 5, 'E': 6, 'Z': 7, 'Y': 8,
}

// sortOptions holds parsed flag state.
type sortOptions struct {
	reverse      bool      // R1.4: -r: reverse sort order
	unique       bool      // R1.5: -u: output only first of equal run
	stable       bool      // R1.7: -s: preserve input order of equal lines
	outputFile   string    // R1.6: -o FILE: write output to file
	mode         sortMode  // R2.x: comparison mode
	fieldSep     string    // R3.1: -t CHAR: field delimiter
	keys         []keySpec // R3.2, R3.3: -k KEYDEF
	ignoreBlanks bool      // R3.4: -b: ignore leading blanks
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags, reads input, sorts, and writes output.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "sort: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	lines, exitCode := readAllLines(files, stdin, stderr)
	sortLines(lines, opts)
	if opts.unique {
		lines = dedup(lines, opts)
	}
	if err := writeOutput(opts.outputFile, stdout, lines); err != nil {
		fmt.Fprintf(stderr, "sort: %v\n", err)
		return 2
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (sortOptions, []string, error) {
	var opts sortOptions
	var files []string
	flagsDone := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if flagsDone || arg == "-" || (len(arg) > 0 && arg[0] != '-') {
			files = append(files, arg)
			i++
			continue
		}
		if arg == "--" {
			flagsDone = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			consumed, err := parseLongFlag(&opts, arg[2:], args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += 1 + consumed
			continue
		}
		consumed, err := parseShortFlags(&opts, arg[1:], args[i+1:])
		if err != nil {
			return opts, nil, err
		}
		i += 1 + consumed
	}
	return opts, files, nil
}

// parseLongFlag handles a single --name or --name=value long option.
func parseLongFlag(opts *sortOptions, name string, rest []string) (int, error) {
	if idx := strings.IndexByte(name, '='); idx >= 0 {
		return parseLongWithValue(opts, name[:idx], name[idx+1:])
	}
	return parseLongNoValue(opts, name, rest)
}

// parseLongWithValue handles --key=value flags.
func parseLongWithValue(opts *sortOptions, key, val string) (int, error) {
	switch key {
	case "output":
		opts.outputFile = val
	case "field-separator":
		opts.fieldSep = val
	case "key":
		k, err := parseKeyDef(val)
		if err != nil {
			return 0, err
		}
		opts.keys = append(opts.keys, k)
	default:
		return 0, fmt.Errorf("unrecognized option '--%s'", key)
	}
	return 0, nil
}

// parseLongNoValue handles long flags without = value.
func parseLongNoValue(opts *sortOptions, name string, rest []string) (int, error) {
	switch name {
	case "reverse":
		opts.reverse = true
	case "unique":
		opts.unique = true
	case "stable":
		opts.stable = true
	case "numeric-sort":
		opts.mode = modeNumeric
	case "human-numeric-sort":
		opts.mode = modeHumanNumeric
	case "month-sort":
		opts.mode = modeMonth
	case "version-sort":
		opts.mode = modeVersion
	case "ignore-leading-blanks":
		opts.ignoreBlanks = true
	case "output", "field-separator", "key":
		return parseLongArgFlag(opts, name, rest)
	default:
		return 0, fmt.Errorf("unrecognized option '--%s'", name)
	}
	return 0, nil
}

// parseLongArgFlag handles long flags that require a separate argument.
func parseLongArgFlag(opts *sortOptions, name string, rest []string) (int, error) {
	if len(rest) == 0 {
		return 0, fmt.Errorf("option '--%s' requires an argument", name)
	}
	switch name {
	case "output":
		opts.outputFile = rest[0]
	case "field-separator":
		opts.fieldSep = rest[0]
	case "key":
		k, err := parseKeyDef(rest[0])
		if err != nil {
			return 0, err
		}
		opts.keys = append(opts.keys, k)
	}
	return 1, nil
}

// parseShortFlags processes flag characters from a single -xyz argument.
// Returns the number of extra args consumed from rest.
func parseShortFlags(opts *sortOptions, chars string, rest []string) (int, error) {
	for idx, ch := range chars {
		switch ch {
		case 'r':
			opts.reverse = true
		case 'u':
			opts.unique = true
		case 's':
			opts.stable = true
		case 'b':
			opts.ignoreBlanks = true
		case 'o':
			return consumeFlagArg(chars[idx+1:], rest, &opts.outputFile, 'o')
		case 't':
			return consumeFlagArg(chars[idx+1:], rest, &opts.fieldSep, 't')
		case 'k':
			return consumeKeyFlag(chars[idx+1:], rest, opts)
		case 'n':
			opts.mode = modeNumeric
		case 'h':
			opts.mode = modeHumanNumeric
		case 'M':
			opts.mode = modeMonth
		case 'V':
			opts.mode = modeVersion
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}

// consumeFlagArg extracts the argument for a flag that requires a value.
// remaining is the rest of the short-flag cluster after the flag char.
func consumeFlagArg(
	remaining string, rest []string, dest *string, flag byte,
) (int, error) {
	if remaining != "" {
		*dest = remaining
		return 0, nil
	}
	if len(rest) == 0 {
		return 0, fmt.Errorf("option requires an argument -- '%c'", flag)
	}
	*dest = rest[0]
	return 1, nil
}

// consumeKeyFlag extracts and parses a -k KEYDEF argument.
func consumeKeyFlag(
	remaining string, rest []string, opts *sortOptions,
) (int, error) {
	var keyStr string
	consumed, err := consumeFlagArg(remaining, rest, &keyStr, 'k')
	if err != nil {
		return 0, err
	}
	k, err := parseKeyDef(keyStr)
	if err != nil {
		return 0, err
	}
	opts.keys = append(opts.keys, k)
	return consumed, nil
}

// readAllLines reads lines from all files, combining into a single slice.
// R1.2: "-" means stdin. R1.3: multiple files merged.
func readAllLines(
	files []string, stdin io.Reader, stderr io.Writer,
) ([]string, int) {
	var lines []string
	exitCode := 0
	for _, name := range files {
		fileLines, err := readFileLines(name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "sort: cannot read: %s: %v\n",
				name, unwrapErr(err))
			exitCode = 2
			continue
		}
		lines = append(lines, fileLines...)
	}
	return lines, exitCode
}

// readFileLines reads all lines from a single file or stdin.
func readFileLines(name string, stdin io.Reader) ([]string, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer.Close()
	}
	return scanLines(r)
}

// openInput returns a reader and optional closer for the given filename.
// R1.2: "-" reads from stdin.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// scanLines reads all lines from a reader using a buffered scanner.
func scanLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// compareLines compares two lines using keys if specified, else global mode.
// R3.3: key-based comparison; falls back to mode-based for no keys.
func compareLines(a, b string, opts sortOptions) int {
	if len(opts.keys) > 0 {
		return compareByKeys(a, b, opts)
	}
	if opts.ignoreBlanks {
		a = strings.TrimLeft(a, " \t")
		b = strings.TrimLeft(b, " \t")
	}
	return compareLine(a, b, opts.mode)
}

// compareLine compares two lines using the active sort mode.
// Returns negative if a < b, 0 if equal, positive if a > b.
func compareLine(a, b string, mode sortMode) int {
	switch mode {
	case modeNumeric:
		return compareNumeric(a, b)
	case modeHumanNumeric:
		return compareHumanNumeric(a, b)
	case modeMonth:
		return compareMonth(a, b)
	case modeVersion:
		return compareVersion(a, b)
	default:
		return strings.Compare(a, b)
	}
}

// sortLines sorts lines using the comparison determined by opts.
// R1.1: default lexicographic. R1.4: -r reverses. R1.7: -s stable.
func sortLines(lines []string, opts sortOptions) {
	less := func(i, j int) bool {
		cmp := compareLines(lines[i], lines[j], opts)
		if cmp == 0 && !opts.stable && !opts.unique {
			cmp = strings.Compare(lines[i], lines[j])
		}
		if opts.reverse {
			return cmp > 0
		}
		return cmp < 0
	}
	sort.SliceStable(lines, less)
}

// dedup removes consecutive equal lines based on the active sort comparison.
// R1.5: equality uses the primary comparison only (no last-resort).
func dedup(lines []string, opts sortOptions) []string {
	if len(lines) == 0 {
		return lines
	}
	result := lines[:1]
	for _, line := range lines[1:] {
		if compareLines(line, result[len(result)-1], opts) != 0 {
			result = append(result, line)
		}
	}
	return result
}

// --- R2.1: Numeric sort ---

// compareNumeric compares two strings by their leading numeric value.
// R2.1: parses leading whitespace and optional sign.
func compareNumeric(a, b string) int {
	va, _ := parseNumber(a)
	vb, _ := parseNumber(b)
	return compareFloat(va, vb)
}

// parseNumber extracts a leading numeric value from a string.
// Returns the parsed value and the index after the number.
// Skips leading blanks, parses optional sign, digits, and decimal.
func parseNumber(s string) (float64, int) {
	i := skipBlanks(s)
	start := i
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	digitStart := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	if i == digitStart && (i == start || i == start+1) {
		return 0, i
	}
	val, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil {
		return 0, i
	}
	return val, i
}

// --- R2.2: Human-numeric sort ---

// compareHumanNumeric compares two strings by numeric value with SI suffixes.
func compareHumanNumeric(a, b string) int {
	return compareFloat(parseHumanValue(a), parseHumanValue(b))
}

// parseHumanValue extracts a numeric value with optional SI suffix.
// R2.2: K, M, G, T, P, E, Z, Y suffixes (base 1024).
func parseHumanValue(s string) float64 {
	val, i := parseNumber(s)
	if i < len(s) {
		if exp, ok := humanSuffix[s[i]]; ok {
			val *= math.Pow(1024, exp)
		}
	}
	return val
}

// --- R2.3: Month sort ---

// compareMonth compares two strings by month name abbreviation.
// R2.3: JAN < FEB < ... < DEC. Unknown strings sort before JAN.
func compareMonth(a, b string) int {
	return parseMonthRank(a) - parseMonthRank(b)
}

// parseMonthRank extracts the month rank from a string.
// Skips leading blanks, takes first 3 chars uppercased, looks up rank.
func parseMonthRank(s string) int {
	i := skipBlanks(s)
	if i+3 > len(s) {
		return 0
	}
	abbrev := strings.ToUpper(s[i : i+3])
	if rank, ok := monthRank[abbrev]; ok {
		return rank
	}
	return 0
}

// --- R2.4: Version sort ---

// compareVersion compares two strings using natural version-number sort.
// R2.4: digit sequences are compared numerically, non-digits lexicographically.
func compareVersion(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigit(a[i]) && isDigit(b[j]) {
			cmp := cmpDigitRuns(a, b, i, j)
			if cmp != 0 {
				return cmp
			}
			i = advanceDigits(a, i)
			j = advanceDigits(b, j)
			continue
		}
		if a[i] != b[j] {
			return int(a[i]) - int(b[j])
		}
		i++
		j++
	}
	return (len(a) - i) - (len(b) - j)
}

// cmpDigitRuns compares digit sequences at positions ai and bj numerically.
func cmpDigitRuns(a, b string, ai, bj int) int {
	ae := advanceDigits(a, ai)
	be := advanceDigits(b, bj)
	a0, b0 := ai, bj
	for a0 < ae && a[a0] == '0' {
		a0++
	}
	for b0 < be && b[b0] == '0' {
		b0++
	}
	sigA, sigB := ae-a0, be-b0
	if sigA != sigB {
		if sigA < sigB {
			return -1
		}
		return 1
	}
	for k := 0; k < sigA; k++ {
		if a[a0+k] != b[b0+k] {
			return int(a[a0+k]) - int(b[b0+k])
		}
	}
	return 0
}

// --- Helpers ---

// skipBlanks returns the index of the first non-blank character.
func skipBlanks(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// advanceDigits returns the index after the digit run starting at from.
func advanceDigits(s string, from int) int {
	i := from
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// isDigit reports whether b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// compareFloat returns -1, 0, or 1 for float comparison.
func compareFloat(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// writeOutput writes lines to the output file or stdout.
// R1.6: -o FILE writes to file; FILE may be the same as an input file.
func writeOutput(path string, stdout io.Writer, lines []string) error {
	if path == "" {
		return writeLines(stdout, lines)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeLines(f, lines)
}

// writeLines writes all sorted lines to the writer, each followed by newline.
func writeLines(w io.Writer, lines []string) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// unwrapErr extracts the underlying syscall error from os.PathError.
func unwrapErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
