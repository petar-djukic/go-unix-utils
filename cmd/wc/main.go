// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the wc utility for counting lines, words, and bytes.
//
// Implements prd005-wc (R1.1, R1.2, R1.3, R1.4).
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// options holds the parsed command-line flags for wc.
type options struct {
	lines bool
	words bool
	bytes bool
}

// counts holds the accumulated counts for a single input.
type counts struct {
	lines int64
	words int64
	bytes int64
}

// reportError writes a wc-style error message to stderr.
// R6.2: per-file error messages to stderr.
func reportError(name string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "wc: %s: %v\n", pathErr.Path, pathErr.Err)
	} else {
		fmt.Fprintf(os.Stderr, "wc: %s: %v\n", name, err)
	}
}

// computeWidth determines the output column width by examining file sizes.
// Files that cannot be stat'd (including stdin) default to width 7, matching
// GNU wc behavior.
func computeWidth(files []string) int {
	width := 1
	for _, name := range files {
		if name == "-" {
			if width < 7 {
				width = 7
			}
			continue
		}
		info, err := os.Stat(name)
		if err != nil {
			if width < 7 {
				width = 7
			}
			continue
		}
		w := digitCount(info.Size())
		if w > width {
			width = w
		}
	}
	return width
}

// digitCount returns the number of decimal digits needed to display n.
func digitCount(n int64) int {
	if n <= 0 {
		return 1
	}
	count := 0
	for n > 0 {
		count++
		n /= 10
	}
	return count
}

// countFile processes a single file or stdin and returns its counts.
// R1.2: "-" means stdin.
func countFile(name string) (counts, error) {
	if name == "-" {
		return countInput(os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return counts{}, err
	}
	defer f.Close()
	return countInput(f)
}

// countInput reads from r and returns line, word, and byte counts.
// R1.1: lines are newline characters, words are maximal non-whitespace sequences.
func countInput(r io.Reader) (counts, error) {
	var c counts
	buf := make([]byte, 32*1024)
	inWord := false
	for {
		n, err := r.Read(buf)
		c.bytes += int64(n)
		for _, b := range buf[:n] {
			if b == '\n' {
				c.lines++
			}
			if isByteSpace(b) {
				inWord = false
			} else if !inWord {
				c.words++
				inWord = true
			}
		}
		if err != nil {
			if err == io.EOF {
				return c, nil
			}
			return c, err
		}
	}
}

// isByteSpace reports whether b is an ASCII whitespace character,
// matching C isspace() under LC_ALL=C.
func isByteSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// printLine writes a single output line with the requested counts.
// R1.3: counts are right-justified, followed by the filename.
// R2.6: column order is lines, words, bytes.
func printLine(w *bufio.Writer, c counts, name string, showName bool, opts *options, width int) {
	first := true
	writeField := func(val int64) {
		if !first {
			w.WriteByte(' ')
		}
		fmt.Fprintf(w, "%*d", width, val)
		first = false
	}

	if opts.lines {
		writeField(c.lines)
	}
	if opts.words {
		writeField(c.words)
	}
	if opts.bytes {
		writeField(c.bytes)
	}

	if showName {
		fmt.Fprintf(w, " %s", name)
	}
	w.WriteByte('\n')
}

func main() {
	// R5.1: handle SIGPIPE by exiting cleanly.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	go func() {
		<-sigpipe
		os.Exit(0)
	}()

	var opts options
	flag.BoolVar(&opts.lines, "l", false, "count lines")
	flag.BoolVar(&opts.words, "w", false, "count words")
	flag.BoolVar(&opts.bytes, "c", false, "count bytes")
	flag.Parse()

	// R1.1: default mode prints lines, words, and bytes.
	if !opts.lines && !opts.words && !opts.bytes {
		opts.lines = true
		opts.words = true
		opts.bytes = true
	}

	// R1.2: read from stdin when no file arguments are given.
	args := flag.Args()
	showNames := len(args) > 0
	if len(args) == 0 {
		args = []string{"-"}
	}

	// Pre-compute column width from file sizes.
	width := computeWidth(args)

	out := bufio.NewWriter(os.Stdout)
	exitCode := 0
	var total counts

	for _, name := range args {
		c, err := countFile(name)
		if err != nil {
			reportError(name, err)
			exitCode = 1
			continue
		}
		printLine(out, c, name, showNames, &opts, width)
		total.lines += c.lines
		total.words += c.words
		total.bytes += c.bytes
	}

	// R1.4: print total when more than one file is given.
	if len(args) > 1 {
		printLine(out, total, "total", true, &opts, width)
	}

	// R6.3: detect write errors on final flush.
	if err := out.Flush(); err != nil {
		exitCode = 1
	}

	os.Exit(exitCode)
}
