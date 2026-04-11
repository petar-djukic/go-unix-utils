// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ptx: produce a permuted (KWIC) index.
// Implements srd111-ptx R1.1, R2.1-R2.3, R3.1, R4.1-R4.2, R5.1-R5.2.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "ptx"

// defaultWidth is the default output line width (R2.1).
const defaultWidth = 72

// defaultGap is the default minimum gap between columns (R2.1).
const defaultGap = 3

// config holds parsed command-line options from srd111-ptx.
type config struct {
	width         int    // R2.1: -w N output width
	gapSize       int    // R2.1: -g N gap size
	ignoreCase    bool   // R2.2: -f fold case
	autoReference bool   // R4.1: -A auto-reference
	references    bool   // R4.2: -r treat first field as reference
	wordRegexp    string // R3.1: -W REGEXP word pattern
	files         []string
	parseErr      bool
}

// indexEntry represents a single entry in the permuted index.
type indexEntry struct {
	ref     string // reference (filename:line or first field)
	head    string // text before the keyword, trailing whitespace trimmed
	keyword string // the keyword itself
	tail    string // raw text after the keyword (may start with whitespace)
}

// lineInfo tracks the source of each input line for reference generation.
type lineInfo struct {
	filename string
	num      int
	text     string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the ptx logic and returns the exit code.
// R5.1: returns 0 on success, 1 on error.
func run(args []string) int {
	cfg := parseArgs(args)
	if cfg.parseErr {
		return 1
	}
	lines, err := readInput(&cfg)
	if err != nil {
		die("%v", err)
		return 1
	}
	entries := buildIndex(&cfg, lines)
	sortEntries(entries, &cfg)
	formatted := formatEntries(entries, &cfg)
	if err := writeOutput(formatted); err != nil {
		return 1
	}
	return 0
}

// parseArgs extracts flags and file arguments from the command line.
func parseArgs(args []string) config {
	cfg := config{width: defaultWidth, gapSize: defaultGap}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n := parseLongFlag(&cfg, args, i)
			if n == 0 {
				die("unrecognized option '%s'", arg)
				cfg.parseErr = true
				return cfg
			}
			i += n - 1
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			n := parseShortFlags(&cfg, args, i)
			if n == 0 {
				return cfg
			}
			i += n - 1
			continue
		}
		cfg.files = append(cfg.files, arg)
	}
	return cfg
}

// parseLongFlag handles --long-form flags. Returns args consumed (0 if not matched).
func parseLongFlag(cfg *config, args []string, i int) int {
	arg := args[i]
	switch {
	case matchLong(arg, "--width"):
		return setLongInt(cfg, arg, args, i, "--width", &cfg.width)
	case matchLong(arg, "--gap-size"):
		return setLongInt(cfg, arg, args, i, "--gap-size", &cfg.gapSize)
	case matchLong(arg, "--word-regexp"):
		return setLongStr(cfg, arg, args, i, "--word-regexp", &cfg.wordRegexp)
	case arg == "--ignore-case":
		cfg.ignoreCase = true
		return 1
	case arg == "--auto-reference":
		cfg.autoReference = true
		return 1
	case arg == "--references":
		cfg.references = true
		return 1
	}
	return 0
}

// matchLong returns true if arg matches name exactly or as name=value.
func matchLong(arg, name string) bool {
	return arg == name || strings.HasPrefix(arg, name+"=")
}

// setLongInt parses an integer value from a long flag.
func setLongInt(cfg *config, arg string, args []string, i int, name string, dst *int) int {
	val, consumed := longValue(arg, args, i, name, cfg)
	if cfg.parseErr {
		return consumed
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		die("invalid argument '%s' for '%s'", val, name)
		cfg.parseErr = true
		return consumed
	}
	*dst = n
	return consumed
}

// setLongStr parses a string value from a long flag.
func setLongStr(cfg *config, arg string, args []string, i int, name string, dst *string) int {
	val, consumed := longValue(arg, args, i, name, cfg)
	if cfg.parseErr {
		return consumed
	}
	*dst = val
	return consumed
}

// longValue extracts the value from --flag=value or --flag value forms.
func longValue(arg string, args []string, i int, name string, cfg *config) (string, int) {
	if _, val, ok := strings.Cut(arg, "="); ok {
		return val, 1
	}
	if i+1 >= len(args) {
		die("option '%s' requires an argument", name)
		cfg.parseErr = true
		return "", 1
	}
	return args[i+1], 2
}

// parseShortFlags handles -x style flags including combined short flags.
func parseShortFlags(cfg *config, args []string, i int) int {
	arg := args[i]
	ch := arg[1]
	switch ch {
	case 'w':
		return setShortInt(cfg, arg, args, i, 'w', &cfg.width)
	case 'g':
		return setShortInt(cfg, arg, args, i, 'g', &cfg.gapSize)
	case 'W':
		return setShortStr(cfg, arg, args, i, 'W', &cfg.wordRegexp)
	default:
		return parseBoolFlags(cfg, arg)
	}
}

// setShortInt parses an integer value from a short flag (-w72 or -w 72).
func setShortInt(cfg *config, arg string, args []string, i int, flag byte, dst *int) int {
	val, consumed := shortValue(arg, args, i, flag, cfg)
	if cfg.parseErr {
		return consumed
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		die("invalid argument '%s' for '-%c'", val, flag)
		cfg.parseErr = true
		return consumed
	}
	*dst = n
	return consumed
}

// setShortStr parses a string value from a short flag (-Wpattern or -W pattern).
func setShortStr(cfg *config, arg string, args []string, i int, flag byte, dst *string) int {
	val, consumed := shortValue(arg, args, i, flag, cfg)
	if cfg.parseErr {
		return consumed
	}
	*dst = val
	return consumed
}

// shortValue extracts the value from -fVALUE or -f VALUE forms.
func shortValue(arg string, args []string, i int, flag byte, cfg *config) (string, int) {
	if len(arg) > 2 {
		return arg[2:], 1
	}
	if i+1 >= len(args) {
		die("option requires an argument -- '%c'", flag)
		cfg.parseErr = true
		return "", 1
	}
	return args[i+1], 2
}

// parseBoolFlags handles boolean short flags like -f, -A, -r, and combined -fAr.
func parseBoolFlags(cfg *config, arg string) int {
	for _, ch := range arg[1:] {
		switch ch {
		case 'f':
			cfg.ignoreCase = true
		case 'A':
			cfg.autoReference = true
		case 'r':
			cfg.references = true
		default:
			die("invalid option -- '%c'", ch)
			cfg.parseErr = true
			return 0
		}
	}
	return 1
}

// readInput reads all input lines from the configured files or stdin.
// R2.3: reads from FILE or stdin when no file or "-" is given.
func readInput(cfg *config) ([]lineInfo, error) {
	if len(cfg.files) == 0 {
		return readStream("<stdin>", os.Stdin)
	}
	var all []lineInfo
	for _, f := range cfg.files {
		lines, err := readOneFile(f)
		if err != nil {
			return nil, err
		}
		all = append(all, lines...)
	}
	return all, nil
}

// readOneFile reads lines from a named file or stdin if name is "-".
func readOneFile(name string) ([]lineInfo, error) {
	if name == "-" {
		return readStream("<stdin>", os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readStream(name, f)
}

// readStream reads all lines from a reader, tracking filename and line number.
func readStream(name string, r io.Reader) ([]lineInfo, error) {
	var lines []lineInfo
	scanner := bufio.NewScanner(r)
	num := 0
	for scanner.Scan() {
		num++
		lines = append(lines, lineInfo{
			filename: name, num: num, text: scanner.Text(),
		})
	}
	return lines, scanner.Err()
}

// buildIndex produces the permuted index entries from input lines.
// R1.1: each significant word appears as a keyword in context.
func buildIndex(cfg *config, lines []lineInfo) []indexEntry {
	re := compileWordRegexp(cfg)
	text := joinInputText(lines, cfg)
	ref := buildJoinedRef(lines, cfg)
	return buildLineEntries(text, ref, re)
}

// joinInputText joins all input lines into a single text for KWIC processing.
func joinInputText(lines []lineInfo, cfg *config) string {
	parts := make([]string, len(lines))
	for i, li := range lines {
		t := li.text
		if cfg.references {
			_, t = stripReference(t)
		}
		parts[i] = t
	}
	return strings.Join(parts, " ")
}

// buildJoinedRef returns a reference string when all lines are joined.
func buildJoinedRef(lines []lineInfo, cfg *config) string {
	if !cfg.autoReference || len(lines) == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", lines[0].filename, lines[0].num)
}

// compileWordRegexp returns the compiled word pattern.
// R3.1: uses -W REGEXP if set, otherwise default \w+ pattern.
func compileWordRegexp(cfg *config) *regexp.Regexp {
	pattern := `\w+`
	if cfg.wordRegexp != "" {
		pattern = cfg.wordRegexp
	}
	return regexp.MustCompile(pattern)
}

// buildLineEntries creates index entries for every keyword in a text.
func buildLineEntries(text, ref string, re *regexp.Regexp) []indexEntry {
	matches := re.FindAllStringIndex(text, -1)
	entries := make([]indexEntry, 0, len(matches))
	for _, m := range matches {
		entries = append(entries, indexEntry{
			ref:     ref,
			head:    strings.TrimRight(text[:m[0]], " \t"),
			keyword: text[m[0]:m[1]],
			tail:    text[m[1]:],
		})
	}
	return entries
}

// sortEntries sorts index entries by keyword.
// R2.2: -f folds case for sorting.
func sortEntries(entries []indexEntry, cfg *config) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].keyword, entries[j].keyword
		if cfg.ignoreCase {
			a, b = strings.ToUpper(a), strings.ToUpper(b)
		}
		return a < b
	})
}

// formatEntries formats index entries for output respecting width and gap.
// R2.1: output width and gap size control column layout.
func formatEntries(entries []indexEntry, cfg *config) []string {
	refW := maxRefWidth(entries)
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = formatEntry(e, cfg, refW)
	}
	return lines
}

// maxRefWidth returns the length of the longest reference string.
func maxRefWidth(entries []indexEntry) int {
	w := 0
	for _, e := range entries {
		if len(e.ref) > w {
			w = len(e.ref)
		}
	}
	return w
}

// formatEntry formats a single index entry into the output line.
// Layout: [ref][gap][left_ctx right-justified in half][gap][keyword+tail].
func formatEntry(e indexEntry, cfg *config, refW int) string {
	avail := cfg.width
	if refW > 0 {
		avail -= refW + cfg.gapSize
	}
	half := avail / 2
	left := truncateLeft(e.head, half)
	right := e.keyword + e.tail
	return buildOutputLine(left, right, cfg.gapSize, half, refW, e.ref)
}

// buildOutputLine assembles the formatted output line from its components.
func buildOutputLine(left, right string, gap, leftW, refW int, ref string) string {
	var sb strings.Builder
	if refW > 0 {
		fmt.Fprintf(&sb, "%*s%s", refW, ref, strings.Repeat(" ", gap))
	}
	fmt.Fprintf(&sb, "%*s%s%s", leftW, left, strings.Repeat(" ", gap), right)
	return sb.String()
}

// truncateLeft truncates a string from the left to fit maxW characters.
func truncateLeft(s string, maxW int) string {
	if maxW <= 0 || s == "" {
		return ""
	}
	if len(s) <= maxW {
		return s
	}
	return s[len(s)-maxW:]
}

// writeOutput writes formatted lines to stdout.
func writeOutput(lines []string) error {
	w := bufio.NewWriter(os.Stdout)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return w.Flush()
}

// stripReference removes the first field from a line when -r is active.
// R4.2: the first field is treated as a reference and excluded from indexing.
func stripReference(line string) (ref string, rest string) {
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return line, ""
	}
	return line[:idx], strings.TrimLeft(line[idx:], " \t")
}

// die prints a diagnostic message to stderr.
func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, fmt.Sprintf(format, args...))
}
