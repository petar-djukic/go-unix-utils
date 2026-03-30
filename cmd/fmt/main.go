// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fmt: a simple text formatter.
// Implements prd070-fmt R1.1, R2.1, R3.1, R4.1.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultWidth = 75
	goalPercent  = 93
)

func main() {
	sys.InstallSIGPIPEHandler()

	files := os.Args[1:]
	rc := run(files, os.Stdout)
	os.Exit(rc)
}

// run reads from files (or stdin) and formats output. Returns exit code.
func run(files []string, w io.Writer) int {
	if len(files) == 0 {
		formatReader(os.Stdin, w)
		return 0
	}
	exitCode := 0
	for _, f := range files {
		if err := formatFile(f, w); err != nil {
			fmt.Fprintf(os.Stderr, "fmt: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// formatFile opens a file (or stdin for "-") and formats it.
func formatFile(name string, w io.Writer) error {
	if name == "-" {
		formatReader(os.Stdin, w)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	formatReader(f, w)
	return nil
}

// formatReader reads all paragraphs from r and writes formatted output.
// R2.1: blank lines separate paragraphs.
// R3.1: indentation changes also separate paragraphs.
func formatReader(r io.Reader, w io.Writer) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	scanner := bufio.NewScanner(r)

	var para []string
	var paraIndent string
	hasPara := false

	for scanner.Scan() {
		line := scanner.Text()
		if isBlank(line) {
			flushParagraph(bw, para, hasPara)
			hasPara = hasPara || len(para) > 0
			para = para[:0]
			bw.WriteByte('\n')
			continue
		}
		indent := extractIndent(line)
		if len(para) > 0 && indent != paraIndent {
			flushParagraph(bw, para, hasPara)
			hasPara = true
			para = para[:0]
		}
		if len(para) == 0 {
			paraIndent = indent
		}
		para = append(para, line)
	}
	flushParagraph(bw, para, hasPara)
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// flushParagraph formats and writes a paragraph.
func flushParagraph(w *bufio.Writer, lines []string, _ bool) {
	if len(lines) == 0 {
		return
	}
	firstIndent := extractIndent(lines[0])
	bodyIndent := firstIndent
	if len(lines) > 1 {
		bodyIndent = extractIndent(lines[1])
	}
	words := extractWords(lines)
	if len(words) == 0 {
		return
	}
	goal := computeGoal(defaultWidth)
	writeFilledLines(w, words, firstIndent, bodyIndent, defaultWidth, goal)
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

// extractWords collects all words from paragraph lines.
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

// writeFilledLines fills words into lines respecting width and goal.
// R1.1: default 75-char width.
func writeFilledLines(w *bufio.Writer, words []string, firstIndent, bodyIndent string, width, goal int) {
	indent := firstIndent
	col := len(indent)
	w.WriteString(indent)
	w.WriteString(words[0])
	col += len(words[0])

	for i := 1; i < len(words); i++ {
		wlen := len(words[i])
		newCol := col + 1 + wlen
		shouldBreak := newCol > width || (col >= goal && newCol > goal)
		if shouldBreak && col > len(indent) {
			w.WriteByte('\n')
			indent = bodyIndent
			w.WriteString(indent)
			col = len(indent) + wlen
			w.WriteString(words[i])
		} else {
			w.WriteByte(' ')
			w.WriteString(words[i])
			col = newCol
		}
	}
	w.WriteByte('\n')
}
