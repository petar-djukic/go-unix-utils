// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fmt: a simple text formatter.
// Implements prd070-fmt R1.1, R2.1, R3.1, R4.1, R5.1, R6.1, R7.1, R8.1.
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
	width int
	goal  int
	files []string
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
// R5.1: -w/--width sets max line width. R6.1: -g/--goal sets goal width.
func parseArgs(args []string) (fmtConfig, error) {
	cfg := fmtConfig{width: defaultWidth, goal: -1}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		var err error
		switch {
		case arg == "-" || !strings.HasPrefix(arg, "-"):
			cfg.files = append(cfg.files, arg)
		case arg == "--width" || strings.HasPrefix(arg, "--width="):
			cfg.width, err = parseIntOpt(arg, "--width", args, &i)
		case strings.HasPrefix(arg, "-w"):
			cfg.width, err = parseIntOpt(arg, "-w", args, &i)
		case arg == "--goal" || strings.HasPrefix(arg, "--goal="):
			cfg.goal, err = parseIntOpt(arg, "--goal", args, &i)
		case strings.HasPrefix(arg, "-g"):
			cfg.goal, err = parseIntOpt(arg, "-g", args, &i)
		default:
			err = fmt.Errorf("invalid option -- '%s'", arg)
		}
		if err != nil {
			return cfg, err
		}
		i++
	}
	if cfg.goal < 0 {
		cfg.goal = computeGoal(cfg.width)
	}
	return cfg, nil
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

// formatReader reads all paragraphs from r and writes formatted output.
// R2.1: blank lines separate paragraphs.
// R3.1: indentation changes also separate paragraphs.
func formatReader(r io.Reader, w io.Writer, cfg fmtConfig) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	scanner := bufio.NewScanner(r)

	var para []string
	var paraIndent string

	for scanner.Scan() {
		line := scanner.Text()
		if isBlank(line) {
			flushParagraph(bw, para, cfg)
			para = para[:0]
			bw.WriteByte('\n')
			continue
		}
		indent := extractIndent(line)
		if len(para) > 0 && indent != paraIndent {
			flushParagraph(bw, para, cfg)
			para = para[:0]
		}
		if len(para) == 0 {
			paraIndent = indent
		}
		para = append(para, line)
	}
	flushParagraph(bw, para, cfg)
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// flushParagraph formats and writes a paragraph.
// Single lines within width pass through unchanged (matches GNU fmt behavior).
func flushParagraph(w *bufio.Writer, lines []string, cfg fmtConfig) {
	if len(lines) == 0 {
		return
	}
	if len(lines) == 1 && len(lines[0]) <= cfg.width {
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
// Strips indentation from each line and joins with a single space.
func joinParagraphText(lines []string) string {
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strings.TrimSpace(line))
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
// Minimizes sum of (lineWidth - goal)^2 with zero cost for the last line.
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
