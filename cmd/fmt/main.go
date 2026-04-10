// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fmt: simple text formatter.
// Implements srd070-fmt R1.1, R2.1, R3.1, R4.1.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "fmt"

// defaultWidth is the GNU fmt default line width. R1.1.
const defaultWidth = 75

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the fmt logic and returns the exit code.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	files := parseArgs(args)
	readers, closers, err := openInputs(files, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1
	}
	defer closeAll(closers)
	w := bufio.NewWriter(stdout)
	for _, r := range readers {
		formatInput(r, w, defaultWidth)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", progName, err)
		return 1
	}
	return 0
}

// parseArgs extracts file arguments from the command line.
// R4.1: files from args; stdin when no file or "-".
func parseArgs(args []string) []string {
	var files []string
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		files = append(files, arg)
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	return files
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

// formatInput reads input and formats paragraphs to the given width.
// R1.1: reformat to fit within width. R2.1: blank lines separate paragraphs.
// R3.1: preserve indentation.
func formatInput(r io.Reader, w *bufio.Writer, width int) {
	scanner := bufio.NewScanner(r)
	var para []string
	firstPara := true
	for scanner.Scan() {
		line := scanner.Text()
		if isBlankLine(line) {
			if len(para) > 0 {
				writeParagraph(w, para, width, firstPara)
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
		writeParagraph(w, para, width, firstPara)
	}
}

// isBlankLine returns true if the line contains only whitespace.
func isBlankLine(line string) bool {
	return strings.TrimSpace(line) == ""
}

// writeParagraph formats and outputs a single paragraph.
// R3.1: preserve first-line indentation; fill to second line's indent.
func writeParagraph(w *bufio.Writer, lines []string, width int, first bool) {
	if !first {
		// blank line already written by caller
	}
	firstIndent := leadingWhitespace(lines[0])
	bodyIndent := firstIndent
	if len(lines) > 1 {
		bodyIndent = leadingWhitespace(lines[1])
	}
	words := collectWords(lines)
	writeWrapped(w, words, firstIndent, bodyIndent, width)
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
func writeWrapped(w *bufio.Writer, words []string, firstIndent, bodyIndent string, width int) {
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
		needed := 1 + len(word)
		if col+needed > width {
			fmt.Fprintln(w)
			indent = bodyIndent
			fmt.Fprint(w, indent)
			fmt.Fprint(w, word)
			col = len(indent) + len(word)
		} else {
			fmt.Fprint(w, " ", word)
			col += needed
		}
	}
	fmt.Fprintln(w)
}
