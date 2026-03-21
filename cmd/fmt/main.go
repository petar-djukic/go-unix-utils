// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd070-fmt R1.1–R1.4: basic paragraph filling and argument parsing.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "fmt"

// defaultMaxWidth is the default maximum line width (R1.1).
const defaultMaxWidth = 75

// defaultGoalWidth is the default goal line width (93% of max).
const defaultGoalWidth = 69

// fmtWord holds a word and whether it was the last word on its input line.
type fmtWord struct {
	text       string
	endOfLine  bool // true if this was the last word on an input line
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and formats files, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	w := bufio.NewWriter(stdout)
	exitCode := 0
	for _, name := range files {
		if err := formatFile(name, stdin, w); err != nil {
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

// parseArgs separates flags from file arguments.
// Returns file list and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) ([]string, int) {
	var files []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if code := handleFlag(arg, stdout, stderr); code >= 0 {
			return nil, code
		}
	}
	return files, -1
}

// handleFlag processes a single flag argument.
// Returns exit code >= 0 for terminal/error flags, -1 to continue.
func handleFlag(arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
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
	fmt.Fprintln(w, "      --help               display this help and exit")
	fmt.Fprintln(w, "      --version            output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// formatFile opens a file (or stdin for "-") and formats its paragraphs.
func formatFile(name string, stdin io.Reader, w *bufio.Writer) error {
	if name == "-" {
		return formatReader(stdin, w)
	}
	f, err := os.Open(name)
	if err != nil {
		return unwrapPathError(err)
	}
	defer f.Close() // best-effort close on read-only file
	return formatReader(f, w)
}

// formatReader reads input and formats paragraphs.
// R1.2: blank lines separate paragraphs.
// R1.3: indentation changes also separate paragraphs.
func formatReader(r io.Reader, w *bufio.Writer) error {
	scanner := bufio.NewScanner(r)
	var lines []string
	var lastIndent string
	first := true

	for scanner.Scan() {
		line := scanner.Text()
		if isBlankLine(line) {
			if err := flushLines(lines, w); err != nil {
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
			if err := flushLines(lines, w); err != nil {
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
	return flushLines(lines, w)
}

// isBlankLine returns true if the line is empty or contains only whitespace.
func isBlankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// flushLines formats and outputs collected paragraph lines.
func flushLines(lines []string, w *bufio.Writer) error {
	if len(lines) == 0 {
		return nil
	}
	return writeParagraph(lines, w)
}

// writeParagraph formats a paragraph's lines to the output.
// R1.3: first line's indentation is preserved; subsequent lines use
// the second line's indentation (or first if only one line).
func writeParagraph(lines []string, w *bufio.Writer) error {
	firstIndent := extractIndent(lines[0])
	subIndent := firstIndent
	if len(lines) > 1 {
		subIndent = extractIndent(lines[1])
	}
	words := collectWords(lines)
	if len(words) == 0 {
		return nil
	}
	breaks := optimalBreaks(words, firstIndent, subIndent)
	return emitLines(words, breaks, firstIndent, subIndent, w)
}

// extractIndent returns the leading whitespace of a line.
func extractIndent(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

// collectWords extracts words from paragraph lines, tracking line boundaries.
// A word that was the last on its input line is marked endOfLine=true.
// GNU fmt only adds double space after sentence punctuation at line ends.
func collectWords(lines []string) []fmtWord {
	var words []fmtWord
	for _, line := range lines {
		fields := strings.Fields(line)
		for i, f := range fields {
			words = append(words, fmtWord{
				text:      f,
				endOfLine: i == len(fields)-1,
			})
		}
	}
	return words
}

// endsWithSentence returns true if word ends with sentence punctuation.
func endsWithSentence(word string) bool {
	if len(word) == 0 {
		return false
	}
	last := word[len(word)-1]
	return last == '.' || last == '!' || last == '?'
}

// spacesAfterWord returns the inter-word space count after a fmtWord.
// Two spaces after sentence-ending punctuation at line boundaries, one otherwise.
func spacesAfterWord(w fmtWord) int {
	if w.endOfLine && endsWithSentence(w.text) {
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
// Returns a slice of indices where each new line starts.
func optimalBreaks(words []fmtWord, firstIndent, subIndent string) []int {
	n := len(words)
	cost := make([]float64, n+1)
	prev := make([]int, n+1)
	for i := 1; i <= n; i++ {
		cost[i] = math.Inf(1)
	}
	for i := range n {
		computeBreaksFrom(i, n, words, firstIndent, subIndent, cost, prev)
	}
	return reconstructBreaks(n, prev)
}

// computeBreaksFrom tries extending lines starting at word i.
func computeBreaksFrom(i, n int, words []fmtWord, firstIndent, subIndent string, cost []float64, prev []int) {
	for j := i + 1; j <= n; j++ {
		indent := subIndent
		if i == 0 {
			indent = firstIndent
		}
		w := lineWidth(words, i, j, indent)
		if w > defaultMaxWidth {
			break
		}
		c := lineCost(w, j == n)
		total := cost[i] + c
		if total < cost[j] {
			cost[j] = total
			prev[j] = i
		}
	}
}

// lineCost computes the cost of a line of given width.
// Last line has zero cost. Other lines penalize deviation from goal.
func lineCost(width int, isLast bool) float64 {
	if isLast {
		return 0
	}
	diff := float64(defaultGoalWidth - width)
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
