// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd005-wc R1.1–R1.4
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultWidth is the minimum column width GNU wc uses when the file size
// is unknown (stdin, pipes, or the "-" argument).
const defaultWidth = 7

// counts holds the accumulated counts for a single input.
type counts struct {
	lines int64
	words int64
	bytes int64
}

// fileResult pairs a count with its metadata.
type fileResult struct {
	c    counts
	name string
	ok   bool
}

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	os.Exit(run(os.Args[1:]))
}

// run parses arguments and processes inputs. Returns exit code.
func run(args []string) int {
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	// R1.1: default behavior — print lines, words, bytes.
	if len(args) == 0 {
		// R1.2: no file arguments — read from stdin.
		c, err := count(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: standard input: %v\n", err)
			exitCode = 1
		}
		// R1.3: no filename for stdin-only input. Width defaults to 7.
		printCounts(w, c, "", defaultWidth)
	} else {
		// Determine column width from file sizes before counting.
		// GNU wc stats files upfront to determine width. If any input
		// is stdin ("-") or unsizable, it falls back to defaultWidth.
		width := computeWidth(args)

		// R1.2: read from named files in order.
		results := make([]fileResult, 0, len(args))
		var total counts
		for _, arg := range args {
			c, err := countFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "wc: %s\n", formatError(arg, err))
				exitCode = 1
				results = append(results, fileResult{name: arg, ok: false})
				continue
			}
			total.lines += c.lines
			total.words += c.words
			total.bytes += c.bytes
			results = append(results, fileResult{c: c, name: arg, ok: true})
		}

		for _, r := range results {
			if !r.ok {
				// GNU wc does not print a line for files that failed to open.
				continue
			}
			printCounts(w, r.c, r.name, width)
		}

		// R1.4: print total when more than one file argument.
		if len(args) > 1 {
			printCounts(w, total, "total", width)
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "wc: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// computeWidth determines the output column width by examining file sizes.
// If any argument is "-" (stdin) or cannot be statted, returns defaultWidth.
// Otherwise returns the number of digits in the largest file size.
func computeWidth(args []string) int {
	var maxSize int64
	for _, arg := range args {
		if arg == "-" {
			return defaultWidth
		}
		fi, err := os.Stat(arg)
		if err != nil {
			// Can't determine size; will be reported later during counting.
			continue
		}
		if fi.Size() > maxSize {
			maxSize = fi.Size()
		}
	}
	w := numberWidth(maxSize)
	if w < 1 {
		return 1
	}
	return w
}

// countFile opens a named file and counts its contents.
// R1.3: "-" means stdin.
func countFile(name string) (counts, error) {
	if name == "-" {
		return count(os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return counts{}, err
	}
	defer f.Close() // best-effort cleanup, error ignored
	return count(f)
}

// count reads from r and returns line, word, and byte counts.
// R1.1: lines are newline characters, words are maximal sequences of
// non-whitespace characters, bytes is the total byte count.
func count(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReader(r)
	inWord := false

	for {
		ru, size, err := br.ReadRune()
		if size > 0 {
			c.bytes += int64(size)
			if ru == utf8.RuneError && size == 1 {
				// Invalid UTF-8 byte — not whitespace, counts as word content.
				if !inWord {
					c.words++
					inWord = true
				}
				continue
			}
			if ru == '\n' {
				c.lines++
			}
			if unicode.IsSpace(ru) {
				inWord = false
			} else {
				if !inWord {
					c.words++
					inWord = true
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return c, fmt.Errorf("read error: %w", err)
		}
	}
	return c, nil
}

// printCounts writes a single output line in GNU wc format.
// R1.3: counts are right-aligned; filename is appended when non-empty.
func printCounts(w *bufio.Writer, c counts, name string, width int) {
	if name != "" {
		fmt.Fprintf(w, "%*d %*d %*d %s\n", width, c.lines, width, c.words, width, c.bytes, name)
	} else {
		fmt.Fprintf(w, "%*d %*d %*d\n", width, c.lines, width, c.words, width, c.bytes)
	}
}

// formatError formats an os.Open error to match GNU wc error message style.
// GNU wc prints: "path: Reason" (no "open" prefix, capitalized reason).
func formatError(name string, err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Sprintf("%s: %s", name, capitalizeFirst(pathErr.Err.Error()))
	}
	return err.Error()
}

// capitalizeFirst returns s with the first byte uppercased if it is ASCII lowercase.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// numberWidth returns the number of decimal digits needed to display n.
func numberWidth(n int64) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for n > 0 {
		n /= 10
		w++
	}
	return w
}
