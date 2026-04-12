// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fmt: simple text formatter.
// Implements srd070-fmt R1.1, R2.1, R3.1, R4.1, R5.1, R6.1, R7.1, R8.1,
// R9.1, R10.1, R11.1, R12.1, R13.1, R13.2, R13.3, R13.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "fmt"

// defaultWidth is the GNU fmt default line width. R1.1.
const defaultWidth = 75

// defaultGoalPct is the default goal as a percentage of width. R6.1.
const defaultGoalPct = 93

// fmtConfig holds formatting parameters.
// R5.1, R6.1, R9.1, R10.1, R11.1, R12.1.
type fmtConfig struct {
	width          int
	goal           int
	splitOnly      bool   // R9.1: split long lines but don't join short ones
	uniformSpacing bool   // R10.1: uniform one/two space rule
	prefix         string // R11.1: only reformat lines with this prefix
	hasPrefix      bool   // R11.1: whether -p was given
	taggedPara     bool   // R12.1: tagged-paragraph mode
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the fmt logic and returns the exit code.
// R13.1: returns 0 on success. R13.2: returns 1 on error.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	cfg, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	readers, closers, err := openInputs(files, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	defer closeAll(closers)
	w := bufio.NewWriter(stdout)
	for _, r := range readers {
		formatInput(r, w, cfg)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", progName, err)
		return 1
	}
	return 0
}

// defaultFmtConfig returns a config with default values.
func defaultFmtConfig() fmtConfig {
	return fmtConfig{
		width: defaultWidth,
		goal:  defaultWidth * defaultGoalPct / 100,
	}
}

// parseArgs extracts flags and file arguments from the command line.
// R4.1, R5.1, R6.1, R9.1, R10.1, R11.1, R12.1, R13.2.
func parseArgs(args []string) (fmtConfig, []string, error) {
	cfg := defaultFmtConfig()
	var files []string
	goalExplicit := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if handled, err := parseValueFlags(arg, args, &i, &cfg, &goalExplicit); handled {
			if err != nil {
				return cfg, nil, err
			}
			continue
		}
		if handled := parseBoolFlags(arg, &cfg); handled {
			continue
		}
		// R13.2: reject unknown options.
		if isUnknownOption(arg) {
			return cfg, nil, fmt.Errorf("invalid option -- '%s'", arg[1:])
		}
		files = append(files, arg)
	}
	return finalizeParse(cfg, files, goalExplicit), files, nil
}

// parseValueFlags handles -w, -g, and -p flags that take a value argument.
func parseValueFlags(arg string, args []string, i *int, cfg *fmtConfig, goalExplicit *bool) (bool, error) {
	if v, ok, err := matchFlag(arg, args, i, "-w", "--width"); ok {
		if err != nil {
			return true, err
		}
		cfg.width, err = parsePositiveInt(v, "width")
		return true, err
	}
	if v, ok, err := matchFlag(arg, args, i, "-g", "--goal"); ok {
		if err != nil {
			return true, err
		}
		cfg.goal, err = parsePositiveInt(v, "goal")
		*goalExplicit = true
		return true, err
	}
	// R11.1: -p PREFIX / --prefix=PREFIX
	if v, ok, err := matchFlag(arg, args, i, "-p", "--prefix"); ok {
		if err != nil {
			return true, err
		}
		cfg.prefix = v
		cfg.hasPrefix = true
		return true, nil
	}
	return false, nil
}

// parseBoolFlags handles -s, -u, and -t boolean flags.
func parseBoolFlags(arg string, cfg *fmtConfig) bool {
	switch arg {
	case "-s", "--split-only":
		cfg.splitOnly = true // R9.1
		return true
	case "-u", "--uniform-spacing":
		cfg.uniformSpacing = true // R10.1
		return true
	case "-t", "--tagged-paragraph":
		cfg.taggedPara = true // R12.1
		return true
	}
	return false
}

// finalizeParse applies defaults and normalizes the parsed config.
func finalizeParse(cfg fmtConfig, _ []string, goalExplicit bool) fmtConfig {
	if !goalExplicit {
		cfg.goal = cfg.width * defaultGoalPct / 100
	}
	if cfg.goal > cfg.width {
		cfg.goal = cfg.width
	}
	return cfg
}

// matchFlag checks if arg matches the short or long form of a flag.
// Returns (value, matched, error). Advances *idx if value is in next arg.
func matchFlag(arg string, args []string, idx *int, short, long string) (string, bool, error) {
	if strings.HasPrefix(arg, long+"=") {
		return arg[len(long)+1:], true, nil
	}
	if arg == long {
		return nextArg(args, idx, long)
	}
	if strings.HasPrefix(arg, short) && len(arg) > len(short) {
		return arg[len(short):], true, nil
	}
	if arg == short {
		return nextArg(args, idx, short)
	}
	return "", false, nil
}

// nextArg returns the next argument for a flag that requires a value.
func nextArg(args []string, idx *int, flag string) (string, bool, error) {
	if *idx+1 >= len(args) {
		return "", true, fmt.Errorf("option '%s' requires an argument", flag)
	}
	*idx++
	return args[*idx], true, nil
}

// isUnknownOption returns true if arg looks like an unrecognized flag.
// R13.2: "-" (stdin) is not an option; "--" is handled before this.
func isUnknownOption(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// parsePositiveInt parses a string as a positive integer for a named option.
// R13.2: error format matches GNU fmt.
func parsePositiveInt(s, name string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s: '%s'", name, s)
	}
	return n, nil
}

// openInputs opens all input files. R4.1: "-" means stdin.
func openInputs(files []string, stdin io.Reader) ([]io.Reader, []func(), error) {
	if len(files) == 0 {
		files = []string{"-"}
	}
	readers := make([]io.Reader, 0, len(files))
	closers := make([]func(), 0, len(files))
	for _, name := range files {
		r, closer, err := openInput(name, stdin)
		if err != nil {
			closeAll(closers)
			return nil, nil, err
		}
		readers = append(readers, r)
		closers = append(closers, closer)
	}
	return readers, closers, nil
}

// openInput opens a single file or returns stdin for "-".
// R13.2: error format matches GNU fmt.
func openInput(name string, stdin io.Reader) (io.Reader, func(), error) {
	if name == "-" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, formatOpenError(name, err)
	}
	return f, func() { f.Close() }, nil
}

// formatOpenError formats a file open error to match GNU fmt's message format.
func formatOpenError(name string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		msg := pathErr.Err.Error()
		return fmt.Errorf("cannot open '%s' for reading: %s", name, capitalizeFirst(msg))
	}
	return fmt.Errorf("cannot open '%s' for reading: %s", name, err)
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// closeAll calls all closer functions.
func closeAll(closers []func()) {
	for _, fn := range closers {
		fn()
	}
}

// formatInput reads input and formats paragraphs.
// R1.1, R2.1, R3.1, R11.1.
func formatInput(r io.Reader, w *bufio.Writer, cfg fmtConfig) {
	scanner := bufio.NewScanner(r)
	var para []string
	firstPara := true
	for scanner.Scan() {
		line := scanner.Text()
		if isBlankLine(line) {
			if len(para) > 0 {
				writeParagraph(w, para, cfg, firstPara)
				firstPara = false
				para = para[:0]
			}
			if !firstPara {
				fmt.Fprintln(w)
			}
			firstPara = false
			continue
		}
		para = append(para, line)
	}
	if len(para) > 0 {
		writeParagraph(w, para, cfg, firstPara)
	}
}

// isBlankLine returns true if the line contains only whitespace.
func isBlankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// writeParagraph formats and outputs a single paragraph.
// R3.1, R9.1, R11.1, R12.1.
func writeParagraph(w *bufio.Writer, lines []string, cfg fmtConfig, _ bool) {
	if cfg.hasPrefix {
		writePrefixParagraph(w, lines, cfg)
		return
	}
	if cfg.splitOnly {
		writeSplitOnly(w, lines, cfg)
		return
	}
	formatParagraphLines(w, lines, cfg)
}

// formatParagraphLines performs normal paragraph formatting.
// R3.1, R12.1, R6.1.
func formatParagraphLines(w *bufio.Writer, lines []string, cfg fmtConfig) {
	// Single line within width: pass through unchanged to preserve spacing.
	if len(lines) == 1 && lineDisplayWidth(lines[0]) <= cfg.width && !cfg.uniformSpacing {
		fmt.Fprintln(w, lines[0])
		return
	}
	firstIndent := leadingWhitespace(lines[0])
	bodyIndent := firstIndent
	if len(lines) > 1 {
		bodyIndent = leadingWhitespace(lines[1])
	}
	words := collectWords(lines, cfg.uniformSpacing)
	writeWrapped(w, words, firstIndent, bodyIndent, cfg)
}

// writeSplitOnly handles -s mode: splits long lines without joining short ones.
// R9.1.
func writeSplitOnly(w *bufio.Writer, lines []string, cfg fmtConfig) {
	for _, line := range lines {
		if lineDisplayWidth(line) <= cfg.width {
			fmt.Fprintln(w, line)
			continue
		}
		splitLongLine(w, line, cfg)
	}
}

// splitLongLine breaks a single long line at word boundaries.
func splitLongLine(w *bufio.Writer, line string, cfg fmtConfig) {
	indent := leadingWhitespace(line)
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	words := strings.Fields(trimmed)
	writeWrapped(w, words, indent, indent, cfg)
}

// writePrefixParagraph handles -p mode: only reformat prefixed lines.
// R11.1: remove prefix before formatting, re-add afterward.
func writePrefixParagraph(w *bufio.Writer, lines []string, cfg fmtConfig) {
	var group []string
	for _, line := range lines {
		stripped, matched := stripPrefix(line, cfg.prefix)
		if !matched {
			flushPrefixGroup(w, group, cfg)
			group = group[:0]
			fmt.Fprintln(w, line)
			continue
		}
		group = append(group, stripped)
	}
	flushPrefixGroup(w, group, cfg)
}

// flushPrefixGroup formats accumulated prefixed lines and writes with prefix.
func flushPrefixGroup(w *bufio.Writer, group []string, cfg fmtConfig) {
	if len(group) == 0 {
		return
	}
	adjustedCfg := cfg
	adjustedCfg.hasPrefix = false
	adjustedCfg.width -= len(cfg.prefix)
	if adjustedCfg.width < 1 {
		adjustedCfg.width = 1
	}
	adjustedCfg.goal = adjustedCfg.width * defaultGoalPct / 100
	var buf strings.Builder
	bw := bufio.NewWriter(&buf)
	formatParagraphLines(bw, group, adjustedCfg)
	bw.Flush()
	addPrefixToOutput(w, buf.String(), cfg.prefix)
}

// addPrefixToOutput prepends prefix to each line of formatted output.
func addPrefixToOutput(w *bufio.Writer, output, prefix string) {
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		fmt.Fprintf(w, "%s%s\n", prefix, line)
	}
}

// stripPrefix checks if a line has the given prefix (after optional whitespace)
// and returns the line with prefix removed.
func stripPrefix(line, prefix string) (string, bool) {
	ws := leadingWhitespace(line)
	rest := line[len(ws):]
	if !strings.HasPrefix(rest, prefix) {
		return "", false
	}
	return ws + rest[len(prefix):], true
}

// lineDisplayWidth returns the visible width of a line.
func lineDisplayWidth(line string) int {
	return len(line)
}

// leadingWhitespace returns the whitespace prefix of a line.
func leadingWhitespace(line string) string {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}

// collectWords extracts all words from paragraph lines, stripping indent.
// R8.1: collapses multiple spaces. R10.1: uniform spacing mode.
func collectWords(lines []string, _ bool) []string {
	var words []string
	for _, line := range lines {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if trimmed == "" {
			continue
		}
		words = append(words, strings.Fields(trimmed)...)
	}
	return words
}

// computeBreaks finds optimal line break positions using dynamic programming.
// R6.1: minimizes deviation from goal width. R7.1: respects max width.
// Returns indices into words where each new line starts (excluding line 0).
func computeBreaks(words []string, firstIndentLen, bodyIndentLen int, cfg fmtConfig) []int {
	n := len(words)
	if n == 0 {
		return nil
	}
	// dp[i] = minimum cost to typeset words[i..n-1]
	dp := make([]int64, n+1)
	nextStart := make([]int, n) // nextStart[i] = first word of the line after the one starting at i
	for i := n - 1; i >= 0; i-- {
		dp[i] = math.MaxInt64
		indentLen, maxW := lineParams(i, firstIndentLen, bodyIndentLen, cfg)
		col := indentLen
		for j := i; j < n; j++ {
			col += wordSpacing(words, i, j) + len(words[j])
			if col > maxW && j > i {
				break
			}
			cost := lineCost(col, cfg.goal, j == n-1) + dp[j+1]
			if cost < dp[i] {
				dp[i] = cost
				nextStart[i] = j + 1
			}
			if col > maxW {
				break
			}
		}
	}
	return reconstructBreaks(nextStart, n)
}

// lineParams returns the indent length and effective max width for a line
// starting at word index i. R12.1: in tagged mode, the first line's effective
// width is reduced by the indent difference so text content is balanced.
func lineParams(i, firstIndentLen, bodyIndentLen int, cfg fmtConfig) (int, int) {
	if i == 0 {
		maxW := cfg.width
		if cfg.taggedPara && firstIndentLen > bodyIndentLen {
			maxW -= firstIndentLen - bodyIndentLen
		}
		return firstIndentLen, maxW
	}
	return bodyIndentLen, cfg.width
}

// wordSpacing returns the number of spaces before words[j] on a line starting at i.
func wordSpacing(words []string, i, j int) int {
	if j == i {
		return 0
	}
	return spacesAfterWord(words[j-1])
}

// underGoalPenalty is the extra per-character penalty for lines shorter than
// the goal width. This asymmetry matches GNU fmt's preference for slightly
// over-goal lines over slightly under-goal lines.
const underGoalPenalty = 3

// lineCost returns the DP cost for a line of given length.
// Last lines have zero cost; other lines penalize deviation from goal.
// Lines under the goal get an extra linear penalty to prefer fuller lines.
func lineCost(lineLen, goal int, isLast bool) int64 {
	if isLast {
		return 0
	}
	diff := int64(lineLen - goal)
	cost := diff * diff
	if lineLen < goal {
		cost += underGoalPenalty * (int64(goal) - int64(lineLen))
	}
	return cost
}

// reconstructBreaks extracts line-start indices from the DP next-start array.
func reconstructBreaks(nextStart []int, n int) []int {
	var breaks []int
	for i := nextStart[0]; i < n; i = nextStart[i] {
		breaks = append(breaks, i)
	}
	return breaks
}

// writeWrapped outputs words wrapped to width with the given indentation.
// R1.1, R3.1, R6.1, R7.1, R8.1.
func writeWrapped(w *bufio.Writer, words []string, firstIndent, bodyIndent string, cfg fmtConfig) {
	if len(words) == 0 {
		return
	}
	breaks := computeBreaks(words, len(firstIndent), len(bodyIndent), cfg)
	breakSet := make(map[int]bool, len(breaks))
	for _, b := range breaks {
		breakSet[b] = true
	}
	indent := firstIndent
	fmt.Fprint(w, indent)
	for i, word := range words {
		if breakSet[i] {
			fmt.Fprintln(w)
			indent = bodyIndent
			fmt.Fprint(w, indent)
		} else if i > 0 {
			writeSpaces(w, spacesAfterWord(words[i-1]))
		}
		fmt.Fprint(w, word)
	}
	fmt.Fprintln(w)
}

// spacesAfterWord returns the number of spaces to insert after a word.
// R8.1: two spaces after sentence-ending punctuation (. ! ?), one otherwise.
func spacesAfterWord(word string) int {
	if isSentenceEnd(word) {
		return 2
	}
	return 1
}

// isSentenceEnd returns true if the word ends with sentence punctuation.
func isSentenceEnd(word string) bool {
	if len(word) == 0 {
		return false
	}
	last := word[len(word)-1]
	return last == '.' || last == '!' || last == '?'
}

// writeSpaces writes n space characters to w.
func writeSpaces(w *bufio.Writer, n int) {
	for range n {
		w.WriteByte(' ')
	}
}
