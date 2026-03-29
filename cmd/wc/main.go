// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/wc implements GNU wc: count lines, words, and bytes.
//
// Implements prd005-wc R1.1, R1.2, R1.3, R1.4.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "wc"

// wcCounts holds the counts for a single input.
type wcCounts struct {
	lines int64
	words int64
	bytes int64
}

// wcResult pairs counts with a display name.
type wcResult struct {
	name   string // empty for stdin-only input
	counts wcCounts
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type runAction int

const (
	actionRun     runAction = iota
	actionHelp
	actionVersion
)

// run parses arguments and counts lines, words, and bytes.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	files, action := parseArgs(args)

	switch action {
	case actionHelp:
		printHelp(stdout)
		return 0
	case actionVersion:
		printVersion(stdout)
		return 0
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	return processFiles(files, stdin, stdout, stderr)
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) ([]string, runAction) {
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}
		switch {
		case arg == "--":
			flagsDone = true
		case arg == "--help":
			return nil, actionHelp
		case arg == "--version":
			return nil, actionVersion
		case arg == "-" || !isFlag(arg):
			files = append(files, arg)
		default:
			// Unrecognized flags ignored; future tasks add -l, -w, -c, -m, -L
		}
	}

	return files, actionRun
}

// isFlag reports whether arg looks like a flag.
func isFlag(arg string) bool {
	return len(arg) >= 2 && arg[0] == '-'
}

// processFiles counts and prints results for all files.
func processFiles(
	files []string, stdin io.Reader, stdout, stderr io.Writer,
) int {
	results, exitCode := collectResults(files, stdin, stderr)

	// R1.4: total line when more than one file is given.
	if len(files) > 1 {
		results = append(results, wcResult{
			name:   "total",
			counts: sumCounts(results),
		})
	}

	width := fieldWidth(results)
	for _, r := range results {
		if err := printResult(stdout, r, width); err != nil {
			return exitCode
		}
	}

	return exitCode
}

// collectResults processes each file and returns results.
func collectResults(
	files []string, stdin io.Reader, stderr io.Writer,
) ([]wcResult, int) {
	results := make([]wcResult, 0, len(files))
	exitCode := 0

	for _, name := range files {
		counts, err := countFile(name, stdin)
		if err != nil {
			if isBrokenPipe(err) {
				return results, 0
			}
			fmt.Fprintf(stderr, "%s: %s: %v\n",
				programName, name, unwrapErr(err))
			exitCode = 1
			continue
		}
		displayName := name
		if name == "-" {
			displayName = ""
		}
		results = append(results, wcResult{
			name: displayName, counts: counts,
		})
	}

	return results, exitCode
}

// countFile opens and counts a single file or stdin.
func countFile(name string, stdin io.Reader) (wcCounts, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return wcCounts{}, err
	}
	if closer != nil {
		defer closer.Close()
	}
	return count(r)
}

// openInput returns a reader and optional closer for the given filename.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// count reads all input and returns line, word, and byte counts.
// R1.1: lines = newline count, words = maximal non-whitespace sequences.
func count(r io.Reader) (wcCounts, error) {
	buf := make([]byte, 64*1024)
	var c wcCounts
	inWord := false

	for {
		n, err := r.Read(buf)
		c.bytes += int64(n)
		countChunk(buf[:n], &c, &inWord)
		if err == io.EOF {
			break
		}
		if err != nil {
			return c, err
		}
	}

	return c, nil
}

// countChunk counts lines and words in a buffer chunk.
func countChunk(buf []byte, c *wcCounts, inWord *bool) {
	for _, b := range buf {
		if b == '\n' {
			c.lines++
		}
		if isSpace(b) {
			*inWord = false
		} else if !*inWord {
			*inWord = true
			c.words++
		}
	}
}

// isSpace matches C isspace() under LC_ALL=C.
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' ||
		b == '\r' || b == '\f' || b == '\v'
}

// sumCounts adds all result counts together.
func sumCounts(results []wcResult) wcCounts {
	var total wcCounts
	for _, r := range results {
		total.lines += r.counts.lines
		total.words += r.counts.words
		total.bytes += r.counts.bytes
	}
	return total
}

// fieldWidth returns the column width for right-aligned formatting.
func fieldWidth(results []wcResult) int {
	var maxVal int64
	for _, r := range results {
		for _, v := range []int64{
			r.counts.lines, r.counts.words, r.counts.bytes,
		} {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	return digitCount(maxVal)
}

// digitCount returns the number of decimal digits in v.
func digitCount(v int64) int {
	if v == 0 {
		return 1
	}
	n := 0
	for v > 0 {
		v /= 10
		n++
	}
	return n
}

// printResult writes one line of wc output.
// R1.3: counts followed by filename; no filename for stdin.
func printResult(w io.Writer, r wcResult, width int) error {
	if r.name == "" {
		_, err := fmt.Fprintf(w, "%*d %*d %*d\n",
			width, r.counts.lines,
			width, r.counts.words,
			width, r.counts.bytes)
		return err
	}
	_, err := fmt.Fprintf(w, "%*d %*d %*d %s\n",
		width, r.counts.lines,
		width, r.counts.words,
		width, r.counts.bytes,
		r.name)
	return err
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: %s [OPTION]... [FILE]...
Print newline, word, and byte counts for each FILE, and a total line if
more than one FILE is specified.

With no FILE, or when FILE is -, read standard input.

  -c, --bytes            print the byte counts
  -m, --chars            print the character counts
  -l, --lines            print the newline counts
  -L, --max-line-length  print the maximum display width
  -w, --words            print the word counts
      --help             display this help and exit
      --version          output version information and exit
`, programName)
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", programName)
}

// unwrapErr extracts the underlying syscall error from os.PathError.
func unwrapErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// isBrokenPipe reports whether an error is caused by writing to a broken pipe.
func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}
