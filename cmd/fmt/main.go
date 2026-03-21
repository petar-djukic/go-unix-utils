// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd070-fmt R1.1–R1.4: basic paragraph filling and argument parsing.
// Implements prd070-fmt R2.1–R2.4: width control, goal width, word breaking, space collapsing.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "fmt"

// defaultMaxWidth is the default maximum line width (R1.1, R2.1).
const defaultMaxWidth = 75

// goalPercent is the percentage of width used for the default goal (R2.2).
const goalPercent = 93

// fmtConfig holds formatting configuration parsed from command-line flags.
type fmtConfig struct {
	maxWidth  int
	goalWidth int
	goalSet   bool // true if -g/--goal was explicitly provided
}

// defaultConfig returns the default formatting configuration.
func defaultConfig() fmtConfig {
	return fmtConfig{
		maxWidth:  defaultMaxWidth,
		goalWidth: defaultMaxWidth * goalPercent / 100,
	}
}

// fmtWord holds a word, its original trailing whitespace count, and
// whether it was the last word on its input line.
type fmtWord struct {
	text       string
	trailSpace int  // original spaces after this word on the same input line
	endOfLine  bool // true if this was the last word on an input line
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and formats files, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	w := bufio.NewWriter(stdout)
	exitCode := 0
	for _, name := range files {
		if err := formatFile(name, stdin, w, cfg); err != nil {
			fmt.Fprintf(stderr, "%s: cannot open '%s' for reading: %s\n",
				progName, name, capitalizeFirst(err.Error()))
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// capitalizeFirst returns s with its first character uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parseArgs separates flags from file arguments and builds fmtConfig.
// Returns config, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (fmtConfig, []string, int) {
	cfg := defaultConfig()
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || isFileArg(arg) {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		consumed, code := processFlag(args, i, &cfg, stdout, stderr)
		if code >= 0 {
			return cfg, nil, code
		}
		i += consumed - 1
	}
	finalizeGoal(&cfg)
	return cfg, files, -1
}

// finalizeGoal sets goal to 93% of width if not explicit, and clamps.
func finalizeGoal(cfg *fmtConfig) {
	if !cfg.goalSet {
		cfg.goalWidth = cfg.maxWidth * goalPercent / 100
	}
	if cfg.goalWidth > cfg.maxWidth {
		cfg.goalWidth = cfg.maxWidth
	}
}

// isFileArg returns true if the argument is a file name, not a flag.
func isFileArg(arg string) bool {
	return arg == "-" || len(arg) == 0 || arg[0] != '-'
}

// processFlag handles a single flag argument. Returns consumed arg
// count and exit code (-1 = continue).
func processFlag(args []string, idx int, cfg *fmtConfig, stdout, stderr io.Writer) (int, int) {
	arg := args[idx]
	switch {
	case arg == "--help":
		printHelp(stdout)
		return 1, 0
	case arg == "--version":
		printVersion(stdout)
		return 1, 0
	case strings.HasPrefix(arg, "--width="):
		return parseSetWidth(arg[len("--width="):], cfg, stderr, 1)
	case arg == "--width" && idx+1 < len(args):
		return parseSetWidth(args[idx+1], cfg, stderr, 2)
	case strings.HasPrefix(arg, "--goal="):
		return parseSetGoal(arg[len("--goal="):], cfg, stderr, 1)
	case arg == "--goal" && idx+1 < len(args):
		return parseSetGoal(args[idx+1], cfg, stderr, 2)
	case arg == "-w" && idx+1 < len(args):
		return parseSetWidth(args[idx+1], cfg, stderr, 2)
	case len(arg) > 2 && arg[:2] == "-w":
		return parseSetWidth(arg[2:], cfg, stderr, 1)
	case arg == "-g" && idx+1 < len(args):
		return parseSetGoal(args[idx+1], cfg, stderr, 2)
	case len(arg) > 2 && arg[:2] == "-g":
		return parseSetGoal(arg[2:], cfg, stderr, 1)
	default:
		return handleDefaultFlag(arg, cfg, stderr)
	}
}

// handleDefaultFlag checks for -NUMBER shorthand or reports unknown flag.
func handleDefaultFlag(arg string, cfg *fmtConfig, stderr io.Writer) (int, int) {
	if n, err := parseNumericArg(arg); err == nil {
		cfg.maxWidth = n
		return 1, -1
	}
	fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
	printTryHelp(stderr)
	return 1, 1
}

// parseNumericArg parses -NUMBER shorthand (e.g., -72 sets width to 72).
func parseNumericArg(arg string) (int, error) {
	if len(arg) < 2 || arg[0] != '-' {
		return 0, fmt.Errorf("not numeric")
	}
	n, err := strconv.Atoi(arg[1:])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid width")
	}
	return n, nil
}

// parseSetWidth parses and sets the width in cfg. R2.1.
func parseSetWidth(val string, cfg *fmtConfig, stderr io.Writer, consumed int) (int, int) {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		fmt.Fprintf(stderr, "%s: invalid width: '%s'\n", progName, val)
		return consumed, 1
	}
	cfg.maxWidth = n
	return consumed, -1
}

// parseSetGoal parses and sets the goal width in cfg. R2.2.
func parseSetGoal(val string, cfg *fmtConfig, stderr io.Writer, consumed int) (int, int) {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		fmt.Fprintf(stderr, "%s: invalid goal: '%s'\n", progName, val)
		return consumed, 1
	}
	cfg.goalWidth = n
	cfg.goalSet = true
	return consumed, -1
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [-WIDTH] [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Reformat each paragraph in the FILE(s), writing to standard output.")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -w, --width=WIDTH        maximum line width (default of 75 columns)")
	fmt.Fprintln(w, "  -g, --goal=GOAL          goal width (default of 93% of width)")
	fmt.Fprintln(w, "      --help               display this help and exit")
	fmt.Fprintln(w, "      --version            output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// formatFile opens a file (or stdin for "-") and formats its paragraphs.
func formatFile(name string, stdin io.Reader, w *bufio.Writer, cfg fmtConfig) error {
	if name == "-" {
		return formatReader(stdin, w, cfg)
	}
	f, err := os.Open(name)
	if err != nil {
		return unwrapPathError(err)
	}
	defer f.Close() // best-effort close on read-only file
	return formatReader(f, w, cfg)
}

// formatReader reads input and formats paragraphs.
// R1.2: blank lines separate paragraphs.
// R1.3: indentation changes also separate paragraphs.
func formatReader(r io.Reader, w *bufio.Writer, cfg fmtConfig) error {
	scanner := bufio.NewScanner(r)
	var lines []string
	var lastIndent string
	first := true

	for scanner.Scan() {
		line := scanner.Text()
		if isBlankLine(line) {
			if err := flushLines(lines, w, cfg); err != nil {
				return err
			}
			lines = nil
			first = true
			if _, err := w.WriteString("\n"); err != nil {
				return err
			}
			continue
		}
		indent := extractIndent(line)
		if !first && indent != lastIndent {
			if err := flushLines(lines, w, cfg); err != nil {
				return err
			}
			lines = nil
		}
		lines = append(lines, line)
		lastIndent = indent
		first = false
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flushLines(lines, w, cfg)
}

// isBlankLine returns true if the line is empty or contains only whitespace.
func isBlankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// flushLines formats and outputs collected paragraph lines.
func flushLines(lines []string, w *bufio.Writer, cfg fmtConfig) error {
	if len(lines) == 0 {
		return nil
	}
	return writeParagraph(lines, w, cfg)
}

// writeParagraph formats a paragraph's lines to the output.
// R1.3: first line's indentation is preserved; subsequent lines use
// the second line's indentation (or first if only one line).
func writeParagraph(lines []string, w *bufio.Writer, cfg fmtConfig) error {
	firstIndent := extractIndent(lines[0])
	subIndent := firstIndent
	if len(lines) > 1 {
		subIndent = extractIndent(lines[1])
	}
	words := collectWords(lines)
	if len(words) == 0 {
		return nil
	}
	breaks := optimalBreaks(words, firstIndent, subIndent, cfg)
	return emitLines(words, breaks, firstIndent, subIndent, w)
}

// extractIndent returns the leading whitespace of a line.
func extractIndent(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

// collectWords extracts words from paragraph lines, tracking line boundaries
// and preserving original inter-word spacing within each line.
// R2.4: when lines are joined, standard spacing is used (1 space, or 2 after
// sentence-ending punctuation). Within a line, original spacing is preserved.
func collectWords(lines []string) []fmtWord {
	var words []fmtWord
	for _, line := range lines {
		words = append(words, extractLineWords(line)...)
	}
	return words
}

// extractLineWords parses a single input line into words, preserving the
// original inter-word spacing.
func extractLineWords(line string) []fmtWord {
	s := strings.TrimLeft(line, " \t")
	var words []fmtWord
	for len(s) > 0 {
		wordEnd := strings.IndexAny(s, " \t")
		if wordEnd == -1 {
			words = append(words, fmtWord{text: s, endOfLine: true})
			break
		}
		word := s[:wordEnd]
		rest := strings.TrimLeft(s[wordEnd:], " \t")
		spaces := len(s) - wordEnd - len(rest)
		s = rest
		words = append(words, fmtWord{
			text:       word,
			trailSpace: spaces,
			endOfLine:  len(s) == 0,
		})
	}
	return words
}

// endsWithSentence returns true if word ends with sentence punctuation.
// R2.4: sentence-ending punctuation is . ! ?
func endsWithSentence(word string) bool {
	if len(word) == 0 {
		return false
	}
	last := word[len(word)-1]
	return last == '.' || last == '!' || last == '?'
}

// spacesAfterWord returns the inter-word space count after a fmtWord.
// R2.4: within the same input line, original spacing is preserved.
// When joining across lines, two spaces after sentence-ending punctuation,
// one space otherwise.
func spacesAfterWord(w fmtWord) int {
	if !w.endOfLine {
		return w.trailSpace
	}
	if endsWithSentence(w.text) {
		return 2
	}
	return 1
}

// lineWidth computes the printed width of words[from..to-1] on a line.
func lineWidth(words []fmtWord, from, to int, indent string) int {
	w := len(indent)
	for i := from; i < to; i++ {
		if i > from {
			w += spacesAfterWord(words[i-1])
		}
		w += len(words[i].text)
	}
	return w
}

// optimalBreaks uses DP to find optimal line break positions.
// R2.1: uses cfg.maxWidth. R2.2: uses cfg.goalWidth.
// Returns a slice of indices where each new line starts.
func optimalBreaks(words []fmtWord, firstIndent, subIndent string, cfg fmtConfig) []int {
	n := len(words)
	cost := make([]float64, n+1)
	prev := make([]int, n+1)
	for i := 1; i <= n; i++ {
		cost[i] = math.Inf(1)
	}
	for i := range n {
		indent := subIndent
		if i == 0 {
			indent = firstIndent
		}
		computeBreaksFrom(i, n, words, indent, cfg, cost, prev)
	}
	return reconstructBreaks(n, prev)
}

// computeBreaksFrom tries extending lines starting at word i.
// R2.3: a single word that exceeds maxWidth gets its own line.
func computeBreaksFrom(i, n int, words []fmtWord, indent string, cfg fmtConfig, cost []float64, prev []int) {
	for j := i + 1; j <= n; j++ {
		w := lineWidth(words, i, j, indent)
		overlong := w > cfg.maxWidth
		if overlong && j > i+1 {
			break
		}
		c := lineCost(w, cfg.goalWidth, j == n)
		total := cost[i] + c
		if total < cost[j] {
			cost[j] = total
			prev[j] = i
		}
		if overlong {
			break
		}
	}
}

// lineCost computes the cost of a line of given width. R2.2.
// Last line has zero cost. Other lines penalize deviation from goal.
func lineCost(width, goalWidth int, isLast bool) float64 {
	if isLast {
		return 0
	}
	diff := float64(goalWidth - width)
	return diff * diff
}

// reconstructBreaks traces back through prev to find line start indices.
func reconstructBreaks(n int, prev []int) []int {
	var breaks []int
	for pos := n; pos > 0; pos = prev[pos] {
		breaks = append(breaks, prev[pos])
	}
	// Reverse to get forward order.
	for i, j := 0, len(breaks)-1; i < j; i, j = i+1, j-1 {
		breaks[i], breaks[j] = breaks[j], breaks[i]
	}
	return breaks
}

// emitLines writes the formatted paragraph using computed break points.
func emitLines(words []fmtWord, breaks []int, firstIndent, subIndent string, w *bufio.Writer) error {
	for lineIdx := range breaks {
		from := breaks[lineIdx]
		to := len(words)
		if lineIdx+1 < len(breaks) {
			to = breaks[lineIdx+1]
		}
		indent := subIndent
		if lineIdx == 0 {
			indent = firstIndent
		}
		if err := emitOneLine(words, from, to, indent, w); err != nil {
			return err
		}
	}
	return nil
}

// emitOneLine writes words[from..to-1] as a single output line.
func emitOneLine(words []fmtWord, from, to int, indent string, w *bufio.Writer) error {
	if _, err := w.WriteString(indent); err != nil {
		return err
	}
	for i := from; i < to; i++ {
		if i > from {
			spaces := spacesAfterWord(words[i-1])
			for range spaces {
				if err := w.WriteByte(' '); err != nil {
					return err
				}
			}
		}
		if _, err := w.WriteString(words[i].text); err != nil {
			return err
		}
	}
	return w.WriteByte('\n')
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
