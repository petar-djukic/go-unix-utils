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

func joinFiles(_ *bufio.Writer, _ *inputFile, _ *inputFile, _ options) {
}
