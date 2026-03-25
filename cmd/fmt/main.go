// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd070-fmt: Simple Text Formatter.
// Covers R1.1-R1.4 (entry point, paragraph detection, indentation, file input),
// R2.1 (-w width), R2.2 (-g goal), R2.3 (word boundary breaking),
// R2.4 (sentence-ending double space), R3.1 (-s split-only),
// R3.2 (-u uniform-spacing).
//
// TODO: prd070-fmt R4.1 (-p prefix), R4.2 (-t tagged-paragraph)
// are not implemented in this task.
// TODO: -c/--crown-margin is listed in prd070-fmt non_goals; skipped per article E6.
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
	progName     = "fmt"
)

// fmtConfig holds parsed command-line options.
type fmtConfig struct {
	width   int  // R2.1: maximum line width
	goal    int  // R2.2: goal line width
	split   bool // R3.1: split long lines only
	uniform bool // R3.2: uniform spacing
}

func main() {
	// D1: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(1)
	}
	os.Exit(run(cfg, files, os.Stdin, os.Stdout, os.Stderr))
}

// --- Flag parsing ---

// parseArgs parses fmt flags and file arguments.
// R1.4: stdin when no files or "-" given.
func parseArgs(args []string) (fmtConfig, []string, error) {
	cfg := fmtConfig{width: defaultWidth}
	var files []string
	goalSet := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}
		consumed, gs, err := parseFmtFlag(arg, args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
		if gs {
			goalSet = true
		}
		i += consumed
	}
	if !goalSet {
		cfg.goal = cfg.width * goalPercent / 100
	}
	if cfg.goal > cfg.width {
		cfg.goal = cfg.width
	}
	return cfg, files, nil
}

// parseFmtFlag dispatches a single flag and returns args consumed.
func parseFmtFlag(arg string, args []string, i int, cfg *fmtConfig) (int, bool, error) {
	switch {
	case arg == "-s" || arg == "--split-only":
		cfg.split = true
		return 1, false, nil
	case arg == "-u" || arg == "--uniform-spacing":
		// R3.2: uniform spacing mode.
		cfg.uniform = true
		return 1, false, nil
	case strings.HasPrefix(arg, "-w"):
		return parseShortWidth(arg, args, i, cfg)
	case strings.HasPrefix(arg, "--width"):
		return parseLongWidth(arg, args, i, cfg)
	case strings.HasPrefix(arg, "-g"):
		return parseShortGoal(arg, args, i, cfg)
	case strings.HasPrefix(arg, "--goal"):
		return parseLongGoal(arg, args, i, cfg)
	default:
		return 0, false, fmt.Errorf("invalid option -- '%s'", arg)
	}
}

// parseShortWidth handles -wN or -w N.
func parseShortWidth(arg string, args []string, i int, cfg *fmtConfig) (int, bool, error) {
	v, consumed, err := shortNumeric(arg[2:], args, i, "w")
	if err != nil {
		return 0, false, err
	}
	cfg.width = v
	return consumed, false, nil
}

// parseLongWidth handles --width=N or --width N.
func parseLongWidth(arg string, args []string, i int, cfg *fmtConfig) (int, bool, error) {
	v, consumed, err := longNumeric(arg, "--width", args, i)
	if err != nil {
		return 0, false, err
	}
	cfg.width = v
	return consumed, false, nil
}

// parseShortGoal handles -gN or -g N.
func parseShortGoal(arg string, args []string, i int, cfg *fmtConfig) (int, bool, error) {
	v, consumed, err := shortNumeric(arg[2:], args, i, "g")
	if err != nil {
		return 0, false, err
	}
	cfg.goal = v
	return consumed, true, nil
}

// parseLongGoal handles --goal=N or --goal N.
func parseLongGoal(arg string, args []string, i int, cfg *fmtConfig) (int, bool, error) {
	v, consumed, err := longNumeric(arg, "--goal", args, i)
	if err != nil {
		return 0, false, err
	}
	cfg.goal = v
	return consumed, true, nil
}

// shortNumeric parses a short flag value: -fVAL or -f VAL.
func shortNumeric(remainder string, args []string, i int, flag string) (int, int, error) {
	if remainder != "" {
		v, err := parsePositiveInt(remainder)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid width: '%s'", remainder)
		}
		return v, 1, nil
	}
	if i+1 >= len(args) {
		return 0, 0, fmt.Errorf("option requires an argument -- '%s'", flag)
	}
	v, err := parsePositiveInt(args[i+1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: '%s'", args[i+1])
	}
	return v, 2, nil
}

// longNumeric parses a long flag value: --flag=VAL or --flag VAL.
func longNumeric(arg, prefix string, args []string, i int) (int, int, error) {
	if strings.HasPrefix(arg, prefix+"=") {
		val := arg[len(prefix)+1:]
		v, err := parsePositiveInt(val)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid width: '%s'", val)
		}
		return v, 1, nil
	}
	if arg != prefix {
		return 0, 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if i+1 >= len(args) {
		return 0, 0, fmt.Errorf("option '%s' requires an argument", prefix)
	}
	v, err := parsePositiveInt(args[i+1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width: '%s'", args[i+1])
	}
	return v, 2, nil
}

// parsePositiveInt parses a positive integer from a string.
func parsePositiveInt(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid")
	}
	return v, nil
}

// --- File processing ---

// run processes all input files and returns the exit code.
// D3: exit 0 on success, non-zero on error.
func run(cfg fmtConfig, files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	for _, name := range files {
		if err := processFile(name, stdin, bw, cfg); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processFile opens and formats a single file.
func processFile(name string, stdin io.Reader, bw *bufio.Writer, cfg fmtConfig) error {
	r, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if name != "-" {
		defer r.Close()
	}
	return formatInput(r, bw, cfg)
}

// openInput opens a file or returns stdin for "-".
func openInput(name string, stdin io.Reader) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(stdin), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", name, unwrapOSError(err))
	}
	return f, nil
}

// unwrapOSError extracts the underlying error from an *os.PathError.
func unwrapOSError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// --- Input formatting ---

// formatInput dispatches to the appropriate formatting mode.
func formatInput(r io.Reader, bw *bufio.Writer, cfg fmtConfig) error {
	if cfg.split {
		return formatSplitOnly(r, bw, cfg)
	}
	return formatParagraphs(r, bw, cfg)
}

// formatSplitOnly processes input in split-only mode.
// R3.1: splits long lines without joining short ones.
func formatSplitOnly(r io.Reader, bw *bufio.Writer, cfg fmtConfig) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := splitLine(scanner.Text(), bw, cfg); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// splitLine outputs a line, breaking it at word boundaries if too long.
// R3.1: short lines pass through unchanged without -u.
// R3.2: with -u, normalize spacing even for short lines.
func splitLine(line string, bw *bufio.Writer, cfg fmtConfig) error {
	indent := getIndent(line)
	words := strings.Fields(strings.TrimSpace(line))
	if len(words) == 0 {
		return bw.WriteByte('\n')
	}
	if !cfg.uniform && len(line) <= cfg.width {
		return writeLine(bw, line)
	}
	return fillWords(words, indent, indent, bw, cfg)
}

// formatParagraphs reads input, detects paragraphs, and formats each.
// R1.2: blank lines delimit paragraphs.
// R1.3: indentation changes delimit paragraphs.
func formatParagraphs(r io.Reader, bw *bufio.Writer, cfg fmtConfig) error {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if isBlankLine(line) {
			if err := flushParagraph(lines, bw, cfg); err != nil {
				return err
			}
			lines = nil
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
			continue
		}
		if shouldBreakParagraph(lines, line) {
			if err := flushParagraph(lines, bw, cfg); err != nil {
				return err
			}
			lines = nil
		}
		lines = append(lines, line)
	}
	if err := flushParagraph(lines, bw, cfg); err != nil {
		return err
	}
	return scanner.Err()
}

// shouldBreakParagraph returns true if line starts a new paragraph.
// D4: indentation change after the second line signals a new paragraph.
func shouldBreakParagraph(current []string, line string) bool {
	if len(current) < 2 {
		return false
	}
	bodyIndent := getIndent(current[1])
	return getIndent(line) != bodyIndent
}

// flushParagraph formats and writes a collected paragraph.
func flushParagraph(lines []string, bw *bufio.Writer, cfg fmtConfig) error {
	if len(lines) == 0 {
		return nil
	}
	return writeFormatted(lines, bw, cfg)
}

// --- Paragraph formatting ---

// writeFormatted fills a paragraph's words to the goal/width.
// R1.3: preserves first-line and body indentation.
func writeFormatted(lines []string, bw *bufio.Writer, cfg fmtConfig) error {
	firstIndent := getIndent(lines[0])
	bodyIndent := firstIndent
	if len(lines) > 1 {
		bodyIndent = getIndent(lines[1])
	}
	words := extractWords(lines)
	if len(words) == 0 {
		return nil
	}
	return fillWords(words, firstIndent, bodyIndent, bw, cfg)
}

// fillWords fills words into output lines respecting goal and width.
// R1.1: reformats to goal width.
// R2.3: breaks at word boundaries; overlong words get their own line.
// R2.4: two spaces after sentence-ending punctuation.
func fillWords(
	words []string, firstIndent, bodyIndent string,
	bw *bufio.Writer, cfg fmtConfig,
) error {
	indent := firstIndent
	lineLen := len(indent)
	started := false
	var prevWord string
	for _, word := range words {
		if started {
			sp := spacesAfter(prevWord)
			if shouldBreakLine(lineLen, len(word), sp, cfg) {
				if err := bw.WriteByte('\n'); err != nil {
					return err
				}
				indent = bodyIndent
				lineLen = len(indent)
				started = false
			}
		}
		if err := emitWord(bw, word, indent, started, spacesAfter(prevWord)); err != nil {
			return err
		}
		if started {
			lineLen += spacesAfter(prevWord) + len(word)
		} else {
			lineLen += len(word)
			started = true
		}
		prevWord = word
	}
	if started {
		return bw.WriteByte('\n')
	}
	return nil
}

// emitWord writes a word to output, prefixed by indent or spaces.
// R2.4: uses sp spaces between words (1 normally, 2 after sentence end).
func emitWord(bw *bufio.Writer, word, indent string, midLine bool, sp int) error {
	if midLine {
		for j := 0; j < sp; j++ {
			if err := bw.WriteByte(' '); err != nil {
				return err
			}
		}
	} else {
		if _, err := bw.WriteString(indent); err != nil {
			return err
		}
	}
	_, err := bw.WriteString(word)
	return err
}

// shouldBreakLine returns true if adding a word requires a line break.
// R2.3: respects variable space count for sentence-ending words.
func shouldBreakLine(lineLen, wordLen, sp int, cfg fmtConfig) bool {
	newLen := lineLen + sp + wordLen
	if newLen > cfg.width {
		return true
	}
	return lineLen >= cfg.goal && newLen > cfg.goal
}

// --- Sentence detection (R2.4) ---

// isSentenceEnd returns true if a word ends with sentence-ending punctuation.
func isSentenceEnd(word string) bool {
	if len(word) == 0 {
		return false
	}
	last := word[len(word)-1]
	return last == '.' || last == '!' || last == '?'
}

// spacesAfter returns the number of spaces to insert after a word.
// R2.4: two spaces after sentence-ending punctuation, one otherwise.
func spacesAfter(word string) int {
	if isSentenceEnd(word) {
		return 2
	}
	return 1
}

// --- Helpers ---

// extractWords collects all whitespace-separated words from paragraph lines.
func extractWords(lines []string) []string {
	var words []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		words = append(words, strings.Fields(trimmed)...)
	}
	return words
}

// getIndent returns the leading whitespace of a line.
func getIndent(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			return line[:i]
		}
	}
	return line
}

// isBlankLine returns true if line contains only whitespace.
func isBlankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// writeLine writes a string followed by a newline.
func writeLine(bw *bufio.Writer, s string) error {
	if _, err := bw.WriteString(s); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}
