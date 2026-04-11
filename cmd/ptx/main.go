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
	width          int    // R2.1: -w N output width
	gapSize        int    // R2.1: -g N gap size
	ignoreCase     bool   // R2.2: -f fold case
	autoReference  bool   // R4.1: -A auto-reference
	references     bool   // R4.2: -r treat first field as reference
	rightReference bool   // -R right-side references
	wordRegexp     string // R3.1: -W REGEXP word pattern
	files          []string
	parseErr       bool
}

// indexEntry represents a single entry in the permuted index.
type indexEntry struct {
	ref     string // reference (filename:linenum or first field)
	head    string // text before the keyword, trailing whitespace trimmed
	keyword string // the keyword itself
	tail    string // raw text after the keyword
}

// lineInfo tracks the source of each input line for reference generation.
type lineInfo struct {
	filename string
	num      int
	text     string
}

// lineSpan maps an offset in the joined text to a per-line reference.
type lineSpan struct {
	offset int
	ref    string
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
	case arg == "--right-side-refs":
		cfg.rightReference = true
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

// parseBoolFlags handles boolean short flags like -f, -A, -r, -R and combined forms.
func parseBoolFlags(cfg *config, arg string) int {
	for _, ch := range arg[1:] {
		switch ch {
		case 'f':
			cfg.ignoreCase = true
		case 'A':
			cfg.autoReference = true
		case 'r':
			cfg.references = true
		case 'R':
			cfg.rightReference = true
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
		return readStream("", os.Stdin)
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
		return readStream("-", os.Stdin)
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
// R4.1/R4.2: per-line references via -A or -r.
func buildIndex(cfg *config, lines []lineInfo) []indexEntry {
	re := compileWordRegexp(cfg)
	if cfg.references {
		return buildPerLineIndex(lines, re)
	}
	text, spans := joinWithSpans(lines, cfg)
	return extractEntries(text, spans, re)
}

// buildPerLineIndex processes each line separately for -r mode.
// R4.2: first field is reference, remaining text is indexed per-line.
func buildPerLineIndex(lines []lineInfo, re *regexp.Regexp) []indexEntry {
	var entries []indexEntry
	for _, li := range lines {
		ref, text := stripReference(li.text)
		matches := re.FindAllStringIndex(text, -1)
		for _, m := range matches {
			entries = append(entries, indexEntry{
				ref:     ref,
				head:    strings.TrimRight(text[:m[0]], " \t"),
				keyword: text[m[0]:m[1]],
				tail:    text[m[1]:],
			})
		}
	}
	return entries
}

// joinWithSpans joins input line text and records per-line reference spans.
func joinWithSpans(lines []lineInfo, cfg *config) (string, []lineSpan) {
	parts := make([]string, len(lines))
	spans := make([]lineSpan, len(lines))
	offset := 0
	for i, li := range lines {
		ref := autoRef(cfg, li)
		spans[i] = lineSpan{offset: offset, ref: ref}
		parts[i] = li.text
		offset += len(li.text) + 1
	}
	return strings.Join(parts, " "), spans
}

// autoRef returns the auto-reference string for a line when -A is set.
// R4.1: format is "filename:linenum".
func autoRef(cfg *config, li lineInfo) string {
	if cfg.autoReference {
		return fmt.Sprintf("%s:%d", li.filename, li.num)
	}
	return ""
}

// extractEntries finds all keywords in text and maps each to its line reference.
func extractEntries(text string, spans []lineSpan, re *regexp.Regexp) []indexEntry {
	matches := re.FindAllStringIndex(text, -1)
	entries := make([]indexEntry, 0, len(matches))
	for _, m := range matches {
		entries = append(entries, indexEntry{
			ref:     findRef(spans, m[0]),
			head:    strings.TrimRight(text[:m[0]], " \t"),
			keyword: text[m[0]:m[1]],
			tail:    text[m[1]:],
		})
	}
	return entries
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

// findRef returns the reference for the line span containing pos.
func findRef(spans []lineSpan, pos int) string {
	ref := ""
	for _, s := range spans {
		if s.offset > pos {
			break
		}
		ref = s.ref
	}
	return ref
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

// displayRef returns the reference string as it appears in output.
// -A auto-refs get a trailing colon on the left side only.
func displayRef(ref string, cfg *config) string {
	if cfg.autoReference && !cfg.rightReference {
		return ref + ":"
	}
	return ref
}

// computeRefWidth returns the maximum displayed reference width.
func computeRefWidth(entries []indexEntry, cfg *config) int {
	w := 0
	for _, e := range entries {
		r := displayRef(e.ref, cfg)
		if len(r) > w {
			w = len(r)
		}
	}
	return w
}

// formatEntries formats index entries for output respecting width and gap.
func formatEntries(entries []indexEntry, cfg *config) []string {
	refW := computeRefWidth(entries, cfg)
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = formatEntry(e, cfg, refW)
	}
	return lines
}

// formatEntry formats a single index entry into the output line.
func formatEntry(e indexEntry, cfg *config, refW int) string {
	ref := displayRef(e.ref, cfg)
	half := cfg.width / 2
	if cfg.rightReference && refW > 0 {
		return fmtRightRef(e, cfg, half, refW, ref)
	}
	return fmtLeftRef(e, cfg, half, refW, ref)
}

// fmtLeftRef formats a line with reference on the left (or no reference).
// Layout: {ref rjust refW}{left rjust leftW}{gap}{keyword+tail}
func fmtLeftRef(e indexEntry, cfg *config, half, refW int, ref string) string {
	leftW := half
	if refW > 0 {
		leftW = half - refW - 1
	}
	leftW = max(leftW, 0)
	left := truncateLeft(e.head, leftW)
	right := e.keyword + e.tail
	var sb strings.Builder
	if refW > 0 {
		fmt.Fprintf(&sb, "%*s", refW, ref)
	}
	fmt.Fprintf(&sb, "%*s%s%s", leftW, left,
		strings.Repeat(" ", cfg.gapSize), right)
	return sb.String()
}

// fmtRightRef formats a line with reference on the right.
// Layout: {left rjust leftW}{gap}{keyword+tail padded to rightW}{gap}{ref}
func fmtRightRef(e indexEntry, cfg *config, half, _ int, ref string) string {
	leftW := max(cfg.width-half-cfg.gapSize, 0)
	rightW := half
	left := truncateLeft(e.head, leftW)
	right := padOrTrunc(e.keyword+e.tail, rightW)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%*s%s%s%s%s", leftW, left,
		strings.Repeat(" ", cfg.gapSize), right,
		strings.Repeat(" ", cfg.gapSize), ref)
	return sb.String()
}

// padOrTrunc pads s with spaces or truncates it to exactly w characters.
func padOrTrunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
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
