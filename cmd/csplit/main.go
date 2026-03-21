// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd068-csplit R1.1–R1.4: pattern-based file splitting
// with regex patterns, skip patterns, and line number patterns.
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
)

// patternKind identifies the type of split pattern. R1.1.
type patternKind int

const (
	patternRegex   patternKind = iota // /REGEXP/ — R1.2
	patternSkip                       // %REGEXP% — R1.3
	patternLineNum                    // INTEGER — R1.4
)

// pattern represents a single csplit pattern argument. R1.1.
type pattern struct {
	kind    patternKind
	regex   *regexp.Regexp
	lineNum int
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

// parsePatternArgs parses pattern arguments into the config. R1.1.
func parsePatternArgs(cfg *config, args []string) error {
	for _, arg := range args {
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

func isRegexPattern(arg string) bool {
	return len(arg) >= 2 && arg[0] == '/' && arg[len(arg)-1] == '/'
}

func isSkipPattern(arg string) bool {
	return len(arg) >= 2 && arg[0] == '%' && arg[len(arg)-1] == '%'
}

// parseRegexPattern parses a /REGEXP/ pattern. R1.2.
func parseRegexPattern(arg string) (pattern, error) {
	expr := arg[1 : len(arg)-1]
	re, err := regexp.Compile(expr)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid regular expression: %s", arg)
	}
	return pattern{kind: patternRegex, regex: re, raw: arg}, nil
}

// parseSkipPattern parses a %REGEXP% pattern. R1.3.
func parseSkipPattern(arg string) (pattern, error) {
	expr := arg[1 : len(arg)-1]
	re, err := regexp.Compile(expr)
	if err != nil {
		return pattern{}, fmt.Errorf("invalid regular expression: %s", arg)
	}
	return pattern{kind: patternSkip, regex: re, raw: arg}, nil
}

// parseLineNumPattern parses an INTEGER pattern. R1.4.
func parseLineNumPattern(arg string) (pattern, error) {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return pattern{}, fmt.Errorf("invalid pattern: %s", arg)
	}
	return pattern{kind: patternLineNum, lineNum: n, raw: arg}, nil
}

// executeCsplit reads input, splits by patterns, and writes output files.
func executeCsplit(cfg *config, stdin io.Reader, stdout io.Writer) error {
	lines, err := readLines(cfg.inputFile, stdin)
	if err != nil {
		return err
	}
	pieces, err := splitByPatterns(lines, cfg.patterns)
	if err != nil {
		return err
	}
	return writePiecesAndReport(lines, pieces, cfg, stdout)
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
func splitByPatterns(lines [][]byte, patterns []pattern) ([]piece, error) {
	var pieces []piece
	pos := 0
	for _, pat := range patterns {
		p, newPos, err := applyPattern(lines, pos, pat)
		if err != nil {
			return nil, err
		}
		if p != nil {
			pieces = append(pieces, *p)
		}
		pos = newPos
	}
	pieces = append(pieces, piece{start: pos, end: len(lines)})
	return pieces, nil
}

// applyPattern applies a single pattern and returns the piece (if any)
// and the new position. R1.2, R1.3, R1.4.
func applyPattern(lines [][]byte, pos int, pat pattern) (*piece, int, error) {
	switch pat.kind {
	case patternRegex:
		return applyRegexPattern(lines, pos, pat)
	case patternSkip:
		return applySkipPattern(lines, pos, pat)
	case patternLineNum:
		return applyLineNumPattern(lines, pos, pat)
	default:
		return nil, pos, fmt.Errorf("unknown pattern kind")
	}
}

// applyRegexPattern splits at the next matching line. R1.2.
func applyRegexPattern(lines [][]byte, pos int, pat pattern) (*piece, int, error) {
	matchIdx := findMatch(lines, pos, pat.regex)
	if matchIdx < 0 {
		return nil, pos, fmt.Errorf("'%s': match not found", pat.raw)
	}
	p := piece{start: pos, end: matchIdx}
	return &p, matchIdx, nil
}

// applySkipPattern skips to the next matching line without output. R1.3.
func applySkipPattern(lines [][]byte, pos int, pat pattern) (*piece, int, error) {
	matchIdx := findMatch(lines, pos, pat.regex)
	if matchIdx < 0 {
		return nil, pos, fmt.Errorf("'%s': match not found", pat.raw)
	}
	return nil, matchIdx, nil
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

// writePiecesAndReport writes output files and prints byte counts to stdout.
func writePiecesAndReport(lines [][]byte, pieces []piece, cfg *config, stdout io.Writer) error {
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
			return err
		}
		created = append(created, filename)
		fmt.Fprintln(stdout, byteCount)
		fileIdx++
	}
	return nil
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
