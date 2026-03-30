// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ptx implements GNU ptx: produce a permuted index.
//
// Implements prd111-ptx R1.1, R2.1, R2.2, R2.3, R5.1, R5.2.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
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
	width      int  // R2.1: output line width
	gapSize    int  // R2.1: minimum gap between columns
	ignoreCase bool // R2.2: fold case for sorting
}

// indexEntry holds one permuted index entry.
type indexEntry struct {
	left    string // text before keyword on the input line
	keyword string // the keyword itself
	right   string // text after keyword on the input line
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
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
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
	entries := buildIndex(lines)
	sortEntries(entries, opts.ignoreCase)
	return writeOutput(entries, opts, stdout, stderr)
}

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
	if args[i] == "--ignore-case" {
		opts.ignoreCase = true
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
	n, err := strconv.Atoi(val)
	if err != nil {
		return i, fmt.Errorf("invalid number: %s", val)
	}
	switch name {
	case "--width":
		opts.width = n
	case "--gap-size":
		opts.gapSize = n
	default:
		return i, fmt.Errorf("unrecognized option '%s'", name)
	}
	return i, nil
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
		case 'w':
			return parseShortValue(chars[j+1:], args, i, &opts.width)
		case 'g':
			return parseShortValue(chars[j+1:], args, i, &opts.gapSize)
		default:
			return i, fmt.Errorf("invalid option -- '%c'", chars[j])
		}
	}
	return i, nil
}

// parseShortValue extracts an integer value for a short flag.
// rest is the remaining characters after the flag letter in the same arg.
func parseShortValue(rest string, args []string, i int, target *int) (int, error) {
	val := rest
	if val == "" {
		if i+1 >= len(args) {
			return i, fmt.Errorf("option requires an argument")
		}
		val = args[i+1]
		i++
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return i, fmt.Errorf("invalid number: %s", val)
	}
	*target = n
	return i, nil
}

// readAllInput reads lines from all named files.
func readAllInput(files []string, stdin io.Reader, stderr io.Writer) ([]string, error) {
	var lines []string
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
func readFile(name string, stdin io.Reader) ([]string, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer.Close()
	}
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
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

// buildIndex creates index entries for every word in every line.
// R1.1: each significant word appears as a keyword in context.
func buildIndex(lines []string) []indexEntry {
	var entries []indexEntry
	for _, line := range lines {
		words := extractWords(line)
		for _, w := range words {
			entries = append(entries, indexEntry{
				left:    strings.TrimRight(line[:w.start], " \t"),
				keyword: w.word,
				right:   line[w.end:],
			})
		}
	}
	return entries
}

// extractWords finds all whitespace-delimited words and their positions.
// R2.1 (task): words are separated by whitespace.
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

// formatEntry formats one index entry as a fixed-width output line.
// R2.1 (PRD): output respects width and gap-size settings.
func formatEntry(e indexEntry, opts ptxOptions) string {
	halfWidth := (opts.width - opts.gapSize) / 2
	rightWidth := opts.width - halfWidth - opts.gapSize

	left := e.left
	if len(left) > halfWidth {
		left = left[len(left)-halfWidth:]
	}

	right := e.keyword + e.right
	if len(right) > rightWidth {
		right = right[:rightWidth]
	}

	return fmt.Sprintf("%*s%*s%s", halfWidth, left, opts.gapSize, "", right)
}

// writeOutput writes all formatted index entries to stdout.
func writeOutput(
	entries []indexEntry, opts ptxOptions,
	stdout io.Writer, stderr io.Writer,
) int {
	bw := bufio.NewWriter(stdout)
	for _, e := range entries {
		line := formatEntry(e, opts)
		if _, err := fmt.Fprintln(bw, line); err != nil {
			return handleWriteErr(err, stderr)
		}
	}
	if err := bw.Flush(); err != nil {
		return handleWriteErr(err, stderr)
	}
	return 0
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
