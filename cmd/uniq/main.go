// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd028-uniq.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type options struct {
	count       bool
	repeated    bool
	allRepeated bool
	unique      bool
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
		os.Exit(1)
	}
	os.Exit(exitCode)
}

func parseArgs(args []string) (options, string, string, error) {
	var opts options
	var positional []string
	for i := range len(args) {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "-" || len(a) == 0 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}
		for _, ch := range a[1:] {
			if err := parseFlag(&opts, ch); err != nil {
				return options{}, "", "", err
			}
		}
	}
	inputFile := ""
	outputFile := ""
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

func deduplicate(r io.Reader, w *bufio.Writer, opts options) int {
	scanner := bufio.NewScanner(r)
	first := true
	var prev string
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			prev = line
			count = 1
			first = false
			continue
		}
		if line == prev {
			count++
			continue
		}
		if err := emitRun(w, prev, count, opts); err != nil {
			return 1
		}
		prev = line
		count = 1
	}
	if !first {
		if err := emitRun(w, prev, count, opts); err != nil {
			return 1
		}
	}
	if err := w.Flush(); err != nil {
		return 1
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
