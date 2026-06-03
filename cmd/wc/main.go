// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	printLines bool
	printWords bool
	printBytes bool
	printChars bool
	printMax   bool
	totalMode  string
	files0From string
}

type counts struct {
	lines   int64
	words   int64
	bytes   int64
	chars   int64
	maxLine int64
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, files := parseArgs(os.Args[1:])
	exitCode := run(opts, files, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(opts options, files []string, stdout io.Writer, stderr io.Writer) int {
	if noFlagsSet(opts) {
		opts.printLines = true
		opts.printWords = true
		opts.printBytes = true
	}
	if opts.totalMode == "" {
		opts.totalMode = "auto"
	}

	if opts.files0From != "" {
		extra, err := readFiles0From(opts.files0From)
		if err != nil {
			fmt.Fprintf(stderr, "wc: %s\n", err)
			return 1
		}
		files = append(files, extra...)
	}

	if len(files) == 0 {
		files = []string{""}
	}

	width := 1
	if opts.totalMode != "only" && opts.files0From != "-" {
		width = computeWidth(files, countColumns(opts))
	}
	results, names, hadError := processFiles(files, stderr)
	writeErr := printResults(stdout, opts, results, names, width, len(files))
	if hadError || writeErr {
		return 1
	}
	return 0
}

func noFlagsSet(opts options) bool {
	return !opts.printLines && !opts.printWords &&
		!opts.printBytes && !opts.printChars && !opts.printMax
}

func processFiles(files []string, stderr io.Writer) ([]counts, []string, bool) {
	results := make([]counts, 0, len(files))
	names := make([]string, 0, len(files))
	hadError := false

	for _, f := range files {
		c, err := countFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "wc: %s\n", err)
			hadError = true
			continue
		}
		results = append(results, c)
		if f == "" {
			names = append(names, "")
		} else {
			names = append(names, f)
		}
	}
	return results, names, hadError
}

func countFile(name string) (counts, error) {
	var r io.Reader
	if name == "-" || name == "" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return counts{}, fmt.Errorf("%s: %s", name, err.(*os.PathError).Err)
		}
		defer f.Close()
		r = f
	}
	return countReader(r)
}

func countReader(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReader(r)
	inWord := false
	lineLen := int64(0)

	buf := make([]byte, 32*1024)
	for {
		n, err := br.Read(buf)
		c.bytes += int64(n)
		processChunk(buf[:n], &c, &inWord, &lineLen)
		if err == io.EOF {
			break
		}
		if err != nil {
			return c, err
		}
	}

	if lineLen > c.maxLine {
		c.maxLine = lineLen
	}
	return c, nil
}

func processChunk(data []byte, c *counts, inWord *bool, lineLen *int64) {
	i := 0
	for i < len(data) {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			c.chars++
			processInvalidByte(data[i], c, inWord, lineLen)
			i++
			continue
		}
		c.chars++
		if r == '\n' {
			c.lines++
			handleNewline(c, inWord, lineLen)
		} else if r == '\t' {
			handleTab(inWord, lineLen)
		} else if r == '\r' || r == '\f' {
			*inWord = false
			handleReturn(c, lineLen)
		} else if unicode.IsSpace(r) {
			*inWord = false
			if isPrintable(r) {
				*lineLen++
			}
		} else {
			if !*inWord {
				c.words++
				*inWord = true
			}
			if isPrintable(r) {
				*lineLen++
			}
		}
		i += size
	}
}

func processInvalidByte(b byte, c *counts, inWord *bool, lineLen *int64) {
	switch b {
	case '\n':
		c.lines++
		handleNewline(c, inWord, lineLen)
	case '\t':
		handleTab(inWord, lineLen)
	case '\r', '\f':
		*inWord = false
		handleReturn(c, lineLen)
	case ' ', '\v':
		*inWord = false
	default:
		if !*inWord {
			c.words++
			*inWord = true
		}
	}
}

func handleNewline(c *counts, inWord *bool, lineLen *int64) {
	*inWord = false
	if *lineLen > c.maxLine {
		c.maxLine = *lineLen
	}
	*lineLen = 0
}

func handleReturn(c *counts, lineLen *int64) {
	if *lineLen > c.maxLine {
		c.maxLine = *lineLen
	}
	*lineLen = 0
}

func handleTab(inWord *bool, lineLen *int64) {
	*inWord = false
	*lineLen = (*lineLen/8 + 1) * 8
}

func isPrintable(r rune) bool {
	return r >= 0x20 && r <= 0x7E
}

func printResults(w io.Writer, opts options, results []counts, names []string, width int, numFiles int) bool {
	total := sumCounts(results)
	showTotal := shouldShowTotal(opts.totalMode, numFiles)
	writeErr := false

	if opts.totalMode != "only" {
		for i, c := range results {
			if err := printLine(w, opts, c, names[i], width); err != nil {
				writeErr = true
			}
		}
	}
	if showTotal {
		label := "total"
		if opts.totalMode == "only" {
			label = ""
		}
		if err := printLine(w, opts, total, label, width); err != nil {
			writeErr = true
		}
	}
	return writeErr
}

func sumCounts(results []counts) counts {
	var total counts
	for _, c := range results {
		total.lines += c.lines
		total.words += c.words
		total.bytes += c.bytes
		total.chars += c.chars
		if c.maxLine > total.maxLine {
			total.maxLine = c.maxLine
		}
	}
	return total
}

func shouldShowTotal(mode string, n int) bool {
	switch mode {
	case "always":
		return true
	case "only":
		return true
	case "never":
		return false
	default:
		return n > 1
	}
}

func countColumns(opts options) int {
	n := 0
	if opts.printLines {
		n++
	}
	if opts.printWords {
		n++
	}
	if opts.printChars {
		n++
	}
	if opts.printBytes {
		n++
	}
	if opts.printMax {
		n++
	}
	return n
}

func computeWidth(files []string, numColumns int) int {
	totalSize := int64(0)
	hasRegular := false
	for _, f := range files {
		if f == "-" || f == "" {
			continue
		}
		info, err := os.Stat(f)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		totalSize += info.Size()
		hasRegular = true
	}
	minWidth := 1
	if numColumns > 1 && !hasRegular {
		minWidth = 7
	}
	if hasRegular {
		return max(len(fmt.Sprintf("%d", totalSize)), minWidth)
	}
	return minWidth
}

func printLine(w io.Writer, opts options, c counts, name string, width int) error {
	fields := collectFields(opts, c, width)
	line := strings.Join(fields, " ")
	if name != "" {
		line += " " + name
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

func collectFields(opts options, c counts, width int) []string {
	fields := make([]string, 0, 5)
	if opts.printLines {
		fields = append(fields, fmt.Sprintf("%*d", width, c.lines))
	}
	if opts.printWords {
		fields = append(fields, fmt.Sprintf("%*d", width, c.words))
	}
	if opts.printChars {
		fields = append(fields, fmt.Sprintf("%*d", width, c.chars))
	}
	if opts.printBytes {
		fields = append(fields, fmt.Sprintf("%*d", width, c.bytes))
	}
	if opts.printMax {
		fields = append(fields, fmt.Sprintf("%*d", width, c.maxLine))
	}
	return fields
}

func readFiles0From(path string) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	parts := strings.Split(string(data), "\x00")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts, nil
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if handled, advance := parseLongFlag(arg, args[i:], &opts); handled {
			i += advance
			continue
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			i++
			continue
		}
		i += parseShortFlags(arg[1:], &opts)
	}
	return opts, files
}

func parseLongFlag(arg string, _ []string, opts *options) (bool, int) {
	if !strings.HasPrefix(arg, "--") {
		return false, 0
	}
	switch {
	case arg == "--lines":
		opts.printLines = true
	case arg == "--words":
		opts.printWords = true
	case arg == "--bytes":
		opts.printBytes = true
	case arg == "--chars":
		opts.printChars = true
	case arg == "--max-line-length":
		opts.printMax = true
	case strings.HasPrefix(arg, "--total="):
		opts.totalMode = arg[len("--total="):]
	case strings.HasPrefix(arg, "--files0-from="):
		opts.files0From = arg[len("--files0-from="):]
	default:
		return false, 0
	}
	return true, 1
}

func parseShortFlags(flags string, opts *options) int {
	for _, ch := range flags {
		switch ch {
		case 'l':
			opts.printLines = true
		case 'w':
			opts.printWords = true
		case 'c':
			opts.printBytes = true
		case 'm':
			opts.printChars = true
		case 'L':
			opts.printMax = true
		}
	}
	return 1
}
