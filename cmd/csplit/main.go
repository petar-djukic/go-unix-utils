// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd068-csplit R1.1–R1.4: pattern-based file splitting,
// R2.1–R2.4: repeat counts and offset patterns.
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

const (
	progName      = "csplit"
	defaultPrefix = "xx"
	defaultDigits = 2
	repeatForever = -1
)

// patternKind identifies the type of split pattern. R1.1.
type patternKind int

const (
	patternRegex   patternKind = iota // /REGEXP/ — R1.2
	patternSkip                       // %REGEXP% — R1.3
	patternLineNum                    // INTEGER — R1.4
)

// pattern represents a single csplit pattern argument.
// R1.1, R2.1 (repeat count), R2.2 (repeat star), R2.3 (offset).
type pattern struct {
	kind    patternKind
	regex   *regexp.Regexp
	lineNum int
	offset  int // R2.3: +N or -N offset from match
	repeat  int // R2.1/R2.2: 0=single, N=N more times, -1=forever
	raw     string
}

// config holds parsed command-line options for csplit.
type config struct {
	prefix     string
	digits     int
	elideEmpty bool
	inputFile  string
	patterns   []pattern
}

// piece represents one output section with start/end indices into lines.
type piece struct {
	start int
	end   int
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and executes the csplit operation.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if err := executeCsplit(cfg, stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	return 0
}

// parseArgs extracts configuration from command-line arguments.
func parseArgs(args []string) (*config, error) {
	cfg := &config{
		prefix: defaultPrefix,
		digits: defaultDigits,
	}
	positional, err := parseOptions(args, cfg)
	if err != nil {
		return nil, err
	}
	if len(positional) < 2 {
		return nil, fmt.Errorf("missing operand")
	}
	cfg.inputFile = positional[0]
	return cfg, parsePatternArgs(cfg, positional[1:])
}

// parseOptions processes flag arguments and returns remaining positional args.
func parseOptions(args []string, cfg *config) ([]string, error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			return positional, nil
		}
		if !isFlag(arg) {
			positional = append(positional, arg)
			continue
		}
		extra, err := parseSingleOption(args, i, cfg)
		if err != nil {
			return nil, err
		}
		i += extra
	}
	return positional, nil
}

// isFlag returns true if arg is a command-line flag, not a pattern or filename.
func isFlag(arg string) bool {
	if arg == "-" {
		return false
	}
	return strings.HasPrefix(arg, "-")
}

// parseSingleOption handles one flag and returns extra args consumed.
func parseSingleOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if n, err := parsePrefixOption(args, i, cfg); n >= 0 {
		return n, err
	}
	if n, err := parseDigitsOption(args, i, cfg); n >= 0 {
		return n, err
	}
	if arg == "-z" || arg == "--elide-empty-files" {
		cfg.elideEmpty = true
		return 0, nil
	}
	return 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// parsePrefixOption handles -f and --prefix flags. Returns -1 if not matched.
func parsePrefixOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--prefix=") {
		cfg.prefix = arg[len("--prefix="):]
		return 0, nil
	}
	if arg == "--prefix" {
		return requireNextArg(args, i, "--prefix", func(v string) error {
			cfg.prefix = v
			return nil
		})
	}
	if arg == "-f" {
		return requireNextArg(args, i, "-f", func(v string) error {
			cfg.prefix = v
			return nil
		})
	}
	if strings.HasPrefix(arg, "-f") {
		cfg.prefix = arg[2:]
		return 0, nil
	}
	return -1, nil
}

// parseDigitsOption handles -n and --digits flags. Returns -1 if not matched.
func parseDigitsOption(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	if strings.HasPrefix(arg, "--digits=") {
		return 0, setDigits(arg[len("--digits="):], cfg)
	}
	if arg == "--digits" {
		return requireNextArg(args, i, "--digits", func(v string) error {
			return setDigits(v, cfg)
		})
	}
	if arg == "-n" {
		return requireNextArg(args, i, "-n", func(v string) error {
			return setDigits(v, cfg)
		})
	}
	if strings.HasPrefix(arg, "-n") {
		return 0, setDigits(arg[2:], cfg)
	}
	return -1, nil
}

// requireNextArg validates that a next argument exists and calls the setter.
func requireNextArg(args []string, i int, name string, setter func(string) error) (int, error) {
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", name)
	}
	return 1, setter(args[i+1])
}

// setDigits parses and sets the suffix digit count.
func setDigits(s string, cfg *config) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid number of digits: '%s'", s)
	}
	cfg.digits = n
	return nil
}

// parsePatternArgs parses pattern and repeat arguments. R1.1, R2.1, R2.2.
func parsePatternArgs(cfg *config, args []string) error {
	for _, arg := range args {
		if isRepeatArg(arg) {
			if err := attachRepeat(cfg, arg); err != nil {
				return err
			}
			continue
		}
		p, err := parsePattern(arg)
		if err != nil {
			return err
		}
		cfg.patterns = append(cfg.patterns, p)
	}
	if len(cfg.patterns) == 0 {
		return fmt.Errorf("missing operand")
	}
	return nil
}

// attachRepeat parses a {N} or {*} arg and sets repeat on the last pattern.
func attachRepeat(cfg *config, arg string) error {
	if len(cfg.patterns) == 0 {
		return fmt.Errorf("'%s': no preceding pattern", arg)
	}
	repeat, err := parseRepeatArg(arg)
	if err != nil {
		return err
	}
	cfg.patterns[len(cfg.patterns)-1].repeat = repeat
	return nil
}

// isRepeatArg checks if arg is a {N} or {*} repeat count. R2.1, R2.2.
func isRepeatArg(arg string) bool {
	return len(arg) >= 3 && arg[0] == '{' && arg[len(arg)-1] == '}'
}

// parseRepeatArg parses a {N} or {*} argument. R2.1, R2.2.
func parseRepeatArg(arg string) (int, error) {
	inner := arg[1 : len(arg)-1]
	if inner == "*" {
		return repeatForever, nil
	}
	n, err := strconv.Atoi(inner)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid repeat count: '%s'", arg)
	}
	return n, nil
}

// parsePattern parses a single pattern argument. R1.1.
func parsePattern(arg string) (pattern, error) {
	if isRegexPattern(arg) {
		return parseRegexPattern(arg)
	}
	if isSkipPattern(arg) {
		return parseSkipPattern(arg)
	}
	return parseLineNumPattern(arg)
}

// isRegexPattern checks if arg starts with / and contains a closing /.
func isRegexPattern(arg string) bool {
	if len(arg) < 2 || arg[0] != '/' {
		return false
	}
	return findClosingDelim(arg, '/') > 0
}

// isSkipPattern checks if arg starts with % and contains a closing %.
func isSkipPattern(arg string) bool {
	if len(arg) < 2 || arg[0] != '%' {
		return false
	}
	return findClosingDelim(arg, '%') > 0
}

// findClosingDelim returns the index of the first closing delimiter after pos 0.
func findClosingDelim(arg string, delim byte) int {
	for i := 1; i < len(arg); i++ {
		if arg[i] == delim {
			return i
		}
	}
	return -1
}

// parseRegexPattern parses a /REGEXP/ or /REGEXP/+N pattern. R1.2, R2.3.
func parseRegexPattern(arg string) (pattern, error) {
	closeIdx := findClosingDelim(arg, '/')
	expr := arg[1:closeIdx]
	re, err := regexp.Compile(expr)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid regular expression: %s", arg)
	}
	offset, err := parseOffset(arg[closeIdx+1:])
	if err != nil {
		return pattern{}, err
	}
	return pattern{kind: patternRegex, regex: re, offset: offset, raw: arg}, nil
}

// parseSkipPattern parses a %REGEXP% or %REGEXP%+N pattern. R1.3, R2.3.
func parseSkipPattern(arg string) (pattern, error) {
	closeIdx := findClosingDelim(arg, '%')
	expr := arg[1:closeIdx]
	re, err := regexp.Compile(expr)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid regular expression: %s", arg)
	}
	offset, err := parseOffset(arg[closeIdx+1:])
	if err != nil {
		return pattern{}, err
	}
	return pattern{kind: patternSkip, regex: re, offset: offset, raw: arg}, nil
}

// parseLineNumPattern parses an INTEGER pattern. R1.4.
func parseLineNumPattern(arg string) (pattern, error) {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return pattern{}, fmt.Errorf("invalid pattern: %s", arg)
	}
	return pattern{kind: patternLineNum, lineNum: n, raw: arg}, nil
}

// parseOffset parses an optional +N or -N offset suffix. R2.3.
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

// executeCsplit reads input, splits by patterns, and writes output files.
// GNU csplit writes output even on split errors, then removes files.
func executeCsplit(cfg *config, stdin io.Reader, stdout io.Writer) error {
	lines, err := readLines(cfg.inputFile, stdin)
	if err != nil {
		return err
	}
	pieces, pos, splitErr := splitByPatterns(lines, cfg.patterns)
	pieces = append(pieces, piece{start: pos, end: len(lines)})
	created, writeErr := writeAndReport(lines, pieces, cfg, stdout)
	if writeErr != nil {
		return writeErr
	}
	if splitErr != nil {
		removeFiles(created)
		return splitErr
	}
	return nil
}

// readLines reads all lines from the input file or stdin.
func readLines(inputFile string, stdin io.Reader) ([][]byte, error) {
	if inputFile == "-" {
		return scanLines(stdin)
	}
	f, err := os.Open(inputFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return scanLines(f)
}

// scanLines reads all lines from a reader, preserving line endings.
func scanLines(r io.Reader) ([][]byte, error) {
	br := bufio.NewReader(r)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return lines, nil
}

// splitByPatterns applies patterns in order to determine split pieces. R1.1.
// On error, returns the pieces accumulated so far plus the current position.
func splitByPatterns(lines [][]byte, patterns []pattern) ([]piece, int, error) {
	var pieces []piece
	pos := 0
	for _, pat := range patterns {
		newPieces, newPos, err := applyWithRepeat(lines, pos, pat)
		pieces = append(pieces, newPieces...)
		pos = newPos
		if err != nil {
			return pieces, pos, err
		}
	}
	return pieces, pos, nil
}

// applyWithRepeat applies a pattern once or with repeats. R2.1, R2.2.
func applyWithRepeat(lines [][]byte, pos int, pat pattern) ([]piece, int, error) {
	total := pat.repeat + 1
	if pat.repeat < 0 {
		total = repeatForever
	}
	var pieces []piece
	searchFrom := pos
	for i := 0; total < 0 || i < total; i++ {
		p, newPos, err := applyPatternFrom(lines, pos, searchFrom, pat)
		if err != nil {
			if pat.repeat < 0 {
				break // R2.2: {*} exhaustion is always OK
			}
			return nil, pos, err // R2.4: no match is an error
		}
		if p != nil {
			pieces = append(pieces, *p)
		}
		pos = newPos
		nextSearch := pos + 1
		if nextSearch <= searchFrom {
			nextSearch = searchFrom + 1
		}
		searchFrom = nextSearch
	}
	return pieces, pos, nil
}

// applyPatternFrom dispatches to the appropriate pattern handler.
func applyPatternFrom(lines [][]byte, pos, searchFrom int, pat pattern) (*piece, int, error) {
	switch pat.kind {
	case patternRegex:
		return applyRegexFrom(lines, pos, searchFrom, pat)
	case patternSkip:
		return applySkipFrom(lines, pos, searchFrom, pat)
	case patternLineNum:
		return applyLineNumPattern(lines, pos, pat)
	default:
		return nil, pos, fmt.Errorf("unknown pattern kind")
	}
}

// applyRegexFrom splits at the next matching line. R1.2, R2.3.
func applyRegexFrom(lines [][]byte, pos, searchFrom int, pat pattern) (*piece, int, error) {
	matchIdx := findMatch(lines, searchFrom, pat.regex)
	if matchIdx < 0 {
		return nil, pos, fmt.Errorf("'%s': match not found", pat.raw)
	}
	splitPoint := clampSplitPoint(matchIdx+pat.offset, pos, len(lines))
	p := piece{start: pos, end: splitPoint}
	return &p, splitPoint, nil
}

// applySkipFrom skips to the next matching line without output. R1.3, R2.3.
func applySkipFrom(lines [][]byte, pos, searchFrom int, pat pattern) (*piece, int, error) {
	matchIdx := findMatch(lines, searchFrom, pat.regex)
	if matchIdx < 0 {
		return nil, pos, fmt.Errorf("'%s': match not found", pat.raw)
	}
	skipPoint := clampSplitPoint(matchIdx+pat.offset, pos, len(lines))
	return nil, skipPoint, nil
}

// applyLineNumPattern splits at the specified line number. R1.4.
func applyLineNumPattern(lines [][]byte, pos int, pat pattern) (*piece, int, error) {
	lineIdx := pat.lineNum - 1
	if lineIdx < pos {
		return nil, pos, fmt.Errorf("'%d': line number out of range", pat.lineNum)
	}
	if lineIdx > len(lines) {
		return nil, pos, fmt.Errorf("'%d': line number out of range", pat.lineNum)
	}
	p := piece{start: pos, end: lineIdx}
	return &p, lineIdx, nil
}

// findMatch returns the index of the first line matching re at or after pos.
// Returns -1 if no match is found.
func findMatch(lines [][]byte, pos int, re *regexp.Regexp) int {
	for i := pos; i < len(lines); i++ {
		if re.Match(lines[i]) {
			return i
		}
	}
	return -1
}

// clampSplitPoint constrains a split point to the valid range [min, max].
func clampSplitPoint(point, minVal, maxVal int) int {
	if point < minVal {
		return minVal
	}
	if point > maxVal {
		return maxVal
	}
	return point
}

// writeAndReport writes output files and prints byte counts to stdout.
// Returns the list of created filenames for cleanup on error.
func writeAndReport(lines [][]byte, pieces []piece, cfg *config, stdout io.Writer) ([]string, error) {
	var created []string
	fileIdx := 0
	for _, p := range pieces {
		if cfg.elideEmpty && p.start == p.end {
			continue
		}
		filename := makeFilename(fileIdx, cfg)
		byteCount, err := writePiece(filename, lines, p)
		if err != nil {
			removeFiles(created)
			return nil, err
		}
		created = append(created, filename)
		fmt.Fprintln(stdout, byteCount)
		fileIdx++
	}
	return created, nil
}

// writePiece writes lines for a piece to the named file and returns byte count.
func writePiece(filename string, lines [][]byte, p piece) (int, error) {
	f, err := os.Create(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	total := 0
	for i := p.start; i < p.end; i++ {
		n, werr := bw.Write(lines[i])
		if werr != nil {
			return total, werr
		}
		total += n
	}
	return total, bw.Flush()
}

// makeFilename constructs the output filename for the given index.
func makeFilename(index int, cfg *config) string {
	return fmt.Sprintf("%s%0*d", cfg.prefix, cfg.digits, index)
}

// removeFiles deletes files created during a failed split operation.
func removeFiles(files []string) {
	for _, f := range files {
		os.Remove(f) // best-effort cleanup
	}
}
