// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: sort [OPTION]... [FILE]...
Write sorted concatenation of all FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

Mandatory arguments to long options are mandatory for short options too.
Ordering options:

  -h, --human-numeric-sort    compare human readable numbers (e.g., 2K 1G)
  -M, --month-sort            compare (unknown) < 'JAN' < ... < 'DEC'
  -n, --numeric-sort          compare according to string numerical value
  -V, --version-sort          natural sort of (version) numbers within text
  -r, --reverse               reverse the result of comparisons

Other options:

  -o, --output=FILE           write result to FILE instead of standard output
  -s, --stable                stabilize sort by disabling last-resort comparison
  -t, --field-separator=SEP   use SEP instead of non-blank to blank transition
  -u, --unique                with -c, check for strict ordering;
                                without -c, output only the first of an equal run
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `sort (go-unix-utils) dev
`

type sortMode int

const (
	sortDefault sortMode = iota
	sortNumeric
	sortHumanNumeric
	sortMonth
	sortVersion
)

type options struct {
	reverse        bool
	unique         bool
	stable         bool
	mode           sortMode
	outputFile     string
	fieldSeparator string
}

var monthMap = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %s\n", err)
		fmt.Fprintf(os.Stderr, "Try 'sort --help' for more information.\n")
		os.Exit(2)
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	lines, err := readAllLines(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %s\n", err)
		os.Exit(2)
	}

	lines = sortAndFilter(lines, opts)

	var w io.Writer = os.Stdout
	if opts.outputFile != "" {
		f, err := os.Create(opts.outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sort: %s\n", err)
			os.Exit(2)
		}
		defer f.Close()
		w = f
	}
	writeLines(lines, w)
}

func parseArgs(args []string) (options, []string, error) {
	var opts options
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, &opts, args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			extra, err := parseShortFlags(arg[1:], &opts, args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += 1 + extra
			continue
		}
		files = append(files, arg)
		i++
	}

	return opts, files, nil
}

func parseLongFlag(flag string, opts *options, remaining []string) (int, error) {
	if v, ok := strings.CutPrefix(flag, "--output="); ok {
		opts.outputFile = v
		return 1, nil
	}
	if v, ok := strings.CutPrefix(flag, "--field-separator="); ok {
		if len(v) != 1 {
			return 0, fmt.Errorf("multi-character tab '%s'", v)
		}
		opts.fieldSeparator = v
		return 1, nil
	}
	switch flag {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	case "--reverse":
		opts.reverse = true
	case "--unique":
		opts.unique = true
	case "--stable":
		opts.stable = true
	case "--numeric-sort":
		opts.mode = sortNumeric
	case "--human-numeric-sort":
		opts.mode = sortHumanNumeric
	case "--month-sort":
		opts.mode = sortMonth
	case "--version-sort":
		opts.mode = sortVersion
	case "--output", "--field-separator":
		return parseLongWithArg(flag, opts, remaining)
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 1, nil
}

func parseLongWithArg(flag string, opts *options, remaining []string) (int, error) {
	if len(remaining) == 0 {
		return 0, fmt.Errorf("option '%s' requires an argument", flag)
	}
	switch flag {
	case "--output":
		opts.outputFile = remaining[0]
	case "--field-separator":
		if len(remaining[0]) != 1 {
			return 0, fmt.Errorf("multi-character tab '%s'", remaining[0])
		}
		opts.fieldSeparator = remaining[0]
	}
	return 2, nil
}

func parseShortFlags(flags string, opts *options, remaining []string) (int, error) {
	for idx := 0; idx < len(flags); idx++ {
		switch flags[idx] {
		case 'r':
			opts.reverse = true
		case 'u':
			opts.unique = true
		case 's':
			opts.stable = true
		case 'n':
			opts.mode = sortNumeric
		case 'h':
			opts.mode = sortHumanNumeric
		case 'M':
			opts.mode = sortMonth
		case 'V':
			opts.mode = sortVersion
		case 'o':
			rest := flags[idx+1:]
			if rest != "" {
				opts.outputFile = rest
				return 0, nil
			}
			if len(remaining) == 0 {
				return 0, fmt.Errorf("option requires an argument -- 'o'")
			}
			opts.outputFile = remaining[0]
			return 1, nil
		case 't':
			return parseFieldSep(flags[idx+1:], remaining, opts)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[idx])
		}
	}
	return 0, nil
}

func parseFieldSep(rest string, remaining []string, opts *options) (int, error) {
	if rest != "" {
		if len(rest) != 1 {
			return 0, fmt.Errorf("multi-character tab '%s'", rest)
		}
		opts.fieldSeparator = rest
		return 0, nil
	}
	if len(remaining) == 0 {
		return 0, fmt.Errorf("option requires an argument -- 't'")
	}
	if len(remaining[0]) != 1 {
		return 0, fmt.Errorf("multi-character tab '%s'", remaining[0])
	}
	opts.fieldSeparator = remaining[0]
	return 1, nil
}

func readAllLines(files []string) ([]string, error) {
	var lines []string
	for _, file := range files {
		r, closer, err := openInput(file)
		if err != nil {
			return nil, err
		}
		fileLines, err := readLines(r)
		if closer != nil {
			closer.Close()
		}
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func sortAndFilter(lines []string, opts options) []string {
	keyCmp := buildKeyCmp(opts)

	fullCmp := keyCmp
	if opts.mode != sortDefault && !opts.stable {
		fullCmp = func(a, b string) int {
			if r := keyCmp(a, b); r != 0 {
				return r
			}
			return strings.Compare(a, b)
		}
	}

	sortCmp := fullCmp
	if opts.reverse {
		sortCmp = func(a, b string) int { return fullCmp(b, a) }
	}

	if opts.stable {
		slices.SortStableFunc(lines, sortCmp)
	} else {
		slices.SortFunc(lines, sortCmp)
	}

	if opts.unique {
		lines = dedup(lines, keyCmp)
	}

	return lines
}

func buildKeyCmp(opts options) func(string, string) int {
	switch opts.mode {
	case sortHumanNumeric:
		return cmpFloat(parseHumanNumeric)
	case sortMonth:
		return func(a, b string) int { return monthRank(a) - monthRank(b) }
	case sortVersion:
		return compareVersion
	case sortNumeric:
		return cmpFloat(parseNumeric)
	default:
		return strings.Compare
	}
}

func cmpFloat(parse func(string) float64) func(string, string) int {
	return func(a, b string) int {
		na, nb := parse(a), parse(b)
		if na < nb {
			return -1
		}
		if na > nb {
			return 1
		}
		return 0
	}
}

func parseNumericEnd(s string) (float64, int) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	start := i
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i == start || (i == start+1 && (s[start] == '-' || s[start] == '+')) {
		return 0, i
	}
	val, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil {
		return 0, i
	}
	return val, i
}

func parseNumeric(s string) float64 {
	val, _ := parseNumericEnd(s)
	return val
}

func parseHumanNumeric(s string) float64 {
	val, end := parseNumericEnd(s)
	if end < len(s) {
		if m := suffixMultiplier(s[end]); m > 0 {
			return val * m
		}
	}
	return val
}

func suffixMultiplier(c byte) float64 {
	switch c {
	case 'K', 'k':
		return 1 << 10
	case 'M':
		return 1 << 20
	case 'G':
		return 1 << 30
	case 'T':
		return 1 << 40
	case 'P':
		return 1 << 50
	case 'E':
		return 1 << 60
	case 'Z':
		return 1 << 70
	case 'Y':
		return 1 << 80
	default:
		return 0
	}
}

func monthRank(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	end := min(i+3, len(s))
	return monthMap[strings.ToUpper(s[i:end])]
}

func compareVersion(a, b string) int {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		da := a[ia] >= '0' && a[ia] <= '9'
		db := b[ib] >= '0' && b[ib] <= '9'
		if da && db {
			r := cmpNumSegments(a, b, &ia, &ib)
			if r != 0 {
				return r
			}
			continue
		}
		if a[ia] != b[ib] {
			return int(a[ia]) - int(b[ib])
		}
		ia++
		ib++
	}
	return len(a) - len(b)
}

func cmpNumSegments(a, b string, ia, ib *int) int {
	startA, startB := *ia, *ib
	for *ia < len(a) && a[*ia] == '0' {
		*ia++
	}
	for *ib < len(b) && b[*ib] == '0' {
		*ib++
	}
	sigA, sigB := *ia, *ib
	endA := sigA
	for endA < len(a) && a[endA] >= '0' && a[endA] <= '9' {
		endA++
	}
	endB := sigB
	for endB < len(b) && b[endB] >= '0' && b[endB] <= '9' {
		endB++
	}
	lenA, lenB := endA-sigA, endB-sigB
	*ia = endA
	*ib = endB
	if lenA != lenB {
		return lenA - lenB
	}
	for k := range lenA {
		if a[sigA+k] != b[sigB+k] {
			return int(a[sigA+k]) - int(b[sigB+k])
		}
	}
	return (sigB - startB) - (sigA - startA)
}

func dedup(lines []string, cmp func(string, string) int) []string {
	if len(lines) == 0 {
		return lines
	}
	result := []string{lines[0]}
	for _, line := range lines[1:] {
		if cmp(line, result[len(result)-1]) != 0 {
			result = append(result, line)
		}
	}
	return result
}

func writeLines(lines []string, w io.Writer) {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		fmt.Fprintln(bw, line)
	}
	bw.Flush()
}
