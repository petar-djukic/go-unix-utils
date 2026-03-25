// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd068-csplit: Split a File into Context-Determined Pieces.
// Covers R1.1-R1.4 (pattern-based splitting, integer/regexp/skip patterns),
// R2.1-R2.2 (repeat counts {N} and {*}), R2.1-R2.2 (task: output control).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags.
var version = "dev"

const (
	defaultPrefix  = "xx"
	defaultDigits  = 2
	repeatInfinite = -1
)

// config holds parsed flag state for csplit.
type config struct {
	file         string
	prefix       string
	digits       int
	suffixFormat string
	elideEmpty   bool
	silent       bool
}

// patternKind distinguishes integer, regexp, and skip patterns.
type patternKind int

const (
	patternLine   patternKind = iota // R1.4: INTEGER pattern
	patternRegexp                    // R1.2: /REGEXP/ pattern
	patternSkip                      // R1.3: %REGEXP% pattern
)

// pattern represents a parsed csplit pattern argument.
type pattern struct {
	kind   patternKind
	lineNo int
	re     *regexp.Regexp
	offset int
	repeat int // 0=no repeat, >0=N additional, -1=infinite
}

// splitter tracks state during pattern-based file splitting.
type splitter struct {
	cfg     config
	lines   []string
	pos     int
	fileIdx int
	files   []string
	sizes   []int
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, patterns, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, patterns))
}

// run reads input, applies patterns, and writes output files.
// R4.1: exit 0 on success. R4.2: exit 1 on error.
func run(cfg config, patterns []pattern) int {
	lines, err := readLines(cfg.file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csplit: %v\n", err)
		return 1
	}
	s := &splitter{cfg: cfg, lines: lines}
	if err := s.split(patterns); err != nil {
		removeFiles(s.files)
		fmt.Fprintf(os.Stderr, "csplit: %v\n", err)
		return 1
	}
	printSizes(cfg, s.sizes)
	return 0
}

// readLines reads all lines from a file or stdin, preserving newlines.
func readLines(name string) ([]string, error) {
	f, err := openInput(name)
	if err != nil {
		return nil, err
	}
	if f != os.Stdin {
		defer f.Close()
	}
	return scanLines(f)
}

// openInput opens a file or returns stdin for "-".
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("cannot open '%s' for reading: %v", name, err)
	}
	return f, nil
}

// scanLines reads all lines preserving trailing newlines.
func scanLines(r io.Reader) ([]string, error) {
	br := bufio.NewReader(r)
	var lines []string
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
	}
	return lines, nil
}

// split processes all patterns and writes the final remaining section.
func (s *splitter) split(patterns []pattern) error {
	for i := range patterns {
		if err := s.applyPattern(&patterns[i]); err != nil {
			return err
		}
	}
	return s.writeFinal()
}

// applyPattern applies a single pattern with its repeat count.
// R2.1: {N} repeats N additional times. R2.2: {*} repeats until exhausted.
func (s *splitter) applyPattern(pat *pattern) error {
	total := pat.repeat + 1
	if pat.repeat == repeatInfinite {
		total = len(s.lines) + 1
	}
	for rep := 0; rep < total; rep++ {
		if err := s.applyOnce(pat, rep); err != nil {
			if pat.repeat == repeatInfinite && rep > 0 {
				return nil
			}
			return err
		}
	}
	return nil
}

// applyOnce applies a pattern once for a given repetition index.
func (s *splitter) applyOnce(pat *pattern, rep int) error {
	splitPos, err := findSplit(pat, s.lines, s.pos, rep)
	if err != nil {
		return err
	}
	if pat.kind == patternSkip {
		s.pos = splitPos
		return nil
	}
	return s.writeAndAdvance(splitPos)
}

// writeAndAdvance writes a section and advances position.
func (s *splitter) writeAndAdvance(splitPos int) error {
	name, n, err := writeSection(
		s.cfg, s.fileIdx, s.lines[s.pos:splitPos],
	)
	if name != "" {
		s.files = append(s.files, name)
		s.sizes = append(s.sizes, n)
		s.fileIdx++
	}
	s.pos = splitPos
	return err
}

// writeFinal writes remaining lines after all patterns are applied.
func (s *splitter) writeFinal() error {
	name, n, err := writeSection(
		s.cfg, s.fileIdx, s.lines[s.pos:],
	)
	if name != "" {
		s.files = append(s.files, name)
		s.sizes = append(s.sizes, n)
	}
	return err
}

// findSplit dispatches to the appropriate split-point finder.
func findSplit(
	pat *pattern, lines []string, pos, rep int,
) (int, error) {
	if pat.kind == patternLine {
		return findLineSplit(pat.lineNo, len(lines), pos, rep)
	}
	searchFrom := pos
	if rep > 0 {
		searchFrom = pos + 1
	}
	return findRegexpSplit(pat, lines, pos, searchFrom)
}

// findLineSplit computes the split point for an integer pattern.
// R1.4: first application uses absolute line number; repeats are relative.
func findLineSplit(
	lineNo, totalLines, pos, rep int,
) (int, error) {
	var target int
	if rep == 0 {
		target = lineNo - 1
	} else {
		target = pos + lineNo
	}
	if target < pos {
		return 0, fmt.Errorf("'%d': line number out of range", lineNo)
	}
	if target > totalLines {
		return 0, fmt.Errorf("'%d': line number out of range", lineNo)
	}
	return target, nil
}

// findRegexpSplit finds the next matching line and returns the split point.
// R1.2: /REGEXP/ splits before the matching line.
// R1.3: %REGEXP% skips to the matching line.
func findRegexpSplit(
	pat *pattern, lines []string, pos, searchFrom int,
) (int, error) {
	for i := searchFrom; i < len(lines); i++ {
		if pat.re.MatchString(lines[i]) {
			target := i + pat.offset
			return clampTarget(target, pos, len(lines))
		}
	}
	return 0, fmt.Errorf("'%s': match not found", pat.re.String())
}

// clampTarget ensures target is within valid bounds.
func clampTarget(target, pos, total int) (int, error) {
	if target < 0 {
		target = 0
	}
	if target > total {
		target = total
	}
	if target < pos {
		return 0, fmt.Errorf("line number out of range on offset")
	}
	return target, nil
}

// writeSection writes lines to an output file.
// R3.4: -z suppresses empty output files.
func writeSection(
	cfg config, fileIdx int, lines []string,
) (string, int, error) {
	var buf []byte
	for _, l := range lines {
		buf = append(buf, l...)
	}
	if cfg.elideEmpty && len(buf) == 0 {
		return "", 0, nil
	}
	name := formatFilename(cfg, fileIdx)
	if err := os.WriteFile(name, buf, 0o666); err != nil {
		return name, 0, fmt.Errorf("write error: %w", err)
	}
	return name, len(buf), nil
}

// formatFilename builds an output filename from prefix and index.
// R3.1: default xx00. R3.2: -f sets prefix. R3.3: -n sets digits.
func formatFilename(cfg config, index int) string {
	if cfg.suffixFormat != "" {
		return cfg.prefix + fmt.Sprintf(cfg.suffixFormat, index)
	}
	return cfg.prefix + fmt.Sprintf("%0*d", cfg.digits, index)
}

// removeFiles deletes created output files on error.
func removeFiles(files []string) {
	for _, f := range files {
		os.Remove(f) // best-effort cleanup
	}
}

// printSizes prints byte counts unless -s/--silent is set.
func printSizes(cfg config, sizes []int) {
	if cfg.silent {
		return
	}
	for _, sz := range sizes {
		fmt.Println(sz)
	}
}

// parseArgs processes command-line arguments into config and patterns.
// Returns exit code -1 to continue, >= 0 for early exit.
func parseArgs(args []string) (config, []pattern, int) {
	cfg := config{
		prefix: defaultPrefix,
		digits: defaultDigits,
	}
	var positional []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		exit := classifyArg(args, &i, &cfg, &positional)
		if exit >= 0 {
			return config{}, nil, exit
		}
	}
	return buildResult(cfg, positional)
}

// classifyArg routes an argument to flag or positional parsing.
func classifyArg(
	args []string, i *int, cfg *config, positional *[]string,
) int {
	arg := args[*i]
	switch {
	case strings.HasPrefix(arg, "--"):
		return parseLongFlag(args, i, cfg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlag(args, i, cfg)
	default:
		*positional = append(*positional, arg)
		return -1
	}
}

// parseLongFlag handles --prefixed flags.
func parseLongFlag(args []string, i *int, cfg *config) int {
	arg := args[*i]
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "--elide-empty-files":
		cfg.elideEmpty = true
		return -1
	case arg == "--quiet", arg == "--silent":
		cfg.silent = true
		return -1
	case strings.HasPrefix(arg, "--prefix="):
		cfg.prefix = arg[len("--prefix="):]
		return -1
	case arg == "--prefix":
		return consumeString(args, i, &cfg.prefix)
	case strings.HasPrefix(arg, "--digits="):
		return applyDigits(cfg, arg[len("--digits="):])
	case arg == "--digits":
		return consumeDigits(args, i, cfg)
	case strings.HasPrefix(arg, "--suffix-format="):
		cfg.suffixFormat = arg[len("--suffix-format="):]
		return -1
	case arg == "--suffix-format":
		return consumeString(args, i, &cfg.suffixFormat)
	default:
		fmt.Fprintf(os.Stderr,
			"csplit: unrecognized option '%s'\n", arg)
		return 1
	}
}

// parseShortFlag handles single-dash flags.
func parseShortFlag(args []string, i *int, cfg *config) int {
	arg := args[*i]
	switch {
	case arg == "-z":
		cfg.elideEmpty = true
		return -1
	case arg == "-s":
		cfg.silent = true
		return -1
	case arg == "-f":
		return consumeString(args, i, &cfg.prefix)
	case strings.HasPrefix(arg, "-f"):
		cfg.prefix = arg[2:]
		return -1
	case arg == "-n":
		return consumeDigits(args, i, cfg)
	case strings.HasPrefix(arg, "-n"):
		return applyDigits(cfg, arg[2:])
	case arg == "-b":
		return consumeString(args, i, &cfg.suffixFormat)
	case strings.HasPrefix(arg, "-b"):
		cfg.suffixFormat = arg[2:]
		return -1
	default:
		fmt.Fprintf(os.Stderr,
			"csplit: unrecognized option '%s'\n", arg)
		return 1
	}
}

// consumeString reads the next argument as a string value.
func consumeString(args []string, i *int, target *string) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"csplit: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	*target = args[*i]
	return -1
}

// consumeDigits reads the next argument as the digit count.
func consumeDigits(args []string, i *int, cfg *config) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"csplit: option requires an argument -- '%s'\n", args[*i])
		return 1
	}
	*i++
	return applyDigits(cfg, args[*i])
}

// applyDigits parses and sets the digit count for output suffixes.
func applyDigits(cfg *config, val string) int {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr,
			"csplit: invalid number of digits: '%s'\n", val)
		return 1
	}
	cfg.digits = n
	return -1
}

// buildResult validates positional arguments and parses patterns.
func buildResult(
	cfg config, positional []string,
) (config, []pattern, int) {
	if len(positional) == 0 {
		fmt.Fprintln(os.Stderr, "csplit: missing operand")
		return config{}, nil, 1
	}
	cfg.file = positional[0]
	if len(positional) < 2 {
		fmt.Fprintf(os.Stderr,
			"csplit: missing operand after '%s'\n", cfg.file)
		return config{}, nil, 1
	}
	patterns, err := parsePatterns(positional[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "csplit: %v\n", err)
		return config{}, nil, 1
	}
	return cfg, patterns, -1
}

// parsePatterns parses pattern arguments and attaches repeat modifiers.
// R1.1: patterns are applied in order to split input.
func parsePatterns(args []string) ([]pattern, error) {
	var patterns []pattern
	for i := 0; i < len(args); i++ {
		if isRepeat(args[i]) {
			if len(patterns) == 0 {
				return nil, fmt.Errorf(
					"'%s': no preceding pattern", args[i])
			}
			rep, err := parseRepeat(args[i])
			if err != nil {
				return nil, err
			}
			patterns[len(patterns)-1].repeat = rep
			continue
		}
		pat, err := parseOnePattern(args[i])
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pat)
	}
	return patterns, nil
}

// isRepeat checks if an argument is a {N} or {*} repeat modifier.
func isRepeat(arg string) bool {
	return strings.HasPrefix(arg, "{") && strings.HasSuffix(arg, "}")
}

// parseRepeat parses {N} or {*} into a repeat count.
// R2.1: {N} = N additional repetitions. R2.2: {*} = infinite.
func parseRepeat(arg string) (int, error) {
	inner := arg[1 : len(arg)-1]
	if inner == "*" {
		return repeatInfinite, nil
	}
	n, err := strconv.Atoi(inner)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid repeat count: '%s'", arg)
	}
	return n, nil
}

// parseOnePattern parses a single non-repeat pattern argument.
func parseOnePattern(arg string) (pattern, error) {
	if len(arg) > 0 && arg[0] == '/' {
		return parseRegexpPattern(arg, '/', patternRegexp)
	}
	if len(arg) > 0 && arg[0] == '%' {
		return parseRegexpPattern(arg, '%', patternSkip)
	}
	return parseLinePattern(arg)
}

// parseLinePattern parses an integer line-number pattern.
// R1.4: INTEGER splits at the given line number.
func parseLinePattern(arg string) (pattern, error) {
	n, err := strconv.Atoi(arg)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid pattern: '%s'", arg)
	}
	if n <= 0 {
		return pattern{},
			fmt.Errorf("'%d': line number must be positive", n)
	}
	return pattern{kind: patternLine, lineNo: n}, nil
}

// parseRegexpPattern parses /REGEXP/[OFFSET] or %REGEXP%[OFFSET].
// R1.2: /REGEXP/ splits before the matching line.
// R1.3: %REGEXP% skips to the matching line.
func parseRegexpPattern(
	arg string, delim byte, kind patternKind,
) (pattern, error) {
	body := arg[1:]
	closeIdx := findClosingDelim(body, delim)
	if closeIdx < 0 {
		return pattern{}, fmt.Errorf("invalid pattern: '%s'", arg)
	}
	reStr := body[:closeIdx]
	rest := body[closeIdx+1:]
	re, err := regexp.Compile(reStr)
	if err != nil {
		return pattern{},
			fmt.Errorf("invalid regexp '%s': %v", reStr, err)
	}
	offset, err := parseOffset(rest)
	if err != nil {
		return pattern{}, err
	}
	return pattern{kind: kind, re: re, offset: offset}, nil
}

// findClosingDelim finds the first unescaped delimiter in s.
func findClosingDelim(s string, delim byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == delim {
			return i
		}
	}
	return -1
}

// parseOffset parses an optional +N or -N offset after a regexp.
func parseOffset(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid offset: '%s'", s)
	}
	return n, nil
}

// printHelp writes usage information and returns exit code 0.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: csplit [OPTION]... FILE PATTERN...
Output pieces of FILE separated by PATTERN(s) to files 'xx00', 'xx01', ...,
and output byte counts of each piece to standard output.

  -b, --suffix-format=FORMAT  use sprintf FORMAT instead of %02d
  -f, --prefix=PREFIX         use PREFIX instead of 'xx'
  -n, --digits=DIGITS         use specified number of digits instead of 2
  -s, --quiet, --silent       do not print counts of output file sizes
  -z, --elide-empty-files     suppress empty output files
      --help                  display this help and exit
      --version               output version information and exit
`)
	return 0
}

// printVersion writes version information and returns exit code 0.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "csplit (go-unix-utils) %s\n", version)
	return 0
}
