// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fmt: simple text formatter.
// Implements srd070-fmt R1.1, R2.1, R3.1, R4.1, R5.1, R6.1, R7.1, R8.1.
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

// fmtConfig holds formatting parameters. R5.1, R6.1.
type fmtConfig struct {
	width int
	goal  int
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
// R4.1: files from args; stdin when no file or "-".
// R5.1: -w WIDTH / --width=WIDTH. R6.1: -g GOAL / --goal=GOAL.
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
		if v, ok, err := matchFlag(arg, args, &i, "-w", "--width"); ok {
			if err != nil {
				return cfg, nil, err
			}
			cfg.width, err = parsePositiveInt(v, "width")
			if err != nil {
				return cfg, nil, err
			}
			continue
		}
		if v, ok, err := matchFlag(arg, args, &i, "-g", "--goal"); ok {
			if err != nil {
				return cfg, nil, err
			}
			cfg.goal, err = parsePositiveInt(v, "goal")
			if err != nil {
				return cfg, nil, err
			}
			goalExplicit = true
			continue
		}
		files = append(files, arg)
	}
	if !goalExplicit {
		cfg.goal = cfg.width * defaultGoalPct / 100
	}
	if cfg.goal > cfg.width {
		cfg.goal = cfg.width
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	return cfg, files, nil
}

// matchFlag checks if arg matches the short or long form of a flag.
// Returns (value, matched, error). Advances *idx if value is in the next arg.
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
// R1.1: reformat to fit within width. R2.1: blank lines separate paragraphs.
// R3.1: preserve indentation.
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
// R3.1: preserve first-line indentation; fill to second line's indent.
func writeParagraph(w *bufio.Writer, lines []string, cfg fmtConfig, _ bool) {
	firstIndent := leadingWhitespace(lines[0])
	bodyIndent := firstIndent
	if len(lines) > 1 {
		bodyIndent = leadingWhitespace(lines[1])
	}
	words := collectWords(lines)
	writeWrapped(w, words, firstIndent, bodyIndent, cfg)
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
// R8.1: uses Fields to collapse multiple spaces during extraction.
func collectWords(lines []string) []string {
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
// R1.1: fit within width. R3.1: first line uses firstIndent, rest use bodyIndent.
// R7.1: break at word boundaries. R8.1: two spaces after sentence-ending punctuation.
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
		// R8.1: two spaces after sentence-ending punctuation.
		spaces := spacesAfter(words[i-1])
		needed := spaces + len(word)
		if col+needed > cfg.width {
			// R7.1: break at word boundary; long words go on their own line.
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

// spacesAfter returns the number of spaces to insert after a word.
// R8.1: two spaces after sentence-ending punctuation (. ! ?), one otherwise.
func spacesAfter(word string) int {
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
