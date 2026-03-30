// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the csplit utility.
// Implements prd068-csplit R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultPrefix = "xx"
	defaultDigits = 2
)

// patternKind identifies the type of a csplit pattern.
type patternKind int

const (
	patternRegex   patternKind = iota // /REGEXP/
	patternSkip                       // %REGEXP%
	patternLineNum                    // INTEGER
)

// noMatchError indicates a pattern failed to find a match.
// R2.4: used to distinguish expected exhaustion ({*}) from real errors.
type noMatchError struct {
	msg string
}

func (e *noMatchError) Error() string {
	return e.msg
}

// isNoMatch reports whether err is a no-match error.
func isNoMatch(err error) bool {
	_, ok := err.(*noMatchError)
	return ok
}

// pattern holds a parsed csplit pattern.
type pattern struct {
	kind    patternKind
	regex   *regexp.Regexp
	lineNum int
	offset  int // R2.3: +N or -N offset for regex/skip patterns
	repeat  int // R2.1/R2.2: -1 for {*}, 0 for no repeat, N for {N}
	raw     string
}

// config holds parsed command-line options for csplit.
type config struct {
	file     string
	prefix   string
	digits   int
	elide    bool
	quiet    bool
	patterns []pattern
}

// splitState tracks progress during pattern-based splitting.
type splitState struct {
	cfg          config
	lines        [][]byte
	pos          int
	fileIndex    int
	createdFiles []string
	lastMatch    int // R2.1/R2.2: last regex/skip match index for repeat search
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments, reads input, and performs context-based splitting.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csplit: %v\n", err)
		return 1
	}
	lines, err := readInput(cfg.file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "csplit: %v\n", err)
		return 1
	}
	if err := splitByPatterns(lines, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "csplit: %v\n", err)
		return 1
	}
	return 0
}

// parseArgs separates options from positional arguments (FILE PATTERN...).
// R1.1: accepts one or more pattern arguments.
func parseArgs(args []string) (config, error) {
	cfg := config{prefix: defaultPrefix, digits: defaultDigits}
	pos, err := extractPositional(&cfg, args)
	if err != nil {
		return cfg, err
	}
	if len(pos) < 2 {
		return cfg, fmt.Errorf("missing operand")
	}
	cfg.file = pos[0]
	return cfg, parsePatternList(&cfg, pos[1:])
}

// extractPositional splits args into options (applied to cfg) and positional args.
func extractPositional(cfg *config, args []string) ([]string, error) {
	var pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			return pos, nil
		}
		if arg == "-" || arg[0] != '-' {
			pos = append(pos, arg)
			continue
		}
		ni, err := handleOption(cfg, args, i)
		if err != nil {
			return nil, err
		}
		i = ni
	}
	return pos, nil
}

// handleOption dispatches to short or long option parsing.
func handleOption(cfg *config, args []string, i int) (int, error) {
	if strings.HasPrefix(args[i], "--") {
		return handleLongOption(cfg, args, i)
	}
	return handleShortOption(cfg, args, i)
}

// handleShortOption processes single-character options.
func handleShortOption(cfg *config, args []string, i int) (int, error) {
	c := args[i][1]
	switch c {
	case 'z':
		cfg.elide = true
		return i, nil
	case 's':
		cfg.quiet = true
		return i, nil
	case 'f':
		val, ni, err := shortOptValue(args, i)
		if err != nil {
			return i, err
		}
		cfg.prefix = val
		return ni, nil
	case 'n':
		return parseShortDigits(cfg, args, i)
	default:
		return i, fmt.Errorf("invalid option -- '%c'", c)
	}
}

// handleLongOption processes --key=value style options.
func handleLongOption(cfg *config, args []string, i int) (int, error) {
	key, val, hasVal := strings.Cut(args[i][2:], "=")
	switch key {
	case "elide-empty-files":
		cfg.elide = true
		return i, nil
	case "quiet", "silent":
		cfg.quiet = true
		return i, nil
	case "prefix":
		v, ni, err := longOptValue(val, hasVal, args, i)
		if err != nil {
			return i, err
		}
		cfg.prefix = v
		return ni, nil
	case "digits":
		return parseLongDigits(cfg, val, hasVal, args, i)
	default:
		return i, fmt.Errorf("unrecognized option '--%s'", key)
	}
}

// shortOptValue extracts the value for a short option (-fVAL or -f VAL).
func shortOptValue(args []string, i int) (string, int, error) {
	val := args[i][2:]
	if val != "" {
		return val, i, nil
	}
	if i+1 >= len(args) {
		return "", i, fmt.Errorf("option requires an argument -- '%c'", args[i][1])
	}
	return args[i+1], i + 1, nil
}

// longOptValue extracts the value for a long option (--key=val or --key val).
func longOptValue(val string, hasVal bool, args []string, i int) (string, int, error) {
	if hasVal {
		return val, i, nil
	}
	if i+1 >= len(args) {
		return "", i, fmt.Errorf("option '--%s' requires an argument", args[i][2:])
	}
	return args[i+1], i + 1, nil
}

// parseShortDigits parses -n DIGITS.
func parseShortDigits(cfg *config, args []string, i int) (int, error) {
	val, ni, err := shortOptValue(args, i)
	if err != nil {
		return i, err
	}
	d, perr := strconv.Atoi(val)
	if perr != nil || d <= 0 {
		return ni, fmt.Errorf("invalid number of digits: '%s'", val)
	}
	cfg.digits = d
	return ni, nil
}

// parseLongDigits parses --digits=N or --digits N.
func parseLongDigits(cfg *config, val string, hasVal bool, args []string, i int) (int, error) {
	v, ni, err := longOptValue(val, hasVal, args, i)
	if err != nil {
		return i, err
	}
	d, perr := strconv.Atoi(v)
	if perr != nil || d <= 0 {
		return ni, fmt.Errorf("invalid number of digits: '%s'", v)
	}
	cfg.digits = d
	return ni, nil
}

// parsePatternList parses a slice of pattern strings into the config.
// R1.1: accepts one or more pattern arguments applied in order.
// R2.1/R2.2: handles {N} and {*} repeat counts as separate arguments.
func parsePatternList(cfg *config, args []string) error {
	for i := 0; i < len(args); i++ {
		pat, err := parsePattern(args[i])
		if err != nil {
			return err
		}
		if i+1 < len(args) && isRepeatArg(args[i+1]) {
			rep, rerr := parseRepeat(args[i+1])
			if rerr != nil {
				return rerr
			}
			pat.repeat = rep
			i++
		}
		cfg.patterns = append(cfg.patterns, pat)
	}
	return nil
}

// parsePattern parses a single pattern string into a pattern struct.
// R1.1: supports /REGEXP/[OFFSET], %REGEXP%[OFFSET], and INTEGER forms.
func parsePattern(s string) (pattern, error) {
	if len(s) > 1 && s[0] == '/' {
		return parseDelimitedPattern(s, '/', patternRegex)
	}
	if len(s) > 1 && s[0] == '%' {
		return parseDelimitedPattern(s, '%', patternSkip)
	}
	return parseLineNumPattern(s)
}

// parseDelimitedPattern parses /REGEXP/[OFFSET] or %REGEXP%[OFFSET] patterns.
// R1.2: regex split. R1.3: skip pattern. R2.3: optional +N/-N offset.
func parseDelimitedPattern(s string, delim byte, kind patternKind) (pattern, error) {
	idx := strings.LastIndexByte(s, delim)
	if idx <= 0 {
		return pattern{}, fmt.Errorf("invalid pattern: '%s'", s)
	}
	expr := s[1:idx]
	re, err := regexp.Compile(expr)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid regexp '%s': %v", expr, err)
	}
	offset, oerr := parseOffset(s[idx+1:])
	if oerr != nil {
		return pattern{}, oerr
	}
	return pattern{kind: kind, regex: re, offset: offset, raw: s}, nil
}

// parseOffset parses an optional +N or -N offset suffix.
// R2.3: offset from the matching line.
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

// isRepeatArg reports whether s is a {N} or {*} repeat argument.
func isRepeatArg(s string) bool {
	return len(s) >= 3 && s[0] == '{' && s[len(s)-1] == '}'
}

// parseRepeat parses a {N} or {*} repeat argument.
// R2.1: {N} returns N. R2.2: {*} returns -1.
func parseRepeat(s string) (int, error) {
	inner := s[1 : len(s)-1]
	if inner == "*" {
		return -1, nil
	}
	n, err := strconv.Atoi(inner)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid repeat count: '%s'", s)
	}
	return n, nil
}

// parseLineNumPattern parses an INTEGER pattern.
// R1.4: line number split point.
func parseLineNumPattern(s string) (pattern, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid pattern: '%s'", s)
	}
	if n <= 0 {
		return pattern{}, fmt.Errorf("'%s': line number out of range", s)
	}
	return pattern{kind: patternLineNum, lineNum: n, raw: s}, nil
}

// readInput reads all lines from the specified file or stdin.
func readInput(file string) ([][]byte, error) {
	r, err := openFile(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return readLines(r)
}

// openFile opens the input file, or returns stdin for "-".
func openFile(file string) (io.ReadCloser, error) {
	if file == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(file)
}

// readLines reads all data from r and splits into lines preserving newlines.
func readLines(r io.Reader) ([][]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var lines [][]byte
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			lines = append(lines, data)
			break
		}
		lines = append(lines, data[:idx+1])
		data = data[idx+1:]
	}
	return lines, nil
}

// splitByPatterns applies patterns in order to split input lines into files.
func splitByPatterns(lines [][]byte, cfg config) error {
	s := &splitState{cfg: cfg, lines: lines, lastMatch: -1}
	if err := applyAllPatterns(s); err != nil {
		cleanupFiles(s.createdFiles)
		return err
	}
	return nil
}

// applyAllPatterns processes each pattern then writes remaining content.
func applyAllPatterns(s *splitState) error {
	for _, pat := range s.cfg.patterns {
		if err := applyPatternWithRepeat(s, pat); err != nil {
			return err
		}
	}
	return writeRemaining(s)
}

// applyPatternWithRepeat applies a pattern once, then repeats per R2.1/R2.2.
func applyPatternWithRepeat(s *splitState, pat pattern) error {
	if err := applyPattern(s, pat, s.pos); err != nil {
		return err
	}
	if pat.repeat == 0 {
		return nil
	}
	if pat.repeat < 0 {
		return repeatUntilDone(s, pat)
	}
	return repeatN(s, pat)
}

// repeatUntilDone applies the pattern until no match is found.
// R2.2: {*} repeats until end of input; final no-match is not an error.
func repeatUntilDone(s *splitState, pat pattern) error {
	for {
		sf := repeatSearchFrom(s, pat)
		if sf >= len(s.lines) {
			return nil
		}
		err := applyPattern(s, pat, sf)
		if err == nil {
			continue
		}
		if isNoMatch(err) {
			return nil
		}
		return err
	}
}

// repeatN applies the pattern N additional times.
// R2.1: {N} repeats N additional times (total N+1 applications).
// R2.4: error messages include the repetition number.
func repeatN(s *splitState, pat pattern) error {
	for i := 0; i < pat.repeat; i++ {
		sf := repeatSearchFrom(s, pat)
		if err := applyPattern(s, pat, sf); err != nil {
			return annotateRepeat(err, i+1)
		}
	}
	return nil
}

// annotateRepeat adds "on repetition N" to no-match errors.
func annotateRepeat(err error, n int) error {
	nme, ok := err.(*noMatchError)
	if !ok {
		return err
	}
	return &noMatchError{
		msg: fmt.Sprintf("%s on repetition %d", nme.msg, n),
	}
}

// repeatSearchFrom computes the search start for a repeat application.
// For regex/skip, starts past the last match to avoid re-matching.
func repeatSearchFrom(s *splitState, pat pattern) int {
	if pat.kind == patternLineNum {
		return s.pos
	}
	if s.lastMatch >= s.pos {
		return s.lastMatch + 1
	}
	return s.pos
}

// applyPattern dispatches to the handler for the pattern kind.
func applyPattern(s *splitState, pat pattern, searchFrom int) error {
	switch pat.kind {
	case patternRegex:
		return processRegex(s, pat, searchFrom)
	case patternSkip:
		return processSkip(s, pat, searchFrom)
	case patternLineNum:
		return processLineNum(s, pat)
	default:
		return fmt.Errorf("unknown pattern kind")
	}
}

// processRegex finds the next matching line and writes preceding lines as output.
// R1.2: the matching line becomes the first line of the next piece.
// R2.3: offset shifts the split point relative to the match.
// R2.4: writes remaining content before reporting no-match error.
func processRegex(s *splitState, pat pattern, searchFrom int) error {
	idx := findMatch(s.lines, searchFrom, pat.regex)
	if idx < 0 {
		writeOnError(s)
		return &noMatchError{msg: fmt.Sprintf("'%s': match not found", pat.raw)}
	}
	s.lastMatch = idx
	splitAt := idx + pat.offset
	if splitAt < s.pos || splitAt > len(s.lines) {
		return fmt.Errorf("'%s': line number out of range", pat.raw)
	}
	return writePiece(s, s.pos, splitAt)
}

// processSkip advances past the next matching line without writing output.
// R1.3: skip to matching line without creating an output file.
// R2.3: offset shifts the skip point relative to the match.
func processSkip(s *splitState, pat pattern, searchFrom int) error {
	idx := findMatch(s.lines, searchFrom, pat.regex)
	if idx < 0 {
		return &noMatchError{msg: fmt.Sprintf("'%s': match not found", pat.raw)}
	}
	s.lastMatch = idx
	skipTo := idx + pat.offset
	if skipTo < s.pos || skipTo > len(s.lines) {
		return fmt.Errorf("'%s': line number out of range", pat.raw)
	}
	s.pos = skipTo
	return nil
}

// processLineNum writes lines from current position to the given line number.
// R1.4: the specified line becomes the first line of the next piece.
func processLineNum(s *splitState, pat pattern) error {
	idx := pat.lineNum - 1
	if idx < s.pos || idx >= len(s.lines) {
		writeOnError(s)
		return &noMatchError{
			msg: fmt.Sprintf("'%d': line number out of range", pat.lineNum),
		}
	}
	return writePiece(s, s.pos, idx)
}

// writePiece writes lines[from:to] to the next output file and prints byte count.
func writePiece(s *splitState, from, to int) error {
	data := joinLineRange(s.lines, from, to)
	name := outputName(s.cfg.prefix, s.cfg.digits, s.fileIndex)
	if s.cfg.elide && len(data) == 0 {
		s.pos = to
		return nil
	}
	if err := os.WriteFile(name, data, 0o666); err != nil {
		return err
	}
	s.createdFiles = append(s.createdFiles, name)
	if !s.cfg.quiet {
		fmt.Println(len(data))
	}
	s.fileIndex++
	s.pos = to
	return nil
}

// writeOnError writes remaining content before reporting an error.
// R2.4: GNU csplit outputs byte count for remaining content on failure.
func writeOnError(s *splitState) {
	if s.pos >= len(s.lines) {
		return
	}
	_ = writePiece(s, s.pos, len(s.lines)) // best-effort, error ignored
}

// writeRemaining writes any lines after the last pattern as the final piece.
func writeRemaining(s *splitState) error {
	if s.pos >= len(s.lines) {
		return nil
	}
	return writePiece(s, s.pos, len(s.lines))
}

// findMatch returns the 0-based index of the first matching line from pos.
func findMatch(lines [][]byte, pos int, re *regexp.Regexp) int {
	for i := pos; i < len(lines); i++ {
		if re.Match(lines[i]) {
			return i
		}
	}
	return -1
}

// joinLineRange concatenates lines[from:to] into a single byte slice.
func joinLineRange(lines [][]byte, from, to int) []byte {
	var buf bytes.Buffer
	for i := from; i < to; i++ {
		buf.Write(lines[i])
	}
	return buf.Bytes()
}

// outputName generates a zero-padded numeric suffix filename.
// R3.1: default prefix "xx" and 2-digit suffixes.
func outputName(prefix string, digits, index int) string {
	s := strconv.Itoa(index)
	if pad := digits - len(s); pad > 0 {
		s = strings.Repeat("0", pad) + s
	}
	return prefix + s
}

// cleanupFiles removes output files created during a failed split.
func cleanupFiles(files []string) {
	for _, f := range files {
		os.Remove(f) // best-effort cleanup, error ignored
	}
}
