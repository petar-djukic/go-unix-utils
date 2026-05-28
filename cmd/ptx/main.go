// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ptx implements srd111-ptx: produce a permuted (KWIC) index of text input.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultWidth   = 72
	defaultGapSize = 3
)

type entry struct {
	before  string
	kwAfter string
	sortKey string
}

type wordSpan struct {
	start, end int
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"-"}
	}

	lines, err := readAllLines(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptx: %s\n", formatErr(err))
		os.Exit(1)
	}

	entries := buildIndex(lines)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].sortKey < entries[j].sortKey
	})

	if err := writeOutput(entries); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "ptx: %s\n", formatErr(err))
		os.Exit(1)
	}
}

func readAllLines(names []string) ([]string, error) {
	var all []string
	for _, name := range names {
		lines, err := readFile(name)
		if err != nil {
			return nil, err
		}
		all = append(all, lines...)
	}
	return all, nil
}

func readFile(name string) ([]string, error) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	sc := bufio.NewScanner(r)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func buildIndex(lines []string) []entry {
	var entries []entry
	for _, line := range lines {
		for _, w := range findWords(line) {
			entries = append(entries, entry{
				before:  line[:w.start],
				kwAfter: line[w.start:],
				sortKey: line[w.start:w.end],
			})
		}
	}
	return entries
}

func findWords(line string) []wordSpan {
	var spans []wordSpan
	i := 0
	for i < len(line) {
		r, size := utf8.DecodeRuneInString(line[i:])
		if !isWordChar(r) {
			i += size
			continue
		}
		start := i
		for i < len(line) {
			r, size = utf8.DecodeRuneInString(line[i:])
			if !isWordChar(r) {
				break
			}
			i += size
		}
		spans = append(spans, wordSpan{start, i})
	}
	return spans
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func writeOutput(entries []entry) error {
	w := bufio.NewWriter(os.Stdout)
	half := defaultWidth / 2
	leftMax := half - defaultGapSize
	rightMax := defaultWidth - half - defaultGapSize
	for _, e := range entries {
		line := formatEntry(e, leftMax, rightMax, half)
		if _, err := w.WriteString(line); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.Flush()
}

func formatEntry(e entry, leftMax, rightMax, half int) string {
	left := e.before
	if len(left) > leftMax {
		left = left[len(left)-leftMax:]
	}
	kwLen := len(e.sortKey)
	right := strings.ToUpper(e.kwAfter[:kwLen]) + e.kwAfter[kwLen:]
	if len(right) > rightMax {
		right = right[:rightMax]
	}
	var buf strings.Builder
	pad := half - defaultGapSize - len(left)
	writeSpaces(&buf, pad)
	buf.WriteString(left)
	writeSpaces(&buf, defaultGapSize)
	buf.WriteString(right)
	return buf.String()
}

func writeSpaces(b *strings.Builder, n int) {
	for range n {
		b.WriteByte(' ')
	}
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("%s: %s", pe.Path, pe.Err)
	}
	return err.Error()
}
