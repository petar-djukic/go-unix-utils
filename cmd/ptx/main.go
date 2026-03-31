// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ptx implements GNU ptx: produce a permuted index.
//
// Implements prd111-ptx R1.1, R2.1, R2.2, R2.3, R3.1, R4.1, R4.2, R5.1, R5.2.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultWidth   = 72
	defaultGapSize = 3
)

// ptxOptions holds parsed flag state.
type ptxOptions struct {
	width          int    // R2.1: output line width
	gapSize        int    // R2.1: minimum gap between columns
	ignoreCase     bool   // R2.2: fold case for sorting
	wordRegexp     string // R3.1: regexp defining words
	sentenceRegexp string // -S: sentence-end regex
	autoRef        bool   // R4.1: automatic references (filename:linenum)
	references     bool   // R4.2: first field is reference
}

// inputLine holds a line of input with source metadata.
type inputLine struct {
	text     string
	filename string
	lineNum  int
}

// indexEntry holds one permuted index entry.
type indexEntry struct {
	left    string
	keyword string
	right   string
	ref     string
}

// wordPos records a word and its byte offsets in a line.
type wordPos struct {
	word  string
	start int
	end   int
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags, reads input, builds and outputs the permuted index.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "ptx: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	lines, err := readAllInput(files, stdin, stderr)
	if err != nil {
		return 1
	}
	wordRE, err := compileWordRegexp(opts)
	if err != nil {
		fmt.Fprintf(stderr, "ptx: %v\n", err)
		return 1
	}
	if err := validateSentenceRegexp(opts.sentenceRegexp); err != nil {
		fmt.Fprintf(stderr, "ptx: %v\n", err)
		return 1
	}
	entries := buildIndex(lines, opts, wordRE)
	sortEntries(entries, opts.ignoreCase)
	return writeOutput(entries, opts, stdout, stderr)
}

// compileWordRegexp compiles the -W pattern, with case-insensitive flag if -f.
func compileWordRegexp(opts ptxOptions) (*regexp.Regexp, error) {
	if opts.wordRegexp == "" {
		return nil, nil
	}
	pattern := opts.wordRegexp
	if opts.ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regexp: %w", err)
	}
	return re, nil
}

// validateSentenceRegexp validates the -S pattern if provided.
func validateSentenceRegexp(pattern string) error {
	if pattern == "" {
		return nil
	}
	_, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regexp: %w", err)
	}
	return nil
}

// --- Flag Parsing ---

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (ptxOptions, []string, error) {
	opts := ptxOptions{width: defaultWidth, gapSize: defaultGapSize}
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		var next int
		var err error
		if strings.HasPrefix(arg, "--") {
			next, err = parseLongFlag(&opts, args, i)
		} else {
			next, err = parseShortFlags(&opts, args, i)
		}
		if err != nil {
			return opts, nil, err
		}
		i = next
	}
	return opts, files, nil
}

// parseLongFlag handles a single --name or --name=value argument.
func parseLongFlag(opts *ptxOptions, args []string, i int) (int, error) {
	switch args[i] {
	case "--ignore-case":
		opts.ignoreCase = true
		return i, nil
	case "--auto-reference":
		opts.autoRef = true
		return i, nil
	case "--references":
		opts.references = true
		return i, nil
	}
	name, val, hasVal := splitLongFlag(args[i])
	if !hasVal {
		if i+1 >= len(args) {
			return i, fmt.Errorf("option '%s' requires an argument", name)
		}
		val = args[i+1]
		i++
	}
	return applyLongValue(opts, name, val, i)
}

// applyLongValue sets the option for a long flag that takes a value.
func applyLongValue(opts *ptxOptions, name, val string, i int) (int, error) {
	switch name {
	case "--width":
		return i, parseInt(val, &opts.width)
	case "--gap-size":
		return i, parseInt(val, &opts.gapSize)
	case "--word-regexp":
		opts.wordRegexp = val
		return i, nil
	case "--sentence-regexp":
		opts.sentenceRegexp = val
		return i, nil
	default:
		return i, fmt.Errorf("unrecognized option '%s'", name)
	}
}

// splitLongFlag splits --name=value into (name, value, true).
// Returns (--name, "", false) if no = is present.
func splitLongFlag(arg string) (string, string, bool) {
	name, val, ok := strings.Cut(arg, "=")
	return name, val, ok
}

// parseShortFlags handles short flag clusters like -fw72.
func parseShortFlags(opts *ptxOptions, args []string, i int) (int, error) {
	chars := args[i][1:]
	j := 0
	for j < len(chars) {
		switch chars[j] {
		case 'f':
			opts.ignoreCase = true
			j++
		case 'A':
			opts.autoRef = true
			j++
		case 'r':
			opts.references = true
			j++
		case 'w':
			return parseShortInt(chars[j+1:], args, i, &opts.width)
		case 'g':
			return parseShortInt(chars[j+1:], args, i, &opts.gapSize)
		case 'W':
			return parseShortString(chars[j+1:], args, i, &opts.wordRegexp)
		case 'S':
			return parseShortString(chars[j+1:], args, i, &opts.sentenceRegexp)
		default:
			return i, fmt.Errorf("invalid option -- '%c'", chars[j])
		}
	}
	return i, nil
}

// parseShortInt extracts an integer value for a short flag.
func parseShortInt(rest string, args []string, i int, target *int) (int, error) {
	val := rest
	if val == "" {
		if i+1 >= len(args) {
			return i, fmt.Errorf("option requires an argument")
		}
		val = args[i+1]
		i++
	}
	return i, parseInt(val, target)
}

// parseShortString extracts a string value for a short flag.
func parseShortString(rest string, args []string, i int, target *string) (int, error) {
	if rest != "" {
		*target = rest
		return i, nil
	}
	if i+1 >= len(args) {
		return i, fmt.Errorf("option requires an argument")
	}
	*target = args[i+1]
	return i + 1, nil
}

// parseInt parses a string as an integer and stores it in target.
func parseInt(val string, target *int) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("invalid number: %s", val)
	}
	*target = n
	return nil
}

// --- Input Reading ---

// readAllInput reads lines from all named files.
func readAllInput(files []string, stdin io.Reader, stderr io.Writer) ([]inputLine, error) {
	var lines []inputLine
	for _, name := range files {
		fl, err := readFile(name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "ptx: %v\n", err)
			return nil, err
		}
		lines = append(lines, fl...)
	}
	return lines, nil
}

// readFile reads all lines from a single file or stdin.
func readFile(name string, stdin io.Reader) ([]inputLine, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer.Close()
	}
	displayName := name
	if name == "-" {
		displayName = ""
	}
	return scanLines(r, displayName)
}

// scanLines reads lines from r, tagging each with filename and line number.
func scanLines(r io.Reader, filename string) ([]inputLine, error) {
	var lines []inputLine
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		lines = append(lines, inputLine{
			text:     scanner.Text(),
			filename: filename,
			lineNum:  lineNum,
		})
	}
	return lines, scanner.Err()
}

// openInput returns a reader for the named file, or stdin if name is "-".
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// --- Index Building ---

// buildIndex creates KWIC entries. With -r, lines stay separate;
// otherwise all input is concatenated into one text.
func buildIndex(lines []inputLine, opts ptxOptions, wordRE *regexp.Regexp) []indexEntry {
	if opts.references {
		return buildIndexPerLine(lines, opts, wordRE)
	}
	return buildIndexConcat(lines, opts, wordRE)
}

// buildIndexPerLine builds entries per line (used with -r).
func buildIndexPerLine(lines []inputLine, _ ptxOptions, wordRE *regexp.Regexp) []indexEntry {
	var entries []indexEntry
	for _, line := range lines {
		text, ref := extractReference(line.text)
		words := extractWordsFrom(text, wordRE)
		for _, w := range words {
			entries = append(entries, indexEntry{
				left:    strings.TrimRight(text[:w.start], " \t"),
				keyword: w.word,
				right:   text[w.end:],
				ref:     ref,
			})
		}
	}
	return entries
}

// buildIndexConcat concatenates all input and builds entries.
func buildIndexConcat(lines []inputLine, opts ptxOptions, wordRE *regexp.Regexp) []indexEntry {
	texts, refs := prepareInput(lines, opts)
	fullText := strings.Join(texts, " ")
	offsets := buildOffsets(texts)
	words := extractWordsFrom(fullText, wordRE)
	return makeEntries(fullText, words, offsets, refs)
}

// prepareInput extracts indexable text and reference for each line.
func prepareInput(lines []inputLine, opts ptxOptions) ([]string, []string) {
	texts := make([]string, len(lines))
	refs := make([]string, len(lines))
	for i, line := range lines {
		texts[i] = line.text
		if opts.autoRef {
			refs[i] = formatAutoRef(line.filename, line.lineNum)
		}
	}
	return texts, refs
}

// buildOffsets computes the starting byte offset of each line in joined text.
func buildOffsets(texts []string) []int {
	offsets := make([]int, len(texts))
	pos := 0
	for i, t := range texts {
		offsets[i] = pos
		pos += len(t) + 1 // +1 for joining space
	}
	return offsets
}

// makeEntries creates index entries from words in the full text.
func makeEntries(fullText string, words []wordPos, offsets []int, refs []string) []indexEntry {
	entries := make([]indexEntry, 0, len(words))
	for _, w := range words {
		entries = append(entries, indexEntry{
			left:    strings.TrimRight(fullText[:w.start], " \t"),
			keyword: w.word,
			right:   fullText[w.end:],
			ref:     findRef(w.start, offsets, refs),
		})
	}
	return entries
}

// findRef determines the reference for a word at the given byte offset.
func findRef(offset int, offsets []int, refs []string) string {
	lineIdx := 0
	for i := len(offsets) - 1; i >= 0; i-- {
		if offset >= offsets[i] {
			lineIdx = i
			break
		}
	}
	if lineIdx < len(refs) {
		return refs[lineIdx]
	}
	return ""
}

// extractReference splits the first field as a reference (R4.2).
func extractReference(text string) (string, string) {
	idx := strings.IndexAny(text, " \t")
	if idx < 0 {
		return "", text
	}
	ref := text[:idx]
	rest := strings.TrimLeft(text[idx:], " \t")
	return rest, ref
}

// formatAutoRef produces a :linenum: or filename:linenum: reference (R4.1).
func formatAutoRef(filename string, lineNum int) string {
	if filename == "" {
		return fmt.Sprintf(":%d:", lineNum)
	}
	return fmt.Sprintf("%s:%d:", filename, lineNum)
}

// --- Word Extraction ---

// extractWordsFrom dispatches to regexp or whitespace word extraction.
func extractWordsFrom(text string, wordRE *regexp.Regexp) []wordPos {
	if wordRE != nil {
		return extractWordsRegexp(text, wordRE)
	}
	return extractWords(text)
}

// extractWordsRegexp finds all words matching the compiled regexp (R3.1).
func extractWordsRegexp(text string, re *regexp.Regexp) []wordPos {
	matches := re.FindAllStringIndex(text, -1)
	words := make([]wordPos, 0, len(matches))
	for _, m := range matches {
		words = append(words, wordPos{
			word:  text[m[0]:m[1]],
			start: m[0],
			end:   m[1],
		})
	}
	return words
}

// extractWords finds all whitespace-delimited words and their positions.
func extractWords(line string) []wordPos {
	var words []wordPos
	i := 0
	for i < len(line) {
		if line[i] == ' ' || line[i] == '\t' {
			i++
			continue
		}
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		words = append(words, wordPos{word: line[start:i], start: start, end: i})
	}
	return words
}

// --- Sorting ---

// sortEntries sorts index entries alphabetically by keyword.
// R2.2: when ignoreCase is true, comparison folds to uppercase.
func sortEntries(entries []indexEntry, ignoreCase bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].keyword, entries[j].keyword
		if ignoreCase {
			a, b = strings.ToUpper(a), strings.ToUpper(b)
		}
		return a < b
	})
}

// --- Output ---

// writeOutput writes all formatted index entries to stdout.
func writeOutput(entries []indexEntry, opts ptxOptions, stdout, stderr io.Writer) int {
	maxRef := maxRefWidth(entries)
	bw := bufio.NewWriter(stdout)
	for _, e := range entries {
		line := formatEntry(e, opts, maxRef)
		if _, err := fmt.Fprintln(bw, line); err != nil {
			return handleWriteErr(err, stderr)
		}
	}
	if err := bw.Flush(); err != nil {
		return handleWriteErr(err, stderr)
	}
	return 0
}

// maxRefWidth returns the length of the longest reference string.
func maxRefWidth(entries []indexEntry) int {
	m := 0
	for _, e := range entries {
		if len(e.ref) > m {
			m = len(e.ref)
		}
	}
	return m
}

// formatEntry formats one index entry as a fixed-width output line.
func formatEntry(e indexEntry, opts ptxOptions, maxRef int) string {
	if maxRef > 0 {
		return formatEntryWithRef(e, opts, maxRef)
	}
	return formatEntryPlain(e, opts)
}

// formatEntryPlain formats an entry without references.
func formatEntryPlain(e indexEntry, opts ptxOptions) string {
	halfWidth := opts.width / 2
	rightWidth := opts.width - halfWidth - opts.gapSize
	left := truncateLeft(e.left, halfWidth)
	right := truncateRight(e.keyword+e.right, rightWidth)
	return fmt.Sprintf("%*s%*s%s", halfWidth, left, opts.gapSize, "", right)
}

// formatEntryWithRef formats an entry with a left-justified reference.
func formatEntryWithRef(e indexEntry, opts ptxOptions, maxRef int) string {
	halfWidth := opts.width / 2
	if opts.autoRef {
		halfWidth = (opts.width - 2) / 2
	}
	remaining := halfWidth - maxRef
	rightWidth := opts.width - halfWidth - opts.gapSize
	left := truncateLeft(e.left, remaining)
	right := truncateRight(e.keyword+e.right, rightWidth)
	return fmt.Sprintf("%-*s%*s%*s%s", maxRef, e.ref, remaining, left, opts.gapSize, "", right)
}

// truncateLeft keeps the rightmost maxLen characters of s.
func truncateLeft(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) > maxLen {
		return s[len(s)-maxLen:]
	}
	return s
}

// truncateRight keeps the leftmost maxLen characters of s.
func truncateRight(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// handleWriteErr returns 0 for broken pipe, 1 for other write errors.
func handleWriteErr(err error, stderr io.Writer) int {
	if isBrokenPipe(err) {
		return 0
	}
	fmt.Fprintf(stderr, "ptx: write error: %v\n", err)
	return 1
}

// isBrokenPipe reports whether err is caused by writing to a broken pipe.
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
