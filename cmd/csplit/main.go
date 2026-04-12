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
	kind    patternKind
	regex   *regexp.Regexp // non-nil for patternRegex and patternSkip
	lineNo  int64          // used for patternLineNum
	offset  int            // R2.3: +N or -N offset for regex patterns
	repeat  int            // R2.1: repeat count; -1 means {*} (R2.2)
	display string         // original pattern text for error messages
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
// R2.1/R2.2: standalone {N} or {*} modifies the previous pattern's repeat.
func applyPositional(cfg config, pos []string) (config, error) {
	if len(pos) == 0 {
		return cfg, nil
	}
	cfg.inputFile = pos[0]
	for _, arg := range pos[1:] {
		if err := applyOnePositional(&cfg, arg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// applyOnePositional handles a single positional argument.
// R2.1/R2.2: {N} and {*} are standalone repeat modifiers.
func applyOnePositional(cfg *config, arg string) error {
	rc, isRepeat, err := parseRepeatArg(arg)
	if isRepeat {
		if err != nil {
			return err
		}
		if len(cfg.patterns) == 0 {
			return fmt.Errorf("'%s': no preceding pattern", arg)
		}
		cfg.patterns[len(cfg.patterns)-1].repeat = rc
		return nil
	}
	p, err := parsePattern(arg)
	if err != nil {
		return err
	}
	cfg.patterns = append(cfg.patterns, p)
	return nil
}

// parseRepeatArg checks if an argument is a standalone {N} or {*} repeat.
// R2.1: {N} sets repeat to N.
// R2.2: {*} sets repeat to -1 (unlimited).
func parseRepeatArg(arg string) (int, bool, error) {
	if !strings.HasPrefix(arg, "{") || !strings.HasSuffix(arg, "}") {
		return 0, false, nil
	}
	body := arg[1 : len(arg)-1]
	if body == "*" {
		return -1, true, nil
	}
	n, err := strconv.Atoi(body)
	if err != nil || n < 0 {
		return 0, true, fmt.Errorf("invalid repeat count: '%s'", arg)
	}
	return n, true, nil
}

// parsePattern parses a single PATTERN argument.
// R1.2: /REGEXP/[+/-N]
// R1.3: %REGEXP%[+/-N]
// R1.4: INTEGER
func parsePattern(arg string) (pattern, error) {
	raw, repeatCount, err := extractRepeat(arg)
	if err != nil {
		return pattern{}, err
	}
	if len(raw) == 0 {
		return pattern{}, fmt.Errorf("invalid pattern: '%s'", arg)
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
		kind:    kind,
		regex:   re,
		offset:  offset,
		repeat:  repeatCount,
		display: raw,
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
		kind:    patternLineNum,
		lineNo:  n,
		repeat:  repeatCount,
		display: raw,
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
// Byte counts are printed immediately as each file is written.
func splitWithPatterns(lines []string, patterns []pattern, gen *suffixGenerator, cfg *config) error {
	pos := 0
	var fileCount int
	for _, pat := range patterns {
		var err error
		pos, fileCount, err = applyPattern(lines, pos, pat, gen, cfg, fileCount)
		if err != nil {
			removeCreatedFiles(cfg, fileCount)
			return err
		}
	}
	// Write remaining lines as the final piece.
	if pos < len(lines) {
		if _, err := writePiece(gen, lines[pos:], cfg); err != nil {
			return err
		}
	}
	return nil
}

// applyPattern applies a single pattern (possibly repeated) to the lines.
// R2.1: {N} repeats pattern N additional times (total N+1).
// R2.2: {*} repeats until no more matches.
// R2.4: error on no match (except {*} after at least one success).
func applyPattern(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config, fileCount int) (int, int, error) {
	total := pat.repeat + 1
	if pat.repeat < 0 {
		total = -1 // {*}: unlimited
	}
	count := 0
	for total < 0 || count < total {
		newPos, wrote, err := applyOnce(lines, pos, pat, gen, cfg, count > 0)
		if wrote {
			fileCount++
		}
		if err != nil {
			pos = newPos
			if total < 0 && count > 0 {
				// R2.4: {*} stops when no more matches; not an error.
				break
			}
			return pos, fileCount, err
		}
		pos = newPos
		count++
	}
	return pos, fileCount, nil
}

// applyOnce applies a pattern once and returns the new position.
// R2.1/R2.2: isRepeat is true for second and subsequent applications.
// Returns (newPos, wroteFile, error).
func applyOnce(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config, isRepeat bool) (int, bool, error) {
	switch pat.kind {
	case patternRegex:
		return applyRegex(lines, pos, pat, gen, cfg, isRepeat)
	case patternSkip:
		return applySkip(lines, pos, pat, isRepeat)
	case patternLineNum:
		return applyLineNum(lines, pos, pat, gen, cfg, isRepeat)
	}
	return pos, false, fmt.Errorf("unknown pattern kind")
}

// applyRegex handles /REGEXP/[+/-N] pattern application.
// R1.2: matching line becomes first line of the next piece.
// R2.3: offset adjusts the split point.
// R2.1/R2.2: on repeat, search starts from pos+1 to avoid re-matching.
// R2.4: on no match, writes remaining lines before returning error.
func applyRegex(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config, isRepeat bool) (int, bool, error) {
	searchFrom := pos
	if isRepeat {
		searchFrom = pos + 1
	}
	matchIdx := findMatch(lines, searchFrom, pat.regex)
	if matchIdx < 0 {
		return writeRemainingAndError(lines, pos, pat, gen, cfg)
	}
	splitAt := clampSplitAt(matchIdx+pat.offset, pos, len(lines))
	_, err := writePiece(gen, lines[pos:splitAt], cfg)
	if err != nil {
		return pos, false, err
	}
	return splitAt, true, nil
}

// writeRemainingAndError writes remaining lines before a no-match error.
// R2.4: GNU csplit writes remaining content before reporting the error.
func writeRemainingAndError(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config) (int, bool, error) {
	_, werr := writePiece(gen, lines[pos:], cfg)
	if werr != nil {
		return pos, false, werr
	}
	return len(lines), true, fmt.Errorf("'%s': match not found", pat.display)
}

// applySkip handles %REGEXP% pattern application.
// R1.3: skips to the match without creating an output file.
// R2.1/R2.2: on repeat, search starts from pos+1 to avoid re-matching.
func applySkip(lines []string, pos int, pat pattern, isRepeat bool) (int, bool, error) {
	searchFrom := pos
	if isRepeat {
		searchFrom = pos + 1
	}
	matchIdx := findMatch(lines, searchFrom, pat.regex)
	if matchIdx < 0 {
		return pos, false, fmt.Errorf("'%s': match not found", pat.display)
	}
	splitAt := clampSplitAt(matchIdx+pat.offset, pos, len(lines))
	return splitAt, false, nil
}

// applyLineNum handles INTEGER pattern application.
// R1.4: line number becomes first line of next piece.
// R2.1: on repeat, the integer is a relative offset from current position.
func applyLineNum(lines []string, pos int, pat pattern, gen *suffixGenerator, cfg *config, isRepeat bool) (int, bool, error) {
	target := computeLineTarget(pat.lineNo, pos, isRepeat)
	if target <= pos && isRepeat {
		return pos, false, fmt.Errorf("'%d': line number out of range", pat.lineNo)
	}
	if target < pos {
		return pos, false, fmt.Errorf("line number '%d' is smaller than current line", pat.lineNo)
	}
	if target > len(lines) {
		target = len(lines)
	}
	_, err := writePiece(gen, lines[pos:target], cfg)
	if err != nil {
		return pos, false, err
	}
	return target, true, nil
}

// computeLineTarget returns the 0-based target index for a line number pattern.
// R1.4: first application uses absolute line number (1-based to 0-based).
// R2.1: repeat applications use relative offset from current position.
func computeLineTarget(lineNo int64, pos int, isRepeat bool) int {
	if isRepeat {
		return pos + int(lineNo)
	}
	return int(lineNo) - 1
}

// clampSplitAt clamps the split position to [minPos, maxPos].
func clampSplitAt(splitAt, minPos, maxPos int) int {
	if splitAt < minPos {
		return minPos
	}
	if splitAt > maxPos {
		return maxPos
	}
	return splitAt
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
// Prints the byte count to stdout immediately unless quiet mode is set.
func writePiece(gen *suffixGenerator, lines []string, cfg *config) (int, error) {
	if len(lines) == 0 && cfg.elideEmpty {
		return 0, nil
	}
	fname := gen.next()
	content := joinLines(lines)
	if err := os.WriteFile(fname, []byte(content), 0o666); err != nil {
		return 0, err
	}
	if !cfg.quiet {
		fmt.Println(len(content))
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
