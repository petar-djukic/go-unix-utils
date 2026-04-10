// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/csplit: split a file into context-determined pieces.
// Implements srd068-csplit R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
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

// progName is used in diagnostic messages.
const progName = "csplit"

// defaultPrefix is the output filename prefix.
// R3.1: default prefix is "xx".
const defaultPrefix = "xx"

// defaultDigits is the default number of digits in numeric suffixes.
// R3.3: default is 2.
const defaultDigits = 2

// patternKind distinguishes the type of split pattern.
type patternKind int

const (
	patternRegex   patternKind = iota // R1.2: /REGEXP/ split
	patternSkip                       // R1.3: %REGEXP% skip
	patternLineNum                    // R1.4: INTEGER split
)

// pattern holds a parsed PATTERN argument from the command line.
type pattern struct {
	kind   patternKind
	regex  *regexp.Regexp // non-nil for patternRegex and patternSkip
	lineNo int64          // used for patternLineNum
	offset int            // R2.3: +N or -N offset for regex patterns
	repeat int            // R2.1: repeat count; -1 means {*} (R2.2)
}

// config holds parsed command-line options for csplit.
type config struct {
	prefix     string // R3.2: -f PREFIX
	digits     int    // R3.3: -n DIGITS
	elideEmpty bool   // R3.4: -z
	keepFiles  bool   // keep files on error (not implemented per non_goals)
	quiet      bool   // -s/--quiet/--silent: suppress byte count output
	inputFile  string // FILE argument
	patterns   []pattern
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the csplit logic and returns the exit code.
// R4.1: returns 0 on success.
// R4.2: returns 1 on error.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		reportError(err.Error())
		return 1
	}
	if len(cfg.patterns) == 0 {
		reportError("missing operand")
		return 1
	}
	rc, err := openInput(cfg.inputFile)
	if err != nil {
		reportError(err.Error())
		return 1
	}
	defer rc.Close()
	if err := execute(rc, &cfg); err != nil {
		reportError(err.Error())
		return 1
	}
	return 0
}

// parseArgs separates flags from positional arguments.
func parseArgs(args []string) (config, error) {
	cfg := config{
		prefix: defaultPrefix,
		digits: defaultDigits,
	}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if strings.HasPrefix(arg, "--") {
			adv, err := parseLongFlag(&cfg, args, i)
			if err != nil {
				return cfg, err
			}
			i += adv
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && !isPatternOrNum(arg) {
			adv, err := parseShortFlags(&cfg, args, i)
			if err != nil {
				return cfg, err
			}
			i += adv
			continue
		}
		break
	}
	return applyPositional(cfg, args[i:])
}

// isPatternOrNum returns true if arg looks like a negative number
// rather than a flag (used to distinguish -N from -f).
func isPatternOrNum(arg string) bool {
	if len(arg) < 2 {
		return false
	}
	_, err := strconv.ParseInt(arg, 10, 64)
	return err == nil
}

// parseLongFlag handles --key=value or --key value flags.
func parseLongFlag(cfg *config, args []string, i int) (int, error) {
	key, val, hasVal := splitLongFlag(args[i])
	switch key {
	case "--prefix":
		return setFlagVal(val, hasVal, args, i, setString(&cfg.prefix))
	case "--digits":
		return setFlagVal(val, hasVal, args, i, setDigits(cfg))
	case "--elide-empty-files":
		cfg.elideEmpty = true
		return 1, nil
	case "--quiet", "--silent":
		cfg.quiet = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", args[i])
	}
}

// splitLongFlag splits --key=value into key, value, hasValue.
func splitLongFlag(arg string) (string, string, bool) {
	key, val, found := strings.Cut(arg, "=")
	return key, val, found
}

// setFlagVal resolves a flag value and applies it via setter.
func setFlagVal(val string, hasVal bool, args []string, i int, apply func(string) error) (int, error) {
	if hasVal {
		return 1, apply(val)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument")
	}
	return 2, apply(args[i+1])
}

// parseShortFlags handles single-character flags.
func parseShortFlags(cfg *config, args []string, i int) (int, error) {
	flags := args[i][1:]
	j := 0
	for j < len(flags) {
		switch flags[j] {
		case 'f':
			return shortWithArg(flags[j+1:], args, i, setString(&cfg.prefix))
		case 'n':
			return shortWithArg(flags[j+1:], args, i, setDigits(cfg))
		case 'z':
			cfg.elideEmpty = true
			j++
		case 's', 'q':
			cfg.quiet = true
			j++
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return 1, nil
}

// shortWithArg extracts the value for a short flag with an argument.
func shortWithArg(rest string, args []string, i int, apply func(string) error) (int, error) {
	if rest != "" {
		return 1, apply(rest)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument")
	}
	return 2, apply(args[i+1])
}

// setString returns a setter that stores a string value.
func setString(dst *string) func(string) error {
	return func(s string) error {
		*dst = s
		return nil
	}
}

// setDigits returns a setter for the -n/--digits value.
// R3.3: must be a positive integer.
func setDigits(cfg *config) func(string) error {
	return func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid number of digits: '%s'", s)
		}
		cfg.digits = n
		return nil
	}
}

// applyPositional handles FILE and PATTERN arguments after flags.
func applyPositional(cfg config, pos []string) (config, error) {
	if len(pos) == 0 {
		return cfg, nil
	}
	cfg.inputFile = pos[0]
	for _, arg := range pos[1:] {
		p, err := parsePattern(arg)
		if err != nil {
			return cfg, err
		}
		cfg.patterns = append(cfg.patterns, p)
	}
	return cfg, nil
}

// parsePattern parses a single PATTERN argument.
// R1.2: /REGEXP/[+/-N]
// R1.3: %REGEXP%[+/-N]
// R1.4: INTEGER
// R2.1: {N} repeat count
// R2.2: {*} repeat until end
func parsePattern(arg string) (pattern, error) {
	raw, repeatCount, err := extractRepeat(arg)
	if err != nil {
		return pattern{}, err
	}
	if raw[0] == '/' {
		return parseRegexPattern(raw, patternRegex, repeatCount)
	}
	if raw[0] == '%' {
		return parseRegexPattern(raw, patternSkip, repeatCount)
	}
	return parseLineNumPattern(raw, repeatCount)
}

// extractRepeat separates a trailing {N} or {*} from the pattern.
// R2.1: {N} means repeat N additional times.
// R2.2: {*} means repeat until end of input.
func extractRepeat(arg string) (string, int, error) {
	idx := strings.LastIndex(arg, "{")
	if idx < 0 {
		return arg, 0, nil
	}
	if !strings.HasSuffix(arg, "}") {
		return arg, 0, nil
	}
	body := arg[idx+1 : len(arg)-1]
	raw := arg[:idx]
	if body == "*" {
		return raw, -1, nil
	}
	n, err := strconv.Atoi(body)
	if err != nil || n < 0 {
		return "", 0, fmt.Errorf("invalid repeat count: '%s'", arg)
	}
	return raw, n, nil
}

// parseRegexPattern parses /REGEXP/[+/-N] or %REGEXP%[+/-N].
// R2.3: offset after the closing delimiter.
func parseRegexPattern(raw string, kind patternKind, repeatCount int) (pattern, error) {
	delim := raw[0]
	end := strings.LastIndexByte(raw[1:], delim)
	if end < 0 {
		return pattern{}, fmt.Errorf("invalid pattern: '%s'", raw)
	}
	expr := raw[1 : end+1]
	offset := 0
	tail := raw[end+2:]
	if tail != "" {
		n, err := strconv.Atoi(tail)
		if err != nil {
			return pattern{}, fmt.Errorf("invalid offset: '%s'", tail)
		}
		offset = n
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid regex: %v", err)
	}
	return pattern{
		kind:   kind,
		regex:  re,
		offset: offset,
		repeat: repeatCount,
	}, nil
}

// parseLineNumPattern parses an INTEGER pattern.
// R1.4: line number must be positive.
func parseLineNumPattern(raw string, repeatCount int) (pattern, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return pattern{}, fmt.Errorf("invalid line number: '%s'", raw)
	}
	return pattern{
		kind:   patternLineNum,
		lineNo: n,
		repeat: repeatCount,
	}, nil
}

// openInput returns the input reader.
// Reads from stdin when FILE is "-" or empty.
func openInput(name string) (io.ReadCloser, error) {
	if name == "" || name == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(name)
}

// suffixGenerator produces sequential numeric suffixes for output files.
type suffixGenerator struct {
	prefix string
	digits int
	index  int
}

// newSuffixGenerator creates a suffix generator.
// R3.1, R3.2, R3.3: configurable prefix and digit width.
func newSuffixGenerator(prefix string, digits int) *suffixGenerator {
	return &suffixGenerator{prefix: prefix, digits: digits}
}

// next returns the next output filename.
func (g *suffixGenerator) next() string {
	name := fmt.Sprintf("%s%0*d", g.prefix, g.digits, g.index)
	g.index++
	return name
}

// execute performs the csplit operation.
// R1.1: applies patterns in order to split input.
func execute(r io.Reader, cfg *config) error {
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	gen := newSuffixGenerator(cfg.prefix, cfg.digits)
	return splitWithPatterns(lines, cfg.patterns, gen, cfg)
}

// readAllLines reads all lines from the input into a string slice.
func readAllLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// splitWithPatterns applies each pattern in order to split lines.
func splitWithPatterns(lines []string, patterns []pattern, gen *suffixGenerator, cfg *config) error {
	pos := 0
	var sizes []int
	for _, pat := range patterns {
		var err error
		pos, sizes, err = applyPattern(lines, pos, pat, gen, cfg, sizes)
		if err != nil {
			removeCreatedFiles(cfg, len(sizes))
			return err
		}
	}
	// Write remaining lines as the final piece.
	if pos < len(lines) {
		n, err := writePiece(gen, lines[pos:], cfg)
		if err != nil {
			return err
		}
		sizes = append(sizes, n)
	}
	printSizes(sizes, cfg.quiet)
	return nil
}

// applyPattern applies a single pattern (possibly repeated) to the lines.
func applyPattern(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config, sizes []int) (int, []int, error) {
	total := pat.repeat + 1
	if pat.repeat < 0 {
		total = -1 // {*}: unlimited
	}
	count := 0
	for total < 0 || count < total {
		newPos, sz, err := applyOnce(lines, pos, pat, gen, cfg)
		if err != nil {
			if total < 0 && count > 0 {
				// R2.4: {*} stops when no more matches; not an error.
				break
			}
			return pos, sizes, err
		}
		sizes = append(sizes, sz...)
		pos = newPos
		count++
	}
	return pos, sizes, nil
}

// applyOnce applies a pattern once and returns the new position.
func applyOnce(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config) (int, []int, error) {
	switch pat.kind {
	case patternRegex:
		return applyRegex(lines, pos, pat, gen, cfg)
	case patternSkip:
		return applySkip(lines, pos, pat)
	case patternLineNum:
		return applyLineNum(lines, pos, pat, gen, cfg)
	}
	return pos, nil, fmt.Errorf("unknown pattern kind")
}

// applyRegex handles /REGEXP/[+/-N] pattern application.
// R1.2: matching line becomes first line of the next piece.
// R2.3: offset adjusts the split point.
func applyRegex(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config) (int, []int, error) {
	matchIdx := findMatch(lines, pos, pat.regex)
	if matchIdx < 0 {
		return pos, nil, fmt.Errorf("'%s': match not found", pat.regex.String())
	}
	splitAt := matchIdx + pat.offset
	if splitAt < pos {
		splitAt = pos
	}
	if splitAt > len(lines) {
		splitAt = len(lines)
	}
	n, err := writePiece(gen, lines[pos:splitAt], cfg)
	if err != nil {
		return pos, nil, err
	}
	return splitAt, []int{n}, nil
}

// applySkip handles %REGEXP% pattern application.
// R1.3: skips to the match without creating an output file.
func applySkip(lines []string, pos int, pat pattern) (int, []int, error) {
	matchIdx := findMatch(lines, pos, pat.regex)
	if matchIdx < 0 {
		return pos, nil, fmt.Errorf("'%s': match not found", pat.regex.String())
	}
	splitAt := matchIdx + pat.offset
	if splitAt < pos {
		splitAt = pos
	}
	if splitAt > len(lines) {
		splitAt = len(lines)
	}
	return splitAt, nil, nil
}

// applyLineNum handles INTEGER pattern application.
// R1.4: line number becomes first line of next piece.
func applyLineNum(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config) (int, []int, error) {
	target := int(pat.lineNo) - 1 // convert 1-based to 0-based
	if target < pos {
		return pos, nil, fmt.Errorf("line number '%d' is smaller than current line", pat.lineNo)
	}
	if target > len(lines) {
		target = len(lines)
	}
	n, err := writePiece(gen, lines[pos:target], cfg)
	if err != nil {
		return pos, nil, err
	}
	return target, []int{n}, nil
}

// findMatch returns the index of the first line matching re, starting at pos.
func findMatch(lines []string, pos int, re *regexp.Regexp) int {
	for i := pos; i < len(lines); i++ {
		if re.MatchString(lines[i]) {
			return i
		}
	}
	return -1
}

// writePiece writes a slice of lines to the next output file.
// R3.4: -z suppresses empty output files.
func writePiece(gen *suffixGenerator, lines []string, cfg *config) (int, error) {
	if len(lines) == 0 && cfg.elideEmpty {
		return 0, nil
	}
	fname := gen.next()
	content := joinLines(lines)
	if err := os.WriteFile(fname, []byte(content), 0o666); err != nil {
		return 0, err
	}
	return len(content), nil
}

// joinLines joins lines with newline terminators.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// printSizes prints the byte count of each created file.
// R3.1: prints sizes to stdout unless -s/--quiet.
func printSizes(sizes []int, quiet bool) {
	if quiet {
		return
	}
	for _, sz := range sizes {
		fmt.Println(sz)
	}
}

// removeCreatedFiles removes output files created before an error.
// Per GNU behavior, files are removed on error by default.
func removeCreatedFiles(cfg *config, count int) {
	rgen := newSuffixGenerator(cfg.prefix, cfg.digits)
	for range count {
		fname := rgen.next()
		os.Remove(fname) // best-effort cleanup
	}
}

// reportError prints a GNU-compatible diagnostic to stderr.
func reportError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
}
