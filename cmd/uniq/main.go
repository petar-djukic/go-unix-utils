// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd028-uniq.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	count       bool
	repeated    bool
	allRepeated bool
	unique      bool
	ignoreCase  bool
	skipFields  int
	skipChars   int
	checkChars  int
}

func main() {
	sys.InstallSIGPIPEHandler()
	w := bufio.NewWriter(os.Stdout)
	opts, inputFile, outputFile, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		os.Exit(1)
	}
	exitCode := run(w, opts, inputFile, outputFile)
	if err := w.Flush(); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func parseArgs(args []string) (options, string, string, error) {
	opts := options{checkChars: -1}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "-" || len(a) == 0 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}
		var err error
		i, err = parseFlags(&opts, args, i)
		if err != nil {
			return options{}, "", "", err
		}
	}
	inputFile, outputFile := "", ""
	if len(positional) > 0 {
		inputFile = positional[0]
	}
	if len(positional) > 1 {
		outputFile = positional[1]
	}
	if len(positional) > 2 {
		return options{}, "", "", fmt.Errorf("extra operand '%s'", positional[2])
	}
	return opts, inputFile, outputFile, nil
}

func parseFlags(opts *options, args []string, i int) (int, error) {
	flagStr := args[i][1:]
	for j := 0; j < len(flagStr); j++ {
		ch := rune(flagStr[j])
		switch ch {
		case 'f', 's', 'w':
			rest := flagStr[j+1:]
			if len(rest) == 0 {
				i++
				if i >= len(args) {
					return i, fmt.Errorf("option requires an argument -- '%c'", ch)
				}
				rest = args[i]
			}
			n, err := strconv.Atoi(rest)
			if err != nil {
				return i, fmt.Errorf("invalid number of %s: '%s'", flagDescription(ch), rest)
			}
			setNumericOpt(opts, ch, n)
			return i, nil
		default:
			if err := parseFlag(opts, ch); err != nil {
				return i, err
			}
		}
	}
	return i, nil
}

func flagDescription(ch rune) string {
	switch ch {
	case 'f':
		return "fields to skip"
	case 's':
		return "bytes to skip"
	default:
		return "bytes to compare"
	}
}

func setNumericOpt(opts *options, ch rune, n int) {
	switch ch {
	case 'f':
		opts.skipFields = n
	case 's':
		opts.skipChars = n
	case 'w':
		opts.checkChars = n
	}
}

func parseFlag(opts *options, ch rune) error {
	switch ch {
	case 'c':
		opts.count = true
	case 'd':
		opts.repeated = true
	case 'D':
		opts.allRepeated = true
	case 'u':
		opts.unique = true
	case 'i':
		opts.ignoreCase = true
	default:
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

func run(w *bufio.Writer, opts options, inputFile, outputFile string) int {
	r, closer, err := openInput(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		return 1
	}
	if closer != nil {
		defer closer.Close()
	}
	out, outCloser, err := openOutput(w, outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "uniq: %s\n", err)
		return 1
	}
	if outCloser != nil {
		defer outCloser.Close()
	}
	return deduplicate(r, out, opts)
}

func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "" || name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func openOutput(w *bufio.Writer, name string) (*bufio.Writer, io.Closer, error) {
	if name == "" {
		return w, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, err
	}
	return bufio.NewWriter(f), f, nil
}

func comparisonKey(line string, opts options) string {
	s := line
	for range opts.skipFields {
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
			s = s[1:]
		}
		for len(s) > 0 && s[0] != ' ' && s[0] != '\t' {
			s = s[1:]
		}
	}
	if opts.skipChars > 0 {
		if opts.skipChars >= len(s) {
			s = ""
		} else {
			s = s[opts.skipChars:]
		}
	}
	if opts.checkChars >= 0 && opts.checkChars < len(s) {
		s = s[:opts.checkChars]
	}
	if opts.ignoreCase {
		s = strings.ToLower(s)
	}
	return s
}

func deduplicate(r io.Reader, w *bufio.Writer, opts options) int {
	scanner := bufio.NewScanner(r)
	first := true
	var prev string
	var prevKey string
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		key := comparisonKey(line, opts)
		if first {
			prev = line
			prevKey = key
			count = 1
			first = false
			continue
		}
		if key == prevKey {
			count++
			continue
		}
		if err := emitRun(w, prev, count, opts); err != nil {
			return epipeOr1(err)
		}
		prev = line
		prevKey = key
		count = 1
	}
	if !first {
		if err := emitRun(w, prev, count, opts); err != nil {
			return epipeOr1(err)
		}
	}
	if err := w.Flush(); err != nil {
		return epipeOr1(err)
	}
	return 0
}

func emitRun(w *bufio.Writer, line string, count int, opts options) error {
	n := runCopies(count, opts)
	for range n {
		if opts.count {
			if _, err := fmt.Fprintf(w, "%7d %s\n", count, line); err != nil {
				return err
			}
		} else {
			if _, err := w.WriteString(line); err != nil {
				return err
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
		}
	}
	return nil
}

func epipeOr1(err error) int {
	if errors.Is(err, syscall.EPIPE) {
		return 0
	}
	return 1
}

func runCopies(count int, opts options) int {
	if (opts.repeated || opts.allRepeated) && opts.unique {
		return 0
	}
	if opts.allRepeated {
		if count > 1 {
			return count
		}
		return 0
	}
	if opts.repeated {
		if count > 1 {
			return 1
		}
		return 0
	}
	if opts.unique {
		if count == 1 {
			return 1
		}
		return 0
	}
	return 1
}
