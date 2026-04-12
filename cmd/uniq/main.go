// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/uniq: report or filter adjacent duplicate lines.
// Implements srd028-uniq R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// config holds parsed command-line options for uniq.
type config struct {
	count          bool   // -c: prefix lines with occurrence count
	repeated       bool   // -d: only print duplicate lines
	allRepeated    string // -D/--all-repeated[=METHOD]: print all duplicate lines
	unique         bool   // -u: only print unique lines
	ignoreCase     bool   // -i: case-insensitive comparison
	skipFields     int    // -f N: skip first N fields before comparing
	skipChars      int    // -s N: skip first N chars before comparing
	checkChars     int    // -w N: compare at most N chars (-1 = unlimited)
	zeroTerminated bool   // -z: NUL-delimited lines
	group          string // --group[=METHOD]: group output with separators
	files          []string
}

// lineSep returns the line delimiter byte.
// R4.2: NUL when -z is set, newline otherwise.
func (c config) lineSep() byte {
	if c.zeroTerminated {
		return 0
	}
	return '\n'
}

// lineProcessor tracks state for adjacent-duplicate detection.
// R1.1: suppresses all but the first of each run of identical adjacent lines.
type lineProcessor struct {
	w         *bufio.Writer
	cfg       config
	prev      string   // first line of current group (with terminator)
	prevKey   string   // comparison key of current group
	count     int      // number of lines in current group
	group     []string // all lines in current group (for -D and --group)
	started   bool
	outGroups int // number of groups output (for separator logic)
}

// R4.4: SIGPIPE handler installed at start.
// R1.1: main entry with flag parsing.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		os.Exit(1)
	}

	// R4.4: validate incompatible flag combinations.
	if err := validateFlags(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		os.Exit(1)
	}
}

// validateFlags checks for incompatible flag combinations.
// R4.4: --group is incompatible with -c, -d, -D, and -u.
func validateFlags(cfg config) error {
	if cfg.group == "" {
		return nil
	}
	if cfg.count || cfg.repeated || cfg.allRepeated != "" || cfg.unique {
		return fmt.Errorf("--group is mutually exclusive with -c/-d/-D/-u")
	}
	return nil
}

// run opens input/output and processes lines.
// R1.3: reads stdin when no file; writes to output file when specified.
func run(cfg config) error {
	r, err := openInput(cfg)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	w, closer, err := openOutput(cfg)
	if err != nil {
		return err
	}
	defer closer()
	bw := bufio.NewWriter(w)
	if err := processLines(bufio.NewReader(r), bw, cfg); err != nil {
		return err
	}
	return bw.Flush()
}

// openInput returns the input reader based on positional args.
// R1.2: stdin when no input file or "-".
func openInput(cfg config) (*os.File, error) {
	if len(cfg.files) == 0 || cfg.files[0] == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(cfg.files[0])
	if err != nil {
		return nil, formatFileError(cfg.files[0], err)
	}
	return f, nil
}

// openOutput returns the output writer and a close function.
// R1.2: stdout when no output file specified.
func openOutput(cfg config) (io.Writer, func(), error) {
	if len(cfg.files) < 2 {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(cfg.files[1])
	if err != nil {
		return nil, nil, formatFileError(cfg.files[1], err)
	}
	return f, func() { f.Close() }, nil
}

// formatFileError extracts the underlying system error for GNU-compatible messages.
func formatFileError(name string, err error) error {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// --- Flag parsing ---

// parseArgs parses command-line arguments into config.
// R1.1: parses -c, -d, -D, -u, -i, -f, -s, -w, -z and long equivalents.
func parseArgs(args []string) (config, error) {
	cfg := config{checkChars: -1}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (arg != "-" && !strings.HasPrefix(arg, "-")) {
			if err := addPositional(&cfg, arg); err != nil {
				return config{}, err
			}
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			if err := addPositional(&cfg, arg); err != nil {
				return config{}, err
			}
			continue
		}
		skip, err := parseFlag(&cfg, arg, args, i)
		if err != nil {
			return config{}, err
		}
		i += skip
	}
	return cfg, nil
}

// addPositional adds a positional argument to config.
// R1.2: accepts 0, 1, or 2 positional arguments.
func addPositional(cfg *config, arg string) error {
	if len(cfg.files) >= 2 {
		return fmt.Errorf("extra operand '%s'", arg)
	}
	cfg.files = append(cfg.files, arg)
	return nil
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, arg string, args []string, i int) (int, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(cfg, arg, args, i)
	}
	return parseShortFlags(cfg, arg[1:], args, i)
}

// parseLongFlag handles --name and --name=value boolean flags.
func parseLongFlag(cfg *config, arg string, args []string, i int) (int, error) {
	switch arg {
	case "--count":
		cfg.count = true
		return 0, nil
	case "--repeated":
		cfg.repeated = true
		return 0, nil
	case "--unique":
		cfg.unique = true
		return 0, nil
	case "--ignore-case":
		cfg.ignoreCase = true
		return 0, nil
	case "--zero-terminated":
		cfg.zeroTerminated = true
		return 0, nil
	default:
		return parseLongFlagWithArg(cfg, arg, args, i)
	}
}

// parseLongFlagWithArg handles long flags that take arguments.
func parseLongFlagWithArg(cfg *config, arg string, args []string, i int) (int, error) {
	switch {
	case arg == "--all-repeated" || strings.HasPrefix(arg, "--all-repeated="):
		return parseAllRepeated(cfg, arg)
	case arg == "--group" || strings.HasPrefix(arg, "--group="):
		return parseGroup(cfg, arg)
	case arg == "--skip-fields" || strings.HasPrefix(arg, "--skip-fields="):
		return parseLongIntFlag(arg, args, i, "--skip-fields", &cfg.skipFields)
	case arg == "--skip-chars" || strings.HasPrefix(arg, "--skip-chars="):
		return parseLongIntFlag(arg, args, i, "--skip-chars", &cfg.skipChars)
	case arg == "--check-chars" || strings.HasPrefix(arg, "--check-chars="):
		return parseLongIntFlag(arg, args, i, "--check-chars", &cfg.checkChars)
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", arg[2:])
	}
}

// parseAllRepeated parses --all-repeated[=METHOD].
// METHOD is none (default), prepend, or separate.
func parseAllRepeated(cfg *config, arg string) (int, error) {
	if arg == "--all-repeated" {
		cfg.allRepeated = "none"
		return 0, nil
	}
	method := arg[len("--all-repeated="):]
	switch method {
	case "none", "prepend", "separate":
		cfg.allRepeated = method
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid argument '%s' for '--all-repeated'", method)
	}
}

// parseGroup parses --group[=METHOD].
// R4.3: METHOD is prepend, append, separate (default), or both.
func parseGroup(cfg *config, arg string) (int, error) {
	if arg == "--group" {
		cfg.group = "separate"
		return 0, nil
	}
	method := arg[len("--group="):]
	switch method {
	case "prepend", "append", "separate", "both":
		cfg.group = method
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid argument '%s' for '--group'", method)
	}
}

// parseLongIntFlag parses --name=N or --name N long flags.
func parseLongIntFlag(arg string, args []string, i int, name string, target *int) (int, error) {
	if strings.HasPrefix(arg, name+"=") {
		val := arg[len(name)+1:]
		return 0, parseIntValue(val, name[2:], target)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '%s' requires an argument", name)
	}
	return 1, parseIntValue(args[i+1], name[2:], target)
}

// parseIntValue converts a string to a non-negative integer for a named flag.
func parseIntValue(val, desc string, target *int) error {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of %s: '%s'", desc, val)
	}
	*target = n
	return nil
}

// parseShortFlags processes bundled short flags like -cdi.
// R1.1: boolean flags can be combined; -f/-s/-w consume the rest as argument.
func parseShortFlags(cfg *config, flags string, args []string, i int) (int, error) {
	j := 0
	for j < len(flags) {
		switch flags[j] {
		case 'c':
			cfg.count = true
		case 'd':
			cfg.repeated = true
		case 'D':
			cfg.allRepeated = "none"
		case 'u':
			cfg.unique = true
		case 'i':
			cfg.ignoreCase = true
		case 'z':
			cfg.zeroTerminated = true
		case 'f', 's', 'w':
			return parseShortIntFlag(cfg, flags[j], flags[j+1:], args, i)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
		j++
	}
	return 0, nil
}

// parseShortIntFlag parses -fN, -f N, -sN, -s N, -wN, -w N.
func parseShortIntFlag(cfg *config, flag byte, rest string, args []string, i int) (int, error) {
	var val string
	var skip int
	if rest != "" {
		val = rest
	} else if i+1 < len(args) {
		val = args[i+1]
		skip = 1
	} else {
		return 0, fmt.Errorf("option requires an argument -- '%c'", flag)
	}
	target := intFlagTarget(cfg, flag)
	return skip, parseIntValue(val, intFlagDesc(flag), target)
}

// intFlagTarget returns a pointer to the config field for the given short flag.
func intFlagTarget(cfg *config, flag byte) *int {
	switch flag {
	case 'f':
		return &cfg.skipFields
	case 's':
		return &cfg.skipChars
	default: // 'w'
		return &cfg.checkChars
	}
}

// intFlagDesc returns the description for integer flag error messages.
func intFlagDesc(flag byte) string {
	switch flag {
	case 'f':
		return "fields to skip"
	case 's':
		return "bytes to skip"
	default: // 'w'
		return "bytes to compare"
	}
}

// --- Line processing ---

// processLines reads input and applies adjacent-duplicate logic.
// R1.1: suppresses adjacent duplicates by default.
// R4.2: uses NUL delimiter when -z is set.
func processLines(r *bufio.Reader, w *bufio.Writer, cfg config) error {
	p := &lineProcessor{w: w, cfg: cfg}
	sep := cfg.lineSep()
	for {
		line, err := r.ReadString(sep)
		if len(line) > 0 {
			if werr := p.handle(line); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return p.finish()
}

// handle processes a single input line against the current group.
// R1.4: case-sensitive comparison by default.
func (p *lineProcessor) handle(line string) error {
	key := extractKey(line, p.cfg)
	if !p.started {
		p.startGroup(line, key)
		return nil
	}
	if keysEqual(key, p.prevKey, p.cfg.ignoreCase) {
		p.count++
		p.group = append(p.group, line)
		return nil
	}
	if err := p.flush(); err != nil {
		return err
	}
	p.startGroup(line, key)
	return nil
}

// startGroup begins a new group with the given line and key.
func (p *lineProcessor) startGroup(line, key string) {
	p.prev = line
	p.prevKey = key
	p.count = 1
	p.group = []string{line}
	p.started = true
}

// finish flushes the last group at EOF.
func (p *lineProcessor) finish() error {
	if !p.started {
		return nil
	}
	return p.flush()
}

// flush writes the current group based on configured output mode.
func (p *lineProcessor) flush() error {
	if p.cfg.group != "" {
		return p.flushGroup()
	}
	if p.cfg.allRepeated != "" {
		return p.flushAllRepeated()
	}
	if p.shouldOutput() {
		return p.writeLine(p.prev, p.count)
	}
	return nil
}

// shouldOutput decides if the current group should be written.
// R1.1: default outputs first line of every group.
func (p *lineProcessor) shouldOutput() bool {
	if p.cfg.repeated && p.count < 2 {
		return false
	}
	if p.cfg.unique && p.count > 1 {
		return false
	}
	return true
}

// flushAllRepeated writes all lines of a duplicate group for -D mode.
func (p *lineProcessor) flushAllRepeated() error {
	if p.count < 2 {
		return nil
	}
	if err := p.writeMethodSep(p.cfg.allRepeated); err != nil {
		return err
	}
	sep := p.cfg.lineSep()
	for _, line := range p.group {
		if _, err := p.w.WriteString(ensureTerminator(line, sep)); err != nil {
			return err
		}
	}
	p.outGroups++
	return nil
}

// flushGroup writes all lines of a group with blank-line separators.
// R4.3: --group outputs all input lines grouped by equality.
func (p *lineProcessor) flushGroup() error {
	method := p.cfg.group
	sep := string(p.cfg.lineSep())
	needBefore := method == "prepend" ||
		(method == "both" && p.outGroups == 0) ||
		(method == "separate" && p.outGroups > 0)
	if needBefore {
		if _, err := p.w.WriteString(sep); err != nil {
			return err
		}
	}
	for _, line := range p.group {
		if _, err := p.w.WriteString(ensureTerminator(line, p.cfg.lineSep())); err != nil {
			return err
		}
	}
	if method == "append" || method == "both" {
		if _, err := p.w.WriteString(sep); err != nil {
			return err
		}
	}
	p.outGroups++
	return nil
}

// writeMethodSep writes a blank line separator for --all-repeated method.
func (p *lineProcessor) writeMethodSep(method string) error {
	sep := string(p.cfg.lineSep())
	if method == "prepend" || (method == "separate" && p.outGroups > 0) {
		_, err := p.w.WriteString(sep)
		return err
	}
	return nil
}

// writeLine writes a single line with optional count prefix.
// GNU uniq always terminates output lines with the line separator.
func (p *lineProcessor) writeLine(line string, count int) error {
	sep := p.cfg.lineSep()
	if p.cfg.count {
		_, err := fmt.Fprintf(p.w, "%7d %s", count, ensureTerminator(line, sep))
		return err
	}
	_, err := p.w.WriteString(ensureTerminator(line, sep))
	return err
}

// ensureTerminator appends the separator if the line does not end with it.
func ensureTerminator(s string, sep byte) string {
	if len(s) > 0 && s[len(s)-1] != sep {
		return s + string(sep)
	}
	return s
}

// --- Comparison helpers ---

// extractKey returns the comparison portion of a line.
// R1.4: strips terminator, applies skip-fields, skip-chars, check-chars.
// R4.1: -w limits comparison to first N characters after skipping.
func extractKey(line string, cfg config) string {
	s := strings.TrimRight(line, string(cfg.lineSep()))
	s = skipFields(s, cfg.skipFields)
	if cfg.skipChars > 0 {
		if cfg.skipChars >= len(s) {
			s = ""
		} else {
			s = s[cfg.skipChars:]
		}
	}
	if cfg.checkChars >= 0 && cfg.checkChars < len(s) {
		s = s[:cfg.checkChars]
	}
	return s
}

// skipFields skips the first n whitespace-separated fields.
// R3.2: fields are blank-delimited (spaces and tabs).
func skipFields(s string, n int) string {
	pos := 0
	for i := 0; i < n && pos < len(s); i++ {
		for pos < len(s) && (s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		for pos < len(s) && s[pos] != ' ' && s[pos] != '\t' {
			pos++
		}
	}
	return s[pos:]
}

// keysEqual compares two keys, optionally case-insensitive.
// R1.4: case-sensitive by default. R3.1: -i for case-insensitive.
func keysEqual(a, b string, ignoreCase bool) bool {
	if ignoreCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}
