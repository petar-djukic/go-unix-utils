// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/wc implements GNU wc: count lines, words, and bytes.
//
// Implements prd005-wc R1.1–R1.4, R2.1–R2.4, R3.1, R3.2, R4.4.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
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

// parsedArgs holds the result of argument parsing.
type parsedArgs struct {
	files      []string
	files0From string // --files0-from=FILE, empty if not set
	action     runAction
}

// run parses arguments and counts lines, words, and bytes.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	parsed := parseArgs(args)

	switch parsed.action {
	case actionHelp:
		printHelp(stdout)
		return 0
	case actionVersion:
		printVersion(stdout)
		return 0
	}

	if parsed.files0From != "" {
		return runFiles0From(parsed, stdin, stdout, stderr)
	}

	files := parsed.files
	if len(files) == 0 {
		// R2.4/R3.2: no file operands — read from stdin, omit filename.
		files = []string{""}
	}

	return processWithWidth(files, precomputeWidth(files),
		stdin, stdout, stderr)
}

// runFiles0From handles the --files0-from flag logic.
func runFiles0From(
	parsed parsedArgs, stdin io.Reader, stdout, stderr io.Writer,
) int {
	// R2.6 (task): error if file operands combined with --files0-from.
	if len(parsed.files) > 0 {
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n",
			programName, parsed.files[0])
		fmt.Fprintf(stderr,
			"file operands cannot be combined with --files0-from\n")
		fmt.Fprintf(stderr,
			"Try '%s --help' for more information.\n", programName)
		return 1
	}

	files, err := readFiles0From(parsed.files0From, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot open '%s' for reading: %v\n",
			programName, parsed.files0From, unwrapErr(err))
		return 1
	}

	if err := validateFiles0Names(files, parsed.files0From, stderr); err != nil {
		return 1
	}

	if len(files) == 0 {
		return 0
	}

	// GNU wc precomputes width from file sizes when --files0-from reads
	// from a regular file, but uses minimum width (1) when reading from
	// stdin because it processes filenames in streaming fashion.
	width := 1
	if parsed.files0From != "-" {
		width = precomputeWidth(files)
	}

	return processWithWidth(files, width, stdin, stdout, stderr)
}

// validateFiles0Names checks for invalid filenames from --files0-from.
func validateFiles0Names(
	files []string, source string, stderr io.Writer,
) error {
	if slices.Contains(files, "") {
		fmt.Fprintf(stderr,
			"%s: %s: invalid zero-length file name\n",
			programName, source)
		return fmt.Errorf("invalid filename")
	}
	return nil
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) parsedArgs {
	var p parsedArgs
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			p.files = append(p.files, arg)
			continue
		}
		i = parseOneArg(arg, args, i, &p, &flagsDone)
	}

	return p
}

// parseOneArg processes a single argument and returns the updated index.
func parseOneArg(
	arg string, args []string, i int,
	p *parsedArgs, flagsDone *bool,
) int {
	switch {
	case arg == "--":
		*flagsDone = true
	case arg == "--help":
		p.action = actionHelp
	case arg == "--version":
		p.action = actionVersion
	case strings.HasPrefix(arg, "--files0-from="):
		p.files0From = arg[len("--files0-from="):]
	case arg == "--files0-from" && i+1 < len(args):
		i++
		p.files0From = args[i]
	case arg == "-" || !isFlag(arg):
		p.files = append(p.files, arg)
	default:
		// Unrecognized flags ignored; future tasks add -l, -w, -c, -m, -L
	}
	return i
}

// isFlag reports whether arg looks like a flag.
func isFlag(arg string) bool {
	return len(arg) >= 2 && arg[0] == '-'
}

// stdinDefaultWidth is the column width GNU wc uses when stdin size is unknown.
const stdinDefaultWidth = 7

// processWithWidth counts and prints results for all files with given width.
// R3.1: right-aligned columns. R1.4/R3.2: total line for multi-file.
func processWithWidth(
	files []string, width int,
	stdin io.Reader, stdout, stderr io.Writer,
) int {
	results, exitCode := collectResults(files, stdin, stderr)
	results = addTotal(files, results)
	printAllResults(stdout, results, width)
	return exitCode
}

// addTotal appends a total line when more than one file is given.
// R1.4, R3.2: total line with label "total".
func addTotal(files []string, results []wcResult) []wcResult {
	if len(files) > 1 {
		return append(results, wcResult{
			name: "total", counts: sumCounts(results),
		})
	}
	return results
}

// printAllResults writes all result lines.
func printAllResults(w io.Writer, results []wcResult, width int) {
	for _, r := range results {
		if err := printResult(w, r, width); err != nil {
			return
		}
	}
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
		// R2.3: explicit "-" displays as "-".
		// R2.4/R3.2: implicit stdin (empty name) displays no filename.
		results = append(results, wcResult{
			name: name, counts: counts,
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
// R4.1: "-" means stdin. R2.4: "" means implicit stdin (no file operands).
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" || name == "" {
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

// precomputeWidth determines column width from file sizes before counting,
// matching GNU wc behavior. For stdin (name "" or "-"), uses stdinDefaultWidth.
// For regular files, uses digits(file_size). Returns the maximum across all.
// R3.1: right-aligned columns with consistent field width.
func precomputeWidth(files []string) int {
	width := 1
	for _, name := range files {
		if name == "" || name == "-" {
			if stdinDefaultWidth > width {
				width = stdinDefaultWidth
			}
			continue
		}
		info, err := os.Stat(name)
		if err != nil {
			continue
		}
		w := digitCount(info.Size())
		if w > width {
			width = w
		}
	}
	return width
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
// R1.3: counts followed by filename; R3.2: no filename for stdin-only.
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

// readFiles0From reads null-delimited filenames from the given path.
// R4.4: when path is "-", reads from stdin.
func readFiles0From(path string, stdin io.Reader) ([]string, error) {
	if path == "-" {
		return scanNullDelimited(stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return scanNullDelimited(f)
}

// scanNullDelimited reads null-byte-delimited strings from r (D1).
func scanNullDelimited(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(splitNull)
	var files []string
	for scanner.Scan() {
		files = append(files, scanner.Text())
	}
	return files, scanner.Err()
}

// splitNull is a bufio.SplitFunc that splits on null bytes.
func splitNull(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: %s [OPTION]... [FILE]...
  or:  %s [OPTION]... --files0-from=F
Print newline, word, and byte counts for each FILE, and a total line if
more than one FILE is specified.

With no FILE, or when FILE is -, read standard input.

      --files0-from=F    read input from the files specified by
                           NUL-terminated names in file F;
                           If F is - then read names from standard input
  -c, --bytes            print the byte counts
  -m, --chars            print the character counts
  -l, --lines            print the newline counts
  -L, --max-line-length  print the maximum display width
  -w, --words            print the word counts
      --help             display this help and exit
      --version          output version information and exit
`, programName, programName)
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
