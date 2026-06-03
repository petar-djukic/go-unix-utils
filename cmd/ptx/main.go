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
	"regexp"
	"sort"
	"strconv"
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

type options struct {
	width      int
	gapSize    int
	ignoreCase bool
	autoRef    bool
	references bool
	wordRegexp *regexp.Regexp
	files      []string
}

type entry struct {
	before  string
	kwAfter string
	sortKey string
	ref     string
}

type wordSpan struct {
	start, end int
}

type inputLine struct {
	text    string
	file    string
	lineNum int
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptx: %s\n", err)
		os.Exit(1)
	}

	lines, err := readAllLines(opts.files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptx: %s\n", formatErr(err))
		os.Exit(1)
	}

	entries := buildIndex(lines, &opts)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].sortKey < entries[j].sortKey
	})

	if err := writeOutput(entries, opts.width, opts.gapSize); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "ptx: %s\n", formatErr(err))
		os.Exit(1)
	}
}

func parseArgs(args []string) (options, error) {
	opts := options{
		width:   defaultWidth,
		gapSize: defaultGapSize,
	}
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if strings.HasPrefix(a, "--") {
			n, err := parseLongFlag(a, args[i+1:], &opts)
			if err != nil {
				return opts, err
			}
			i += 1 + n
			continue
		}
		if a != "-" && strings.HasPrefix(a, "-") {
			n, err := parseShortFlags(a[1:], args[i+1:], &opts)
			if err != nil {
				return opts, err
			}
			i += 1 + n
			continue
		}
		break
	}
	opts.files = args[i:]
	if len(opts.files) == 0 {
		opts.files = []string{"-"}
	}
	return opts, nil
}

func parseLongFlag(flag string, rest []string, opts *options) (int, error) {
	if flag == "--ignore-case" {
		opts.ignoreCase = true
		return 0, nil
	}
	if flag == "--auto-reference" {
		opts.autoRef = true
		return 0, nil
	}
	if flag == "--references" {
		opts.references = true
		return 0, nil
	}
	if strings.HasPrefix(flag, "--width=") {
		return 0, parseIntVal(flag[len("--width="):], &opts.width, "width")
	}
	if flag == "--width" {
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		return 1, parseIntVal(rest[0], &opts.width, "width")
	}
	if strings.HasPrefix(flag, "--gap-size=") {
		return 0, parseIntVal(flag[len("--gap-size="):], &opts.gapSize, "gap-size")
	}
	if flag == "--gap-size" {
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		return 1, parseIntVal(rest[0], &opts.gapSize, "gap-size")
	}
	if strings.HasPrefix(flag, "--word-regexp=") {
		return 0, parseRegexpVal(flag[len("--word-regexp="):], &opts.wordRegexp)
	}
	if flag == "--word-regexp" {
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '%s' requires an argument", flag)
		}
		return 1, parseRegexpVal(rest[0], &opts.wordRegexp)
	}
	return 0, fmt.Errorf("unrecognized option '%s'", flag)
}

func parseShortFlags(flags string, rest []string, opts *options) (int, error) {
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case 'f':
			opts.ignoreCase = true
		case 'A':
			opts.autoRef = true
		case 'r':
			opts.references = true
		case 'w':
			return extractAndSetInt(flags[i+1:], rest, &opts.width, 'w')
		case 'g':
			return extractAndSetInt(flags[i+1:], rest, &opts.gapSize, 'g')
		case 'W':
			return extractAndSetRegexp(flags[i+1:], rest, &opts.wordRegexp)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[i])
		}
	}
	return 0, nil
}

func extractAndSetInt(tail string, rest []string, dst *int, flag byte) (int, error) {
	val, consumed, err := extractFlagValue(tail, rest, flag)
	if err != nil {
		return 0, err
	}
	if err := parseIntVal(val, dst, string(flag)); err != nil {
		return 0, err
	}
	return consumed, nil
}

func extractFlagValue(tail string, rest []string, flag byte) (string, int, error) {
	if tail != "" {
		return tail, 0, nil
	}
	if len(rest) > 0 {
		return rest[0], 1, nil
	}
	return "", 0, fmt.Errorf("option requires an argument -- '%c'", flag)
}

func extractAndSetRegexp(tail string, rest []string, dst **regexp.Regexp) (int, error) {
	val, consumed, err := extractFlagValue(tail, rest, 'W')
	if err != nil {
		return 0, err
	}
	if err := parseRegexpVal(val, dst); err != nil {
		return 0, err
	}
	return consumed, nil
}

func parseRegexpVal(s string, dst **regexp.Regexp) error {
	re, err := regexp.Compile(s)
	if err != nil {
		return fmt.Errorf("invalid word regexp '%s': %s", s, err)
	}
	*dst = re
	return nil
}

func parseIntVal(s string, dst *int, name string) error {
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid %s argument '%s'", name, s)
	}
	*dst = v
	return nil
}

func readAllLines(names []string) ([]inputLine, error) {
	var all []inputLine
	for _, name := range names {
		lines, err := readFile(name)
		if err != nil {
			return nil, err
		}
		for i, text := range lines {
			all = append(all, inputLine{text: text, file: name, lineNum: i + 1})
		}
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

func buildIndex(lines []inputLine, opts *options) []entry {
	var entries []entry
	for _, il := range lines {
		text := il.text
		var ref string
		if opts.references {
			ref, text = splitReference(text)
		}
		if opts.autoRef {
			ref = formatAutoRef(il.file, il.lineNum)
		}
		for _, w := range findWords(text, opts.wordRegexp) {
			sk := text[w.start:w.end]
			if opts.ignoreCase {
				sk = strings.ToUpper(sk)
			}
			entries = append(entries, entry{
				before:  text[:w.start],
				kwAfter: text[w.start:],
				sortKey: sk,
				ref:     ref,
			})
		}
	}
	return entries
}

func findWords(line string, wordRe *regexp.Regexp) []wordSpan {
	if wordRe != nil {
		return findWordsRegexp(line, wordRe)
	}
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

func findWordsRegexp(line string, re *regexp.Regexp) []wordSpan {
	locs := re.FindAllStringIndex(line, -1)
	spans := make([]wordSpan, len(locs))
	for i, loc := range locs {
		spans[i] = wordSpan{loc[0], loc[1]}
	}
	return spans
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r)
}

func writeOutput(entries []entry, width, gapSize int) error {
	w := bufio.NewWriter(os.Stdout)
	refMax := maxRefWidth(entries)
	kwicWidth := width
	if refMax > 0 {
		kwicWidth = width - refMax - gapSize
	}
	half := kwicWidth / 2
	rightMax := kwicWidth - half - gapSize
	for _, e := range entries {
		line := formatEntry(e, half, gapSize, rightMax, refMax)
		if _, err := w.WriteString(line); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.Flush()
}

func formatEntry(e entry, half, gapSize, rightMax, refMax int) string {
	left := strings.TrimRight(e.before, " ")
	leftMax := max(half-gapSize, 0)
	if len(left) > leftMax {
		left = left[len(left)-leftMax:]
	}
	right := e.kwAfter
	if rightMax > 0 && len(right) > rightMax {
		right = right[:rightMax]
	}
	var buf strings.Builder
	if refMax > 0 {
		buf.WriteString(e.ref)
		writeSpaces(&buf, refMax-len(e.ref)+gapSize)
	}
	writeSpaces(&buf, half-len(left))
	buf.WriteString(left)
	writeSpaces(&buf, gapSize)
	buf.WriteString(right)
	return buf.String()
}

func writeSpaces(b *strings.Builder, n int) {
	for range n {
		b.WriteByte(' ')
	}
}

func splitReference(line string) (string, string) {
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return line, ""
	}
	ref := line[:i]
	rest := line[i:]
	j := 0
	for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
		j++
	}
	return ref, rest[j:]
}

func formatAutoRef(file string, lineNum int) string {
	if file == "-" {
		file = ""
	}
	return file + ":" + strconv.Itoa(lineNum)
}

func maxRefWidth(entries []entry) int {
	m := 0
	for _, e := range entries {
		if len(e.ref) > m {
			m = len(e.ref)
		}
	}
	return m
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("%s: %s", pe.Path, pe.Err)
	}
	return err.Error()
}
