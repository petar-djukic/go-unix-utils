// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd005-wc R1.1–R1.4: default wc behavior with line, word,
// and byte counting from stdin or named files, with totals for multiple files.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "wc"

// counts holds line, word, and byte counts for a single input.
type counts struct {
	lines int64
	words int64
	bytes int64
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes files, returning the exit code.
// R1.2: reads stdin when no file args; reads named files in order.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	files := parseArgs(args)
	if len(files) == 0 {
		// R1.3: implicit stdin, no filename printed.
		files = []string{""}
	}
	return processFiles(files, stdin, stdout, stderr)
}

// parseArgs extracts file arguments from the command line.
// Handles "--" as end-of-flags and "-" as explicit stdin.
func parseArgs(args []string) []string {
	var files []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		files = append(files, arg)
	}
	return files
}

// processFiles counts and prints results for all files.
// R1.4: prints a total line when more than one file is given.
func processFiles(files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	width := computeWidth(files)
	exitCode := 0
	var total counts

	for _, name := range files {
		c, err := countFile(name, stdin)
		if err != nil {
			w.Flush() // best-effort flush before stderr
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		total = addCounts(total, c)
		printCounts(w, c, width, name)
	}
	if len(files) > 1 {
		printCounts(w, total, width, "total")
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// computeWidth determines the column width for count formatting.
// Uses fstat on files to match GNU wc's pre-processing width calculation.
// When no files can be statted (stdin-only), defaults to 7 matching GNU wc.
func computeWidth(files []string) int {
	maxSize := int64(-1)
	for _, name := range files {
		if name == "" || name == "-" {
			continue
		}
		fi, err := os.Stat(name)
		if err != nil {
			continue
		}
		if fi.Size() > maxSize {
			maxSize = fi.Size()
		}
	}
	if maxSize < 0 {
		return 7 // no statable files; GNU wc default for stdin/pipes
	}
	return numDigits(maxSize)
}

// numDigits returns the number of decimal digits in n.
func numDigits(n int64) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

// countFile opens a file (or reads stdin) and returns its counts.
// R1.2: "" and "-" both read from stdin.
func countFile(name string, stdin io.Reader) (counts, error) {
	if name == "" || name == "-" {
		return countReader(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return counts{}, err
	}
	defer f.Close() // best-effort close on read-only file
	return countReader(f)
}

// countReader reads all data from r and returns line, word, and byte counts.
// R1.1: counts newlines, words (maximal non-whitespace sequences), and bytes.
func countReader(r io.Reader) (counts, error) {
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
			if isSpaceByte(b) {
				inWord = false
			} else if !inWord {
				c.words++
				inWord = true
			}
		}
		if err == io.EOF {
			return c, nil
		}
		if err != nil {
			return c, err
		}
	}
}

// isSpaceByte returns true for C locale whitespace characters.
// Matches isspace() under LC_ALL=C: space, tab, newline, vtab, formfeed, cr.
func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' ||
		b == '\v' || b == '\f' || b == '\r'
}

// printCounts writes a formatted counts line.
// R1.3: counts followed by filename; no filename for implicit stdin (name="").
func printCounts(w *bufio.Writer, c counts, width int, name string) {
	fmt.Fprintf(w, "%*d %*d %*d", width, c.lines, width, c.words, width, c.bytes)
	if name != "" {
		fmt.Fprintf(w, " %s", name)
	}
	w.WriteByte('\n')
}

// addCounts returns the element-wise sum of two counts.
func addCounts(a, b counts) counts {
	return counts{
		lines: a.lines + b.lines,
		words: a.words + b.words,
		bytes: a.bytes + b.bytes,
	}
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
