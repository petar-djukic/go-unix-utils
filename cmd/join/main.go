// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	field1     int
	field2     int
	separator  string
	hasSep     bool
	output     string
	unpairFile []int
	onlyUnpair []int
	empty      string
	header     bool
	checkOrder bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	opts, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %s\n", err)
		os.Exit(1)
	}
	code := run(w, opts, files)
	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(code)
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{field1: 1, field2: 1}
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, &opts, args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && !isFileArg(arg) {
			extra, err := parseShortFlag(arg, &opts, args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += 1 + extra
			continue
		}
		files = append(files, arg)
		i++
	}

	return opts, files, nil
}

func isFileArg(arg string) bool {
	return arg == "-"
}

func parseLongFlag(flag string, opts *options, _ []string) (int, error) {
	switch flag {
	case "--header":
		opts.header = true
		return 1, nil
	case "--check-order":
		opts.checkOrder = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlag(arg string, opts *options, remaining []string) (int, error) {
	switch {
	case strings.HasPrefix(arg, "-1"):
		return parseFieldFlag(arg[2:], &opts.field1, remaining)
	case strings.HasPrefix(arg, "-2"):
		return parseFieldFlag(arg[2:], &opts.field2, remaining)
	case strings.HasPrefix(arg, "-j"):
		extra, err := parseFieldFlagBoth(arg[2:], opts, remaining)
		return extra, err
	case strings.HasPrefix(arg, "-t"):
		return parseSepFlag(arg[2:], opts, remaining)
	case strings.HasPrefix(arg, "-o"):
		return parseStringOpt(arg[2:], &opts.output, remaining)
	case strings.HasPrefix(arg, "-a"):
		return parseFileNum(arg[2:], &opts.unpairFile, remaining)
	case strings.HasPrefix(arg, "-v"):
		return parseFileNum(arg[2:], &opts.onlyUnpair, remaining)
	case strings.HasPrefix(arg, "-e"):
		return parseStringOpt(arg[2:], &opts.empty, remaining)
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
}

func parseFieldFlag(val string, dst *int, remaining []string) (int, error) {
	if val == "" {
		if len(remaining) == 0 {
			return 0, fmt.Errorf("option requires an argument")
		}
		n, err := strconv.Atoi(remaining[0])
		if err != nil {
			return 0, fmt.Errorf("invalid field number: '%s'", remaining[0])
		}
		*dst = n
		return 1, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid field number: '%s'", val)
	}
	*dst = n
	return 0, nil
}

func parseFieldFlagBoth(val string, opts *options, remaining []string) (int, error) {
	var field int
	extra, err := parseFieldFlag(val, &field, remaining)
	if err != nil {
		return extra, err
	}
	opts.field1 = field
	opts.field2 = field
	return extra, nil
}

func parseSepFlag(val string, opts *options, remaining []string) (int, error) {
	if val == "" {
		if len(remaining) == 0 {
			return 0, fmt.Errorf("option requires an argument -- 't'")
		}
		opts.separator = remaining[0]
		opts.hasSep = true
		return 1, nil
	}
	opts.separator = val
	opts.hasSep = true
	return 0, nil
}

func parseStringOpt(val string, dst *string, remaining []string) (int, error) {
	if val == "" {
		if len(remaining) == 0 {
			return 0, fmt.Errorf("option requires an argument")
		}
		*dst = remaining[0]
		return 1, nil
	}
	*dst = val
	return 0, nil
}

func parseFileNum(val string, dst *[]int, remaining []string) (int, error) {
	raw := val
	extra := 0
	if raw == "" {
		if len(remaining) == 0 {
			return 0, fmt.Errorf("option requires an argument")
		}
		raw = remaining[0]
		extra = 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || (n != 1 && n != 2) {
		return 0, fmt.Errorf("invalid file number: '%s'", raw)
	}
	*dst = append(*dst, n)
	return extra, nil
}

func run(w *bufio.Writer, opts options, files []string) int {
	if len(files) != 2 {
		fmt.Fprintf(os.Stderr, "join: missing operand\n")
		return 1
	}
	r1, err := openInput(files[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	defer r1.Close()
	r2, err := openInput(files[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "join: %v\n", err)
		return 1
	}
	defer r2.Close()

	joinFiles(w, r1, r2, opts)
	return 0
}

type inputFile struct {
	scanner *bufio.Scanner
	closer  func() error
}

func (f *inputFile) Close() error {
	return f.closer()
}

func openInput(name string) (*inputFile, error) {
	if name == "-" {
		return &inputFile{
			scanner: bufio.NewScanner(os.Stdin),
			closer:  func() error { return nil },
		}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return &inputFile{
		scanner: bufio.NewScanner(f),
		closer:  f.Close,
	}, nil
}

type outputField struct {
	fileNum  int
	fieldNum int
}

type lineReader struct {
	scanner *bufio.Scanner
	line    string
	hasLine bool
}

func newReader(f *inputFile) *lineReader {
	return &lineReader{scanner: f.scanner}
}

func (r *lineReader) next() bool {
	r.hasLine = r.scanner.Scan()
	if r.hasLine {
		r.line = r.scanner.Text()
	}
	return r.hasLine
}

func splitLine(line string, hasSep bool, sep string) []string {
	if hasSep {
		return strings.Split(line, sep)
	}
	return strings.Fields(line)
}

func getField(fields []string, n int) string {
	if n >= 1 && n <= len(fields) {
		return fields[n-1]
	}
	return ""
}

func parseOutputSpec(s string) []outputField {
	if s == "" {
		return nil
	}
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' '
	})
	spec := make([]outputField, len(tokens))
	for i, tok := range tokens {
		if tok == "0" {
			spec[i] = outputField{fileNum: 0}
			continue
		}
		parts := strings.SplitN(tok, ".", 2)
		fn, _ := strconv.Atoi(parts[0])
		fld := 0
		if len(parts) == 2 {
			fld, _ = strconv.Atoi(parts[1])
		}
		spec[i] = outputField{fileNum: fn, fieldNum: fld}
	}
	return spec
}

func joinFiles(w *bufio.Writer, r1, r2 *inputFile, opts options) {
	spec := parseOutputSpec(opts.output)
	lr1 := newReader(r1)
	lr2 := newReader(r2)
	lr1.next()
	lr2.next()
	for lr1.hasLine && lr2.hasLine {
		f1 := splitLine(lr1.line, opts.hasSep, opts.separator)
		f2 := splitLine(lr2.line, opts.hasSep, opts.separator)
		key1 := getField(f1, opts.field1)
		key2 := getField(f2, opts.field2)
		cmp := strings.Compare(key1, key2)
		switch {
		case cmp < 0:
			lr1.next()
		case cmp > 0:
			lr2.next()
		default:
			joinMatch(w, lr1, lr2, key1, spec, opts)
		}
	}
}

func joinMatch(w *bufio.Writer, lr1, lr2 *lineReader, key string, spec []outputField, opts options) {
	group := collectGroup(lr2, opts.field2, key, opts)
	for lr1.hasLine {
		f1 := splitLine(lr1.line, opts.hasSep, opts.separator)
		if getField(f1, opts.field1) != key {
			break
		}
		for _, f2 := range group {
			writePair(w, key, f1, f2, spec, opts)
		}
		lr1.next()
	}
}

func collectGroup(lr *lineReader, fieldNum int, key string, opts options) [][]string {
	var group [][]string
	for lr.hasLine {
		fields := splitLine(lr.line, opts.hasSep, opts.separator)
		if getField(fields, fieldNum) != key {
			break
		}
		group = append(group, fields)
		lr.next()
	}
	return group
}

func writePair(w *bufio.Writer, key string, f1, f2 []string, spec []outputField, opts options) {
	sep := " "
	if opts.hasSep {
		sep = opts.separator
	}
	if len(spec) > 0 {
		fmt.Fprintln(w, formatOutput(key, f1, f2, spec, sep))
	} else {
		fmt.Fprintln(w, defaultOutput(key, f1, opts.field1, f2, opts.field2, sep))
	}
}

func defaultOutput(key string, f1 []string, j1 int, f2 []string, j2 int, sep string) string {
	parts := []string{key}
	for i, f := range f1 {
		if i+1 != j1 {
			parts = append(parts, f)
		}
	}
	for i, f := range f2 {
		if i+1 != j2 {
			parts = append(parts, f)
		}
	}
	return strings.Join(parts, sep)
}

func formatOutput(key string, f1, f2 []string, spec []outputField, sep string) string {
	parts := make([]string, len(spec))
	for i, s := range spec {
		switch s.fileNum {
		case 0:
			parts[i] = key
		case 1:
			parts[i] = getField(f1, s.fieldNum)
		case 2:
			parts[i] = getField(f2, s.fieldNum)
		}
	}
	return strings.Join(parts, sep)
}
