// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd026-cut: Remove Sections from Lines.
// Covers R1.1-R1.4 (byte and character selection),
// R2.1-R2.4 (field selection, delimiter, suppress, output-delimiter),
// R3.1-R3.3 (complement mode).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// selectMode specifies which selection mode is active.
type selectMode int

const (
	modeNone   selectMode = iota
	modeBytes              // -b
	modeChars              // -c
	modeFields             // -f
)

// cutRange is a 1-based inclusive range. end=0 means "to end of line".
type cutRange struct {
	start int
	end   int
}

// resolvedSpan is a 0-based half-open [start, end) span.
type resolvedSpan struct {
	start int
	end   int
}

// config holds parsed flag state.
type config struct {
	mode        selectMode
	ranges      []cutRange
	delimiter   byte
	outputDelim string
	outputSet   bool // true if --output-delimiter was explicitly set
	complement  bool
	suppress    bool // -s: suppress lines without delimiter
}

func main() {
	// R4.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, files))
}

// run processes all input files and returns the exit code.
// R1.3: when no files given, reads stdin.
// R4.1/R4.2: exit 0 on success, exit 1 if any file fails.
func run(cfg config, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	bw := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range files {
		if err := processFile(cfg, name, bw); err != nil {
			fmt.Fprintf(os.Stderr, "cut: %s\n", err)
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processFile opens and processes a single file.
// R4.2: returns error on open failure so the caller reports and continues.
func processFile(cfg config, name string, bw *bufio.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if name != "-" {
		defer r.Close()
	}
	return processInput(cfg, r, bw)
}

// openInput opens a file or returns stdin for "-".
// R1.3: "-" means stdin.
func openInput(name string) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError produces a GNU-compatible error message.
func formatOpenError(name string, err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// processInput reads lines from r and applies the cut operation.
func processInput(cfg config, r io.Reader, bw *bufio.Writer) error {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if wErr := processLine(cfg, line, bw); wErr != nil {
				return wErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// processLine dispatches a single line to the appropriate mode handler.
// R1.3: newlines are not counted; they terminate the output line.
func processLine(cfg config, line []byte, bw *bufio.Writer) error {
	hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
	content := line
	if hasNewline {
		content = line[:len(line)-1]
	}
	switch cfg.mode {
	case modeBytes, modeChars:
		return cutBytes(cfg, content, hasNewline, bw)
	case modeFields:
		return cutFields(cfg, content, hasNewline, bw)
	}
	return nil
}

// --- Byte/character selection ---

// cutBytes selects bytes/characters from content.
// R1.1/R1.2: byte and character selection (equivalent under LC_ALL=C).
// R1.4: out-of-range positions produce no output.
func cutBytes(cfg config, content []byte, nl bool, bw *bufio.Writer) error {
	spans := resolveSpans(cfg.ranges, len(content), cfg.complement)
	for i, s := range spans {
		if i > 0 && cfg.outputSet {
			if _, err := bw.WriteString(cfg.outputDelim); err != nil {
				return err
			}
		}
		if _, err := bw.Write(content[s.start:s.end]); err != nil {
			return err
		}
	}
	return writeNewline(nl, bw)
}

// --- Field selection ---

// cutFields selects fields from content by delimiter.
// R2.1: field selection with delimiter.
// R2.3: -s suppresses non-delimited lines.
func cutFields(cfg config, content []byte, nl bool, bw *bufio.Writer) error {
	if !containsByte(content, cfg.delimiter) {
		if cfg.suppress {
			return nil
		}
		return writeLine(content, nl, bw)
	}
	fields := splitFields(content, cfg.delimiter)
	spans := resolveSpans(cfg.ranges, len(fields), cfg.complement)
	return writeSelectedFields(fields, spans, cfg.outputDelim, nl, bw)
}

// writeSelectedFields outputs selected fields with delimiters between them.
// R2.4: uses output delimiter between fields.
func writeSelectedFields(
	fields [][]byte, spans []resolvedSpan, delim string, nl bool, bw *bufio.Writer,
) error {
	first := true
	for _, s := range spans {
		for j := s.start; j < s.end; j++ {
			if !first {
				if _, err := bw.WriteString(delim); err != nil {
					return err
				}
			}
			first = false
			if _, err := bw.Write(fields[j]); err != nil {
				return err
			}
		}
	}
	return writeNewline(nl, bw)
}

// --- Output helpers ---

// writeLine writes content and optional newline.
func writeLine(content []byte, nl bool, bw *bufio.Writer) error {
	if _, err := bw.Write(content); err != nil {
		return err
	}
	return writeNewline(nl, bw)
}

// writeNewline writes a newline byte if nl is true.
func writeNewline(nl bool, bw *bufio.Writer) error {
	if nl {
		return bw.WriteByte('\n')
	}
	return nil
}

// --- Data helpers ---

// splitFields splits content by a single-byte delimiter.
func splitFields(content []byte, delim byte) [][]byte {
	var fields [][]byte
	start := 0
	for i, b := range content {
		if b == delim {
			fields = append(fields, content[start:i])
			start = i + 1
		}
	}
	return append(fields, content[start:])
}

// containsByte returns true if b exists in data.
func containsByte(data []byte, b byte) bool {
	for _, c := range data {
		if c == b {
			return true
		}
	}
	return false
}

// --- Range resolution ---

// resolveSpans converts cutRanges to resolved 0-based half-open spans.
// Handles open-ended ranges, sorting, merging, and complement.
func resolveSpans(ranges []cutRange, length int, complement bool) []resolvedSpan {
	spans := convertAndClamp(ranges, length)
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].start < spans[j].start
	})
	merged := mergeSpans(spans)
	if complement {
		return complementSpans(merged, length)
	}
	return merged
}

// convertAndClamp converts 1-based cutRanges to 0-based spans clamped to length.
func convertAndClamp(ranges []cutRange, length int) []resolvedSpan {
	var spans []resolvedSpan
	for _, r := range ranges {
		s := r.start - 1
		e := r.end
		if r.end == 0 {
			e = length
		}
		if s >= length || e <= 0 {
			continue
		}
		if s < 0 {
			s = 0
		}
		if e > length {
			e = length
		}
		spans = append(spans, resolvedSpan{s, e})
	}
	return spans
}

// mergeSpans merges overlapping or contiguous sorted spans.
func mergeSpans(spans []resolvedSpan) []resolvedSpan {
	if len(spans) == 0 {
		return nil
	}
	merged := []resolvedSpan{spans[0]}
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s.start <= last.end {
			if s.end > last.end {
				last.end = s.end
			}
		} else {
			merged = append(merged, s)
		}
	}
	return merged
}

// complementSpans returns spans covering [0, length) minus the given spans.
func complementSpans(spans []resolvedSpan, length int) []resolvedSpan {
	var result []resolvedSpan
	pos := 0
	for _, s := range spans {
		if pos < s.start {
			result = append(result, resolvedSpan{pos, s.start})
		}
		pos = s.end
	}
	if pos < length {
		result = append(result, resolvedSpan{pos, length})
	}
	return result
}

// --- Flag parsing ---

// parseArgs processes command-line flags and returns config, files, exit code.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (config, []string, int) {
	cfg := config{delimiter: '\t'}
	var files []string
	var rangeStr string

	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}
		consumed, exit := parseFlag(args, i, &cfg, &rangeStr)
		if exit >= 0 {
			return config{}, nil, exit
		}
		i += consumed - 1
	}

	return finalizeConfig(cfg, rangeStr, files)
}

// parseFlag handles a single flag starting at args[i].
// Returns (args consumed, exit code). exit=-1 means continue.
func parseFlag(args []string, i int, cfg *config, rangeStr *string) (int, int) {
	arg := args[i]
	switch {
	case arg == "--help":
		return 1, printHelp()
	case arg == "--version":
		return 1, printVersion()
	case arg == "--complement":
		cfg.complement = true
		return 1, -1
	case arg == "-s" || arg == "--only-delimited":
		cfg.suppress = true
		return 1, -1
	case strings.HasPrefix(arg, "--output-delimiter"):
		return parseLongOutputDelim(arg, args, i, cfg)
	case strings.HasPrefix(arg, "--bytes"):
		return parseLongMode(modeBytes, "--bytes", arg, args, i, cfg, rangeStr)
	case strings.HasPrefix(arg, "--characters"):
		return parseLongMode(modeChars, "--characters", arg, args, i, cfg, rangeStr)
	case strings.HasPrefix(arg, "--fields"):
		return parseLongMode(modeFields, "--fields", arg, args, i, cfg, rangeStr)
	case strings.HasPrefix(arg, "--delimiter"):
		return parseLongDelim(arg, args, i, cfg)
	case strings.HasPrefix(arg, "-b"):
		return parseShortMode(modeBytes, arg[2:], args, i, cfg, rangeStr)
	case strings.HasPrefix(arg, "-c"):
		return parseShortMode(modeChars, arg[2:], args, i, cfg, rangeStr)
	case strings.HasPrefix(arg, "-f"):
		return parseShortMode(modeFields, arg[2:], args, i, cfg, rangeStr)
	case strings.HasPrefix(arg, "-d"):
		return parseShortDelim(arg[2:], args, i, cfg)
	default:
		fmt.Fprintf(os.Stderr, "cut: invalid option -- '%s'\n", arg[1:])
		return 1, 1
	}
}

// parseShortMode handles -b/-c/-f with optional attached value.
func parseShortMode(
	m selectMode, val string, args []string, i int, cfg *config, rangeStr *string,
) (int, int) {
	if val != "" {
		return setModeAndRange(m, val, cfg, rangeStr)
	}
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "cut: option requires an argument -- '%c'\n", modeFlag(m))
		return 1, 1
	}
	c, e := setModeAndRange(m, args[i+1], cfg, rangeStr)
	return c + 1, e
}

// parseLongMode handles --bytes/--characters/--fields with = or space.
func parseLongMode(
	m selectMode, prefix, arg string, args []string, i int, cfg *config, rangeStr *string,
) (int, int) {
	if strings.HasPrefix(arg, prefix+"=") {
		return setModeAndRange(m, arg[len(prefix)+1:], cfg, rangeStr)
	}
	if arg != prefix {
		fmt.Fprintf(os.Stderr, "cut: unrecognized option '%s'\n", arg)
		return 1, 1
	}
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "cut: option '%s' requires an argument\n", prefix)
		return 1, 1
	}
	c, e := setModeAndRange(m, args[i+1], cfg, rangeStr)
	return c + 1, e
}

// setModeAndRange sets the selection mode and range string.
// R1.4: enforces that only one mode is specified.
func setModeAndRange(m selectMode, val string, cfg *config, rangeStr *string) (int, int) {
	if cfg.mode != modeNone {
		fmt.Fprintln(os.Stderr, "cut: only one type of list may be specified")
		return 1, 1
	}
	cfg.mode = m
	*rangeStr = val
	return 1, -1
}

// modeFlag returns the short flag character for a mode.
func modeFlag(m selectMode) byte {
	switch m {
	case modeBytes:
		return 'b'
	case modeChars:
		return 'c'
	default:
		return 'f'
	}
}

// parseShortDelim handles -d with optional attached value.
func parseShortDelim(val string, args []string, i int, cfg *config) (int, int) {
	if val != "" {
		return applyDelimiter(val, cfg)
	}
	if i+1 >= len(args) {
		fmt.Fprintln(os.Stderr, "cut: option requires an argument -- 'd'")
		return 1, 1
	}
	c, e := applyDelimiter(args[i+1], cfg)
	return c + 1, e
}

// parseLongDelim handles --delimiter= and --delimiter DELIM.
func parseLongDelim(arg string, args []string, i int, cfg *config) (int, int) {
	prefix := "--delimiter"
	if strings.HasPrefix(arg, prefix+"=") {
		return applyDelimiter(arg[len(prefix)+1:], cfg)
	}
	if arg != prefix {
		fmt.Fprintf(os.Stderr, "cut: unrecognized option '%s'\n", arg)
		return 1, 1
	}
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "cut: option '%s' requires an argument\n", prefix)
		return 1, 1
	}
	c, e := applyDelimiter(args[i+1], cfg)
	return c + 1, e
}

// applyDelimiter validates and sets the delimiter.
// R2.2: delimiter must be exactly one byte.
func applyDelimiter(val string, cfg *config) (int, int) {
	if len(val) != 1 {
		fmt.Fprintln(os.Stderr, "cut: the delimiter must be a single character")
		return 1, 1
	}
	cfg.delimiter = val[0]
	return 1, -1
}

// parseLongOutputDelim handles --output-delimiter= and --output-delimiter STRING.
func parseLongOutputDelim(arg string, args []string, i int, cfg *config) (int, int) {
	prefix := "--output-delimiter"
	if strings.HasPrefix(arg, prefix+"=") {
		cfg.outputDelim = arg[len(prefix)+1:]
		cfg.outputSet = true
		return 1, -1
	}
	if arg != prefix {
		fmt.Fprintf(os.Stderr, "cut: unrecognized option '%s'\n", arg)
		return 1, 1
	}
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "cut: option '%s' requires an argument\n", prefix)
		return 1, 1
	}
	cfg.outputDelim = args[i+1]
	cfg.outputSet = true
	return 2, -1
}

// finalizeConfig validates the parsed config and parses the range string.
func finalizeConfig(cfg config, rangeStr string, files []string) (config, []string, int) {
	if cfg.mode == modeNone {
		fmt.Fprintln(os.Stderr,
			"cut: you must specify a list of bytes, characters, or fields")
		fmt.Fprintln(os.Stderr, "Try 'cut --help' for more information.")
		return config{}, nil, 1
	}
	ranges, err := parseRangeList(rangeStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %v\n", err)
		return config{}, nil, 1
	}
	cfg.ranges = ranges
	if !cfg.outputSet {
		cfg.outputDelim = string(cfg.delimiter)
	}
	return cfg, files, -1
}

// --- Range parsing ---

// parseRangeList parses a comma-separated list of range specifications.
// R1.1: N, N-M, N-, -M formats, 1-indexed.
func parseRangeList(s string) ([]cutRange, error) {
	if s == "" {
		return nil, fmt.Errorf("invalid byte, character or field list")
	}
	parts := strings.Split(s, ",")
	ranges := make([]cutRange, 0, len(parts))
	for _, p := range parts {
		r, err := parseOneRange(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

// parseOneRange parses a single range element.
func parseOneRange(s string) (cutRange, error) {
	if s == "" {
		return cutRange{}, rangeErr()
	}
	idx := strings.IndexByte(s, '-')
	if idx < 0 {
		return parseSinglePos(s)
	}
	if idx == 0 {
		return parseEndRange(s[1:])
	}
	if idx == len(s)-1 {
		return parseStartRange(s[:idx])
	}
	return parseFullRange(s[:idx], s[idx+1:])
}

// parseSinglePos parses "N" into a single-position range.
func parseSinglePos(s string) (cutRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return cutRange{}, rangeErr()
	}
	return cutRange{n, n}, nil
}

// parseEndRange parses "-M" into a range from 1 to M.
func parseEndRange(s string) (cutRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return cutRange{}, rangeErr()
	}
	return cutRange{1, n}, nil
}

// parseStartRange parses "N-" into an open-ended range from N.
func parseStartRange(s string) (cutRange, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return cutRange{}, rangeErr()
	}
	return cutRange{n, 0}, nil
}

// parseFullRange parses "N-M" into an inclusive range.
func parseFullRange(startStr, endStr string) (cutRange, error) {
	start, err := strconv.Atoi(startStr)
	if err != nil || start <= 0 {
		return cutRange{}, rangeErr()
	}
	end, err := strconv.Atoi(endStr)
	if err != nil || end <= 0 {
		return cutRange{}, rangeErr()
	}
	if start > end {
		return cutRange{}, fmt.Errorf("invalid decreasing range")
	}
	return cutRange{start, end}, nil
}

// rangeErr returns a standard range parsing error.
func rangeErr() error {
	return fmt.Errorf("invalid byte, character or field list")
}

// --- Help and version ---

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: cut OPTION... [FILE]...
Print selected parts of lines from each FILE to standard output.

With no FILE, or when FILE is -, read standard input.

  -b, --bytes=LIST        select only these bytes
  -c, --characters=LIST   select only these characters
  -d, --delimiter=DELIM   use DELIM instead of TAB for field delimiter
  -f, --fields=LIST       select only these fields
  -s, --only-delimited    do not print lines not containing delimiters
      --complement        complement the set of selected bytes, characters
                            or fields
      --output-delimiter=STRING  use STRING as the output delimiter
      --help     display this help and exit
      --version  output version information and exit

Use one, and only one of -b, -c or -f.  Each LIST is made up of one
range, or many ranges separated by commas.

Each range is one of:

  N     N'th byte, character or field, counted from 1
  N-    from N'th byte, character or field, to end of line
  N-M   from N'th to M'th byte, character or field
  -M    from first to M'th byte, character or field
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "cut (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
