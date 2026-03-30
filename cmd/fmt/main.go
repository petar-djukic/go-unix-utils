// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fmt: a simple text formatter.
// Implements prd070-fmt R1.1, R2.1, R3.1, R4.1, R5.1, R6.1, R7.1, R8.1, R9.1, R10.1, R11.1, R12.1,
// R13.1, R13.2, R13.3, R13.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultWidth = 75
	goalPercent  = 93
)

// fmtConfig holds formatting options parsed from command-line arguments.
type fmtConfig struct {
	width           int
	goal            int
	splitOnly       bool
	uniformSpacing  bool
	prefix          string
	taggedParagraph bool
	files           []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "fmt: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(cfg, os.Stdout))
}

// parseArgs parses command-line arguments into a fmtConfig.
func parseArgs(args []string) (fmtConfig, error) {
	cfg := fmtConfig{width: defaultWidth, goal: -1}
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if err := parseFlag(&cfg, args, &i); err != nil {
			return cfg, err
		}
		i++
	}
	if cfg.goal < 0 {
		cfg.goal = computeGoal(cfg.width)
	}
	return cfg, nil
}

// parseFlag dispatches a single flag or positional argument.
func parseFlag(cfg *fmtConfig, args []string, i *int) error {
	arg := args[*i]
	var err error
	switch {
	case arg == "-" || !strings.HasPrefix(arg, "-"):
		cfg.files = append(cfg.files, arg)
	case arg == "--width" || strings.HasPrefix(arg, "--width="):
		cfg.width, err = parseIntOpt(arg, "--width", args, i)
	case strings.HasPrefix(arg, "-w"):
		cfg.width, err = parseIntOpt(arg, "-w", args, i)
	case arg == "--goal" || strings.HasPrefix(arg, "--goal="):
		cfg.goal, err = parseIntOpt(arg, "--goal", args, i)
	case strings.HasPrefix(arg, "-g"):
		cfg.goal, err = parseIntOpt(arg, "-g", args, i)
	case arg == "-s" || arg == "--split-only":
		cfg.splitOnly = true
	case arg == "-u" || arg == "--uniform-spacing":
		cfg.uniformSpacing = true
	case arg == "-t" || arg == "--tagged-paragraph":
		cfg.taggedParagraph = true
	case arg == "--prefix" || strings.HasPrefix(arg, "--prefix="):
		cfg.prefix, err = parseStringOpt(arg, "--prefix", args, i)
	case strings.HasPrefix(arg, "-p"):
		cfg.prefix, err = parseStringOpt(arg, "-p", args, i)
	default:
		return fmt.Errorf("invalid option -- '%s'", arg)
	}
	return err
}

// optValue extracts the value for a short (-w30) or long (--width=30) option.
func optValue(arg, flag string, args []string, i *int) (string, error) {
	if idx := strings.IndexByte(arg, '='); idx >= 0 {
		return arg[idx+1:], nil
	}
	if rest := arg[len(flag):]; rest != "" {
		return rest, nil
	}
	if *i+1 >= len(args) {
		return "", fmt.Errorf("option '%s' requires an argument", flag)
	}
	*i++
	return args[*i], nil
}

// parseIntOpt parses a positive integer value from an option argument.
func parseIntOpt(arg, flag string, args []string, i *int) (int, error) {
	val, err := optValue(arg, flag, args, i)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid %s value: '%s'", flag, val)
	}
	return n, nil
}

// parseStringOpt parses a string value from an option argument.
func parseStringOpt(arg, flag string, args []string, i *int) (string, error) {
	return optValue(arg, flag, args, i)
}

// run reads from files (or stdin) and formats output. Returns exit code.
func run(cfg fmtConfig, w io.Writer) int {
	if len(cfg.files) == 0 {
		formatReader(os.Stdin, w, cfg)
		return 0
	}
	exitCode := 0
	for _, f := range cfg.files {
		if err := formatFile(f, w, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "fmt: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// formatFile opens a file (or stdin for "-") and formats it.
func formatFile(name string, w io.Writer, cfg fmtConfig) error {
	if name == "-" {
		formatReader(os.Stdin, w, cfg)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	formatReader(f, w, cfg)
	return nil
}

// formatReader routes input through the appropriate formatting strategy.
func formatReader(r io.Reader, w io.Writer, cfg fmtConfig) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	scanner := bufio.NewScanner(r)
	if cfg.prefix != "" {
		formatWithPrefix(bw, scanner, cfg)
		return
	}
	if cfg.splitOnly {
		formatSplitOnly(bw, scanner, cfg)
		return
	}
	formatParagraphs(bw, scanner, cfg)
}

// formatParagraphs collects lines into paragraphs and formats them.
// R2.1: blank lines separate paragraphs.
// R3.1: indentation changes separate paragraphs.
// R12.1: tagged-paragraph mode allows first line to differ in indent.
func formatParagraphs(bw *bufio.Writer, scanner *bufio.Scanner, cfg fmtConfig) {
	var para []string
	var bodyIndent string
	for scanner.Scan() {
		line := scanner.Text()
		if isBlank(line) {
			flushParagraph(bw, para, cfg)
			para = para[:0]
			bw.WriteByte('\n')
			continue
		}
		indent := extractIndent(line)
		if len(para) > 0 && shouldBreakPara(para, indent, bodyIndent, cfg) {
			flushParagraph(bw, para, cfg)
			para = para[:0]
		}
		if len(para) == 0 {
			bodyIndent = indent
		} else if len(para) == 1 && cfg.taggedParagraph {
			bodyIndent = indent
		}
		para = append(para, line)
	}
	flushParagraph(bw, para, cfg)
}

// shouldBreakPara determines if a new line starts a new paragraph.
func shouldBreakPara(para []string, indent, bodyIndent string, cfg fmtConfig) bool {
	if cfg.taggedParagraph && len(para) == 1 {
		return false
	}
	return indent != bodyIndent
}

// formatSplitOnly processes each line independently.
// R9.1: splits long lines but does not join short lines.
func formatSplitOnly(bw *bufio.Writer, scanner *bufio.Scanner, cfg fmtConfig) {
	for scanner.Scan() {
		line := scanner.Text()
		if isBlank(line) {
			bw.WriteByte('\n')
			continue
		}
		if len(line) <= cfg.width {
			bw.WriteString(line)
			bw.WriteByte('\n')
			continue
		}
		splitLongLine(bw, line, cfg)
	}
}

// splitLongLine splits a single long line at word boundaries.
func splitLongLine(bw *bufio.Writer, line string, cfg fmtConfig) {
	indent := extractIndent(line)
	text := strings.TrimSpace(line)
	words, gaps := parseText(text)
	if cfg.uniformSpacing {
		gaps = uniformGaps(gaps)
	}
	fillLines(bw, words, gaps, indent, indent, cfg.width, cfg.goal)
}

// formatWithPrefix handles prefix mode.
// R11.1: only reformat lines beginning with PREFIX, processed independently.
func formatWithPrefix(bw *bufio.Writer, scanner *bufio.Scanner, cfg fmtConfig) {
	for scanner.Scan() {
		line := scanner.Text()
		if isBlank(line) {
			bw.WriteByte('\n')
			continue
		}
		ws, ok := matchPrefix(line, cfg.prefix)
		if !ok {
			bw.WriteString(line)
			bw.WriteByte('\n')
			continue
		}
		stripped := stripAfterPrefix(line, cfg.prefix)
		fmtPrefixLine(bw, stripped, ws+cfg.prefix, cfg)
	}
}

// matchPrefix checks if line starts with optional whitespace followed by prefix.
func matchPrefix(line, prefix string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	return line[:len(line)-len(trimmed)], true
}

// stripAfterPrefix returns text after whitespace and prefix.
func stripAfterPrefix(line, prefix string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return trimmed[len(prefix):]
}

// fmtPrefixLine formats one stripped prefix line and re-adds the prefix.
func fmtPrefixLine(bw *bufio.Writer, line, pfx string, cfg fmtConfig) {
	subWidth := max(cfg.width-len(pfx), 1)
	if len(line) <= subWidth && !cfg.uniformSpacing {
		bw.WriteString(pfx)
		bw.WriteString(line)
		bw.WriteByte('\n')
		return
	}
	subCfg := cfg
	subCfg.width = subWidth
	subCfg.goal = computeGoal(subWidth)
	subCfg.prefix = ""
	var buf strings.Builder
	inner := bufio.NewWriter(&buf)
	flushParagraph(inner, []string{line}, subCfg)
	inner.Flush()
	prependPrefix(bw, buf.String(), pfx)
}

// prependPrefix adds prefix to each line of formatted text.
func prependPrefix(bw *bufio.Writer, text, prefix string) {
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		bw.WriteString(prefix)
		bw.WriteString(line)
		bw.WriteByte('\n')
	}
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// flushParagraph formats and writes a paragraph.
// R10.1: uniform spacing normalizes gaps when enabled.
func flushParagraph(w *bufio.Writer, lines []string, cfg fmtConfig) {
	if len(lines) == 0 {
		return
	}
	if !cfg.uniformSpacing && len(lines) == 1 && len(lines[0]) <= cfg.width {
		w.WriteString(lines[0])
		w.WriteByte('\n')
		return
	}
	firstIndent := extractIndent(lines[0])
	bodyIndent := firstIndent
	if len(lines) > 1 {
		bodyIndent = extractIndent(lines[1])
	}
	text := joinParagraphText(lines)
	words, gaps := parseText(text)
	if len(words) == 0 {
		return
	}
	if cfg.uniformSpacing {
		gaps = uniformGaps(gaps)
	}
	fillLines(w, words, gaps, firstIndent, bodyIndent, cfg.width, cfg.goal)
}

func computeGoal(width int) int {
	g := width * goalPercent / 100
	return max(g, 1)
}

// extractIndent returns the leading whitespace of a line.
func extractIndent(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

// joinParagraphText joins paragraph lines into a single string.
// Strips indentation from each line. Uses two spaces between lines
// when the previous line ends with sentence-ending punctuation.
func joinParagraphText(lines []string) string {
	var sb strings.Builder
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i > 0 {
			prev := strings.TrimSpace(lines[i-1])
			if isSentenceEnd(prev) {
				sb.WriteString("  ")
			} else {
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(trimmed)
	}
	return sb.String()
}

// parseText splits text into words and inter-word gap sizes.
// R8.1: preserves original spacing between words.
func parseText(text string) ([]string, []int) {
	var words []string
	var gaps []int
	i := 0
	n := len(text)
	for i < n {
		for i < n && text[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}
		start := i
		for i < n && text[i] != ' ' {
			i++
		}
		words = append(words, text[start:i])
		gapStart := i
		for i < n && text[i] == ' ' {
			i++
		}
		gaps = append(gaps, i-gapStart)
	}
	return words, gaps
}

// uniformGaps normalizes all gaps to exactly one space.
// R10.1: uniform spacing mode.
func uniformGaps(gaps []int) []int {
	result := make([]int, len(gaps))
	for i := range gaps {
		result[i] = 1
	}
	return result
}

// isSentenceEnd checks if text ends with sentence-ending punctuation,
// optionally followed by closing characters like quotes or brackets.
func isSentenceEnd(text string) bool {
	i := len(text) - 1
	for i >= 0 && isClosePunct(text[i]) {
		i--
	}
	if i < 0 {
		return false
	}
	return text[i] == '.' || text[i] == '!' || text[i] == '?'
}

func isClosePunct(c byte) bool {
	return c == '"' || c == '\'' || c == ')' || c == ']'
}

// fillLines fills words into lines using optimal line breaking.
// R7.1: breaks at word boundaries only.
func fillLines(
	w *bufio.Writer, words []string, gaps []int,
	firstIndent, bodyIndent string, width, goal int,
) {
	breaks := optimalBreaks(words, gaps, len(firstIndent), len(bodyIndent), width, goal)
	emitLines(w, words, gaps, breaks, firstIndent, bodyIndent)
}

// optimalBreaks computes line break positions using dynamic programming.
// Uses asymmetric cost: overshoot penalized 10x more than undershoot.
func optimalBreaks(
	words []string, gaps []int, firstInd, bodyInd, width, goal int,
) []int {
	n := len(words)
	const inf = 1 << 62
	cost := make([]int, n+1)
	from := make([]int, n+1)
	for i := 1; i <= n; i++ {
		cost[i] = inf
	}
	for i := 0; i < n; i++ {
		if cost[i] >= inf {
			continue
		}
		ind := bodyInd
		if i == 0 {
			ind = firstInd
		}
		tryBreaks(words, gaps, cost, from, i, n, ind, width, goal)
	}
	return reconstructBreaks(from, n)
}

// tryBreaks evaluates all possible line endings starting at word start.
func tryBreaks(
	words []string, gaps, cost, from []int,
	start, n, ind, width, goal int,
) {
	lineLen := ind
	for j := start; j < n; j++ {
		if j == start {
			lineLen += len(words[j])
		} else {
			lineLen += gaps[j-1] + len(words[j])
		}
		if lineLen > width && j > start {
			break
		}
		lc := squareDiff(lineLen, goal)
		if j+1 == n {
			lc = 0 // no penalty for last line
		}
		if cost[start]+lc < cost[j+1] {
			cost[j+1] = cost[start] + lc
			from[j+1] = start
		}
	}
}

func squareDiff(a, b int) int {
	d := a - b
	return d * d
}

// reconstructBreaks builds the list of line-start indices from the DP result.
func reconstructBreaks(from []int, n int) []int {
	var breaks []int
	for pos := n; pos > 0; pos = from[pos] {
		breaks = append(breaks, pos)
	}
	breaks = append(breaks, 0)
	for l, r := 0, len(breaks)-1; l < r; l, r = l+1, r-1 {
		breaks[l], breaks[r] = breaks[r], breaks[l]
	}
	return breaks
}

// emitLines writes the formatted output using the computed break positions.
func emitLines(
	w *bufio.Writer, words []string, gaps, breaks []int,
	firstIndent, bodyIndent string,
) {
	for b := range len(breaks) - 1 {
		start := breaks[b]
		end := breaks[b+1]
		indent := bodyIndent
		if b == 0 {
			indent = firstIndent
		}
		w.WriteString(indent)
		for k := start; k < end; k++ {
			if k > start {
				writeSpaces(w, gaps[k-1])
			}
			w.WriteString(words[k])
		}
		w.WriteByte('\n')
	}
}

// writeSpaces writes n space characters to w.
func writeSpaces(w *bufio.Writer, n int) {
	for range n {
		w.WriteByte(' ')
	}
}
