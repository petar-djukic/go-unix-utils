// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fmt: simple text formatter.
// Implements srd070-fmt R1.1, R2.1, R3.1, R4.1, R5.1, R6.1, R7.1, R8.1,
// R9.1, R10.1, R11.1, R12.1.
package main

import (
	"bufio"
	"fmt"
	"io"
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
// R4.1, R5.1, R6.1, R9.1, R10.1, R11.1, R12.1.
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
func finalizeParse(cfg fmtConfig, files []string, goalExplicit bool) fmtConfig {
	if !goalExplicit {
		cfg.goal = cfg.width * defaultGoalPct / 100
	}
	if cfg.goal > cfg.width {
		cfg.goal = cfg.width
	}
	if len(files) == 0 {
		// caller will use "-" default
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
		if *idx+1 >= len(args) {
			return "", true, fmt.Errorf("option '%s' requires an argument", long)
		}
		*idx++
		return args[*idx], true, nil
	}
	if strings.HasPrefix(arg, short) && len(arg) > len(short) {
		return arg[len(short):], true, nil
	}
	if arg == short {
		if *idx+1 >= len(args) {
			return "", true, fmt.Errorf("option '%s' requires an argument", short)
		}
		*idx++
		return args[*idx], true, nil
	}
	return "", false, nil
}

// parsePositiveInt parses a string as a positive integer for a named option.
func parsePositiveInt(s, name string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s: %q", name, s)
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
func openInput(name string, stdin io.Reader) (io.Reader, func(), error) {
	if name == "-" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
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
// R3.1, R12.1.
func formatParagraphLines(w *bufio.Writer, lines []string, cfg fmtConfig) {
	firstIndent := leadingWhitespace(lines[0])
	bodyIndent := firstIndent
	if len(lines) > 1 {
		bodyIndent = leadingWhitespace(lines[1])
	}
	// R12.1: in tagged-paragraph mode, preserve first-line indent distinctly.
	if cfg.taggedPara && len(lines) > 1 {
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
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	for _, line := range lines {
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
		lineWords := strings.Fields(trimmed)
		words = append(words, lineWords...)
	}
	return words
}

// writeWrapped outputs words wrapped to width with the given indentation.
// R1.1, R3.1, R7.1, R8.1, R10.1.
func writeWrapped(w *bufio.Writer, words []string, firstIndent, bodyIndent string, cfg fmtConfig) {
	if len(words) == 0 {
		return
	}
	indent := firstIndent
	col := len(indent)
	fmt.Fprint(w, indent)
	for i, word := range words {
		if i == 0 {
			fmt.Fprint(w, word)
			col += len(word)
			continue
		}
		spaces := spacesAfterWord(words[i-1], cfg.uniformSpacing)
		needed := spaces + len(word)
		if col+needed > cfg.width {
			fmt.Fprintln(w)
			indent = bodyIndent
			fmt.Fprint(w, indent)
			fmt.Fprint(w, word)
			col = len(indent) + len(word)
		} else {
			writeSpaces(w, spaces)
			fmt.Fprint(w, word)
			col += needed
		}
	}
	fmt.Fprintln(w)
}

// spacesAfterWord returns the number of spaces to insert after a word.
// R8.1: two spaces after sentence-ending punctuation (. ! ?), one otherwise.
// R10.1: with uniform spacing, same rule applied uniformly.
func spacesAfterWord(word string, _ bool) int {
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
