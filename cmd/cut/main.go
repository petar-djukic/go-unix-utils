// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd026-cut.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	byteList      string
	charList      string
	fieldList     string
	delimiter     byte
	outputDelim   string
	complement    bool
	onlyDelimited bool
	hasOutputDel  bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %s\n", err)
		os.Exit(1)
	}
	exitCode := processFiles(w, opts, files)
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func processFiles(w *bufio.Writer, opts options, files []string) int {
	exitCode := 0
	if len(files) == 0 {
		cutLine(os.Stdin, w, opts)
		return exitCode
	}
	for _, name := range files {
		if name == "-" {
			cutLine(os.Stdin, w, opts)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cut: %v\n", err)
			exitCode = 1
			continue
		}
		cutLine(f, w, opts)
		f.Close()
	}
	return exitCode
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{delimiter: '\t'}
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if a == "--complement" {
			opts.complement = true
			continue
		}
		if a == "--output-delimiter" {
			if i+1 >= len(args) {
				return options{}, nil, fmt.Errorf("option '--output-delimiter' requires an argument")
			}
			i++
			opts.outputDelim = args[i]
			opts.hasOutputDel = true
			continue
		}
		if strings.HasPrefix(a, "--output-delimiter=") {
			opts.outputDelim = a[len("--output-delimiter="):]
			opts.hasOutputDel = true
			continue
		}
		advanced, newI, err := parseShortFlag(a, args, i, &opts)
		if err != nil {
			return options{}, nil, err
		}
		i = newI
		if advanced {
			continue
		}
		files = append(files, a)
	}
	return opts, files, validateOpts(opts)
}

func parseShortFlag(a string, args []string, i int, opts *options) (bool, int, error) {
	switch {
	case a == "-b" || strings.HasPrefix(a, "-b"):
		val, idx, err := flagValue(a, "-b", args, i)
		if err != nil {
			return false, i, err
		}
		opts.byteList = val
		return true, idx, nil
	case a == "-c" || strings.HasPrefix(a, "-c"):
		val, idx, err := flagValue(a, "-c", args, i)
		if err != nil {
			return false, i, err
		}
		opts.charList = val
		return true, idx, nil
	case a == "-f" || strings.HasPrefix(a, "-f"):
		val, idx, err := flagValue(a, "-f", args, i)
		if err != nil {
			return false, i, err
		}
		opts.fieldList = val
		return true, idx, nil
	case a == "-d" || strings.HasPrefix(a, "-d"):
		val, idx, err := flagValue(a, "-d", args, i)
		if err != nil {
			return false, i, err
		}
		if len(val) != 1 {
			return false, i, fmt.Errorf("the delimiter must be a single character")
		}
		opts.delimiter = val[0]
		return true, idx, nil
	case a == "-s":
		opts.onlyDelimited = true
		return true, i, nil
	}
	if strings.HasPrefix(a, "-") && a != "-" {
		return false, i, fmt.Errorf("invalid option -- '%s'", a[1:])
	}
	return false, i, nil
}

func flagValue(a, flag string, args []string, i int) (string, int, error) {
	if a == flag {
		if i+1 >= len(args) {
			return "", i, fmt.Errorf("option requires an argument -- '%s'", flag[1:])
		}
		return args[i+1], i + 1, nil
	}
	return a[len(flag):], i, nil
}

func validateOpts(opts options) error {
	count := 0
	if opts.byteList != "" {
		count++
	}
	if opts.charList != "" {
		count++
	}
	if opts.fieldList != "" {
		count++
	}
	if count == 0 {
		return fmt.Errorf("you must specify a list of bytes, characters, or fields")
	}
	if count > 1 {
		return fmt.Errorf("only one type of list may be specified")
	}
	return nil
}

func cutLine(r io.Reader, w *bufio.Writer, opts options) {
	switch {
	case opts.byteList != "":
		cutBytes(r, w, opts)
	case opts.charList != "":
		cutChars(r, w, opts)
	default:
		cutFields(r, w, opts)
	}
}

func cutBytes(r io.Reader, w *bufio.Writer, opts options) {
	ranges, err := parseRangeList(opts.byteList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %s\n", err)
		return
	}
	if opts.complement {
		ranges = complementRanges(ranges)
	}
	processLines(r, w, ranges)
}

func cutChars(r io.Reader, w *bufio.Writer, opts options) {
	ranges, err := parseRangeList(opts.charList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %s\n", err)
		return
	}
	if opts.complement {
		ranges = complementRanges(ranges)
	}
	processLines(r, w, ranges)
}

func processLines(r io.Reader, w *bufio.Writer, ranges [][2]int) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if len(line) > 0 {
			writeSelectedBytes(w, line, ranges)
		}
		if err == nil || len(line) > 0 {
			w.WriteByte('\n')
		}
		if err != nil {
			break
		}
	}
}

func writeSelectedBytes(w *bufio.Writer, line []byte, ranges [][2]int) {
	lineLen := len(line)
	for _, rng := range ranges {
		lo := rng[0] - 1
		hi := rng[1]
		if lo >= lineLen {
			continue
		}
		if hi > lineLen {
			hi = lineLen
		}
		w.Write(line[lo:hi])
	}
}

func parseRangeList(list string) ([][2]int, error) {
	parts := strings.Split(list, ",")
	var ranges [][2]int
	for _, p := range parts {
		r, err := parseRange(p)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return mergeRanges(ranges), nil
}

func parseRange(s string) ([2]int, error) {
	if s == "" {
		return [2]int{}, fmt.Errorf("invalid range")
	}
	idx := strings.Index(s, "-")
	if idx < 0 {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return [2]int{}, fmt.Errorf("invalid byte/character position '%s'", s)
		}
		return [2]int{n, n}, nil
	}
	return parseHyphenRange(s, idx)
}

func parseHyphenRange(s string, idx int) ([2]int, error) {
	if idx == 0 {
		hi, err := strconv.Atoi(s[1:])
		if err != nil || hi <= 0 {
			return [2]int{}, fmt.Errorf("invalid range '%s'", s)
		}
		return [2]int{1, hi}, nil
	}
	lo, err := strconv.Atoi(s[:idx])
	if err != nil || lo <= 0 {
		return [2]int{}, fmt.Errorf("invalid range '%s'", s)
	}
	if idx == len(s)-1 {
		return [2]int{lo, math.MaxInt}, nil
	}
	hi, err := strconv.Atoi(s[idx+1:])
	if err != nil || hi <= 0 {
		return [2]int{}, fmt.Errorf("invalid range '%s'", s)
	}
	if lo > hi {
		return [2]int{}, fmt.Errorf("invalid decreasing range")
	}
	return [2]int{lo, hi}, nil
}

func mergeRanges(ranges [][2]int) [][2]int {
	if len(ranges) == 0 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i][0] < ranges[j][0]
	})
	merged := [][2]int{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r[0] <= last[1]+1 {
			if r[1] > last[1] {
				last[1] = r[1]
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

func complementRanges(ranges [][2]int) [][2]int {
	var result [][2]int
	prev := 1
	for _, r := range ranges {
		if r[0] > prev {
			result = append(result, [2]int{prev, r[0] - 1})
		}
		prev = r[1] + 1
	}
	if prev > 0 && prev < math.MaxInt {
		result = append(result, [2]int{prev, math.MaxInt})
	}
	return result
}

func cutFields(r io.Reader, w *bufio.Writer, opts options) {
	ranges, err := parseRangeList(opts.fieldList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cut: %s\n", err)
		return
	}
	if opts.complement {
		ranges = complementRanges(ranges)
	}
	outDelim := string(opts.delimiter)
	if opts.hasOutputDel {
		outDelim = opts.outputDelim
	}
	br := bufio.NewReader(r)
	for {
		line, readErr := br.ReadString('\n')
		hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
		if hasNewline {
			line = line[:len(line)-1]
		}
		hasDelim := containsByte(line, opts.delimiter)
		if !hasDelim && opts.onlyDelimited {
			if readErr != nil {
				break
			}
			continue
		}
		if hasDelim {
			writeSelectedFields(w, line, opts.delimiter, ranges, outDelim)
		} else if len(line) > 0 {
			w.WriteString(line)
		}
		if readErr == nil || len(line) > 0 {
			w.WriteByte('\n')
		}
		if readErr != nil {
			break
		}
	}
}

func writeSelectedFields(w *bufio.Writer, line string, delim byte, ranges [][2]int, outDelim string) {
	fields := splitFields(line, delim)
	selected := selectFields(fields, ranges)
	w.WriteString(strings.Join(selected, outDelim))
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}

func splitFields(line string, delim byte) []string {
	var fields []string
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] == delim {
			fields = append(fields, line[start:i])
			start = i + 1
		}
	}
	fields = append(fields, line[start:])
	return fields
}

func selectFields(fields []string, ranges [][2]int) []string {
	var result []string
	n := len(fields)
	for _, rng := range ranges {
		lo := rng[0]
		hi := rng[1]
		if lo > n {
			continue
		}
		if hi > n {
			hi = n
		}
		for i := lo; i <= hi; i++ {
			result = append(result, fields[i-1])
		}
	}
	return result
}
