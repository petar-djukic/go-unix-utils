// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd005-wc R1.1–R1.4, R2.1–R2.6
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

// stdinMinWidth is the minimum column width GNU wc uses for stdin when
// multiple columns are displayed.
const stdinMinWidth = 7

// tabWidth is the tab stop interval used by -L for display column calculation.
// R2.5: each tab advances to the next multiple of 8.
const tabWidth = 8

// counts holds the accumulated counts for a single input.
type counts struct {
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
}

// selection records which counts the user requested via flags.
// R2.6: output column order is always lines, words, chars, bytes, max-line-length.
type selection struct {
	lines      bool
	words      bool
	bytesFlag  bool
	chars      bool
	maxLineLen bool
}

// anySet returns true if the user explicitly selected at least one flag.
func (s selection) anySet() bool {
	return s.lines || s.words || s.bytesFlag || s.chars || s.maxLineLen
}

// numColumns returns how many count columns will be displayed.
func (s selection) numColumns() int {
	n := 0
	if s.lines {
		n++
	}
	if s.words {
		n++
	}
	if s.chars {
		n++
	}
	if s.bytesFlag {
		n++
	}
	if s.maxLineLen {
		n++
	}
	return n
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
	sel, fileArgs := parseFlags(args)

	// R2.2: when no selection flag is given, default to lines, words, bytes.
	if !sel.anySet() {
		sel.lines = true
		sel.words = true
		sel.bytesFlag = true
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	isStdin := len(fileArgs) == 0

	if isStdin {
		// R1.2: no file arguments — read from stdin.
		c, err := count(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: standard input: %v\n", err)
			exitCode = 1
		}
		width := computeWidthFromCounts([]counts{c}, counts{}, false, sel, true)
		// R1.3: no filename for stdin-only input.
		printCounts(w, c, "", width, sel)
	} else {
		// Determine column width upfront for multi-file case.
		// GNU wc uses number_width(sum_of_file_sizes) for multi-file,
		// and count-based width for single file.
		width := computeWidthForFiles(fileArgs)

		// R1.2: read from named files in order.
		results := make([]fileResult, 0, len(fileArgs))
		var total counts
		for _, arg := range fileArgs {
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
			total.chars += c.chars
			if c.maxLineLen > total.maxLineLen {
				total.maxLineLen = c.maxLineLen
			}
			results = append(results, fileResult{c: c, name: arg, ok: true})
		}

		showTotal := len(fileArgs) > 1

		// For single file (not stdin via "-"), recompute width from actual
		// counts. GNU wc uses count-based width for single file but
		// stdinMinWidth when the single arg is "-".
		if len(fileArgs) == 1 && fileArgs[0] != "-" {
			for _, r := range results {
				if r.ok {
					width = computeWidthFromCounts([]counts{r.c}, counts{}, false, sel, false)
				}
			}
		}

		for _, r := range results {
			if !r.ok {
				// GNU wc does not print a line for files that failed to open.
				continue
			}
			printCounts(w, r.c, r.name, width, sel)
		}

		// R1.4: print total when more than one file argument.
		if showTotal {
			printCounts(w, total, "total", width, sel)
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "wc: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// computeWidthForFiles determines column width for file arguments by
// examining file sizes. GNU wc uses number_width(sum_of_sizes) for multi-file.
// If any argument is "-" (stdin), returns stdinMinWidth since stdin can't be statted.
func computeWidthForFiles(args []string) int {
	var totalSize int64
	for _, arg := range args {
		if arg == "-" {
			return stdinMinWidth
		}
		fi, err := os.Stat(arg)
		if err != nil {
			// Can't determine size; will be reported later during counting.
			continue
		}
		totalSize += fi.Size()
	}
	w := numberWidth(totalSize)
	if w < 1 {
		return 1
	}
	return w
}

// computeWidthFromCounts determines column width from actual count values.
// Used for single-file and stdin cases where GNU wc sizes columns to fit
// the largest displayed value. For stdin with multiple columns, a minimum
// width of stdinMinWidth applies.
func computeWidthFromCounts(fileCounts []counts, total counts, includeTotal bool, sel selection, isStdin bool) int {
	minWidth := 1
	if isStdin && sel.numColumns() > 1 {
		minWidth = stdinMinWidth
	}

	var maxVal int64
	updateMax := func(c counts) {
		if sel.lines && c.lines > maxVal {
			maxVal = c.lines
		}
		if sel.words && c.words > maxVal {
			maxVal = c.words
		}
		if sel.chars && c.chars > maxVal {
			maxVal = c.chars
		}
		if sel.bytesFlag && c.bytes > maxVal {
			maxVal = c.bytes
		}
		if sel.maxLineLen && c.maxLineLen > maxVal {
			maxVal = c.maxLineLen
		}
	}

	for _, c := range fileCounts {
		updateMax(c)
	}
	if includeTotal {
		updateMax(total)
	}

	w := numberWidth(maxVal)
	if w < minWidth {
		return minWidth
	}
	return w
}

// parseFlags extracts selection flags and file arguments from args.
// GNU wc accepts flags with a single dash, supports combined flags (e.g., -lw),
// and treats "--" as end-of-flags.
func parseFlags(args []string) (selection, []string) {
	var sel selection
	var fileArgs []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags || arg == "-" || !isFlag(arg) {
			fileArgs = append(fileArgs, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		// Process each character in the flag string.
		for _, ch := range arg[1:] {
			switch ch {
			case 'l':
				sel.lines = true
			case 'w':
				sel.words = true
			case 'c':
				sel.bytesFlag = true
			case 'm':
				sel.chars = true
			case 'L':
				sel.maxLineLen = true
			default:
				fmt.Fprintf(os.Stderr, "wc: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
	}
	return sel, fileArgs
}

// isFlag returns true if arg looks like a flag (starts with "-" and has more chars).
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
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

// count reads from r and returns line, word, byte, char, and max-line-length counts.
// R1.1: lines are newline characters, words are maximal sequences of
// non-whitespace characters, bytes is the total byte count.
// R2.4: chars counts Unicode code points; invalid UTF-8 bytes count as one char each.
// R2.5: max-line-length measures display columns with tab expansion to multiples of 8.
func count(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReader(r)
	inWord := false
	var lineLen int64

	for {
		ru, size, err := br.ReadRune()
		if size > 0 {
			c.bytes += int64(size)
			c.chars++
			if ru == utf8.RuneError && size == 1 {
				// Invalid UTF-8 byte — not whitespace, counts as word content.
				// R2.4: each invalid byte counts as one character (already incremented above).
				// R2.5: invalid byte occupies one display column.
				lineLen++
				if !inWord {
					c.words++
					inWord = true
				}
				continue
			}
			if ru == '\n' {
				c.lines++
				if lineLen > c.maxLineLen {
					c.maxLineLen = lineLen
				}
				lineLen = 0
				inWord = false
			} else if ru == '\t' {
				// R2.5: tab advances to the next multiple of tabWidth.
				lineLen = ((lineLen / tabWidth) + 1) * tabWidth
				inWord = false
			} else if unicode.IsSpace(ru) {
				lineLen++
				inWord = false
			} else {
				lineLen++
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
	// R2.5: account for the last line if it has no trailing newline.
	if lineLen > c.maxLineLen {
		c.maxLineLen = lineLen
	}
	return c, nil
}

// printCounts writes a single output line in GNU wc format.
// R2.6: column order is always lines, words, chars, bytes, max-line-length.
// R1.3: counts are right-aligned; filename is appended when non-empty.
func printCounts(w *bufio.Writer, c counts, name string, width int, sel selection) {
	first := true
	writeField := func(val int64) {
		if first {
			fmt.Fprintf(w, "%*d", width, val)
			first = false
		} else {
			fmt.Fprintf(w, " %*d", width, val)
		}
	}

	// R2.6: fixed column order: lines, words, chars, bytes, max-line-length.
	if sel.lines {
		writeField(c.lines)
	}
	if sel.words {
		writeField(c.words)
	}
	if sel.chars {
		writeField(c.chars)
	}
	if sel.bytesFlag {
		writeField(c.bytes)
	}
	if sel.maxLineLen {
		writeField(c.maxLineLen)
	}

	if name != "" {
		fmt.Fprintf(w, " %s", name)
	}
	fmt.Fprint(w, "\n")
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
