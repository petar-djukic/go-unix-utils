// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd005-wc: Count Lines, Words, and Bytes.
// Covers R1.1-R1.4 (default counting, stdin/files, output format, totals),
// R2.1 (-l lines), R2.2 (-w words), R2.3/R2.4 (-c bytes, -m chars).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const defaultWidth = 7

// counts holds computed counts for a single input.
type counts struct {
	lines int64
	words int64
	bytes int64
	chars int64
}

// showFlags holds which counts to display.
type showFlags struct {
	lines bool
	words bool
	bytes bool
	chars bool
}

// noFlagsSet returns true when no counting flags were specified.
func (f showFlags) noFlagsSet() bool {
	return !f.lines && !f.words && !f.bytes && !f.chars
}

// effective returns display flags with defaults applied.
// R1.1: default shows lines, words, bytes.
// R2.3: -m takes precedence over -c when both given.
func (f showFlags) effective() showFlags {
	if f.noFlagsSet() {
		return showFlags{lines: true, words: true, bytes: true}
	}
	if f.chars && f.bytes {
		f.bytes = false
	}
	return f
}

func main() {
	sys.InstallSIGPIPEHandler()
	fl, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(fl, files))
}

// run processes all inputs and returns the exit code.
// R1.2: reads stdin when no files given. R1.4: total line for multiple files.
func run(fl showFlags, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	width := computeWidth(files)
	eff := fl.effective()
	return processFiles(files, eff, width)
}

// processFiles counts and prints results for each file.
// R6.1/R6.2: exit 0 on success, exit 1 if any file fails.
func processFiles(files []string, fl showFlags, width int) int {
	var total counts
	exitCode := 0
	showTotal := len(files) > 1

	for _, name := range files {
		c, err := countFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %v\n", err)
			exitCode = 1
			continue
		}
		addCounts(&total, c)
		printLine(c, displayName(name), width, fl)
	}

	if showTotal {
		printLine(total, "total", width, fl)
	}
	return exitCode
}

// addCounts accumulates src into dst.
func addCounts(dst *counts, src counts) {
	dst.lines += src.lines
	dst.words += src.words
	dst.bytes += src.bytes
	dst.chars += src.chars
}

// countFile opens and counts a single input.
func countFile(name string) (counts, error) {
	r, err := openInput(name)
	if err != nil {
		return counts{}, err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return countReader(r)
}

// countReader counts lines, words, bytes, and chars from r.
// R2.1: lines = newline count. R2.2: words = maximal non-whitespace sequences.
// R2.4: chars = Unicode code points; invalid bytes count as one character each.
func countReader(r io.Reader) (counts, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var c counts
	inWord := false

	for {
		ru, size, err := br.ReadRune()
		if err != nil {
			if err == io.EOF {
				return c, nil
			}
			return c, err
		}
		c.bytes += int64(size)
		c.chars++
		if ru == '\n' {
			c.lines++
		}
		if unicode.IsSpace(ru) {
			inWord = false
		} else if !inWord {
			c.words++
			inWord = true
		}
	}
}

// computeWidth determines the column width for output formatting.
// Uses file sizes from stat where possible; defaults to 7 for stdin.
func computeWidth(files []string) int {
	var totalSize int64
	anyRegular := false

	for _, name := range files {
		if name == "-" {
			continue
		}
		info, err := os.Stat(name)
		if err == nil {
			totalSize += info.Size()
			anyRegular = true
		}
	}

	if !anyRegular {
		return defaultWidth
	}
	w := digitCount(totalSize)
	if w < 1 {
		w = 1
	}
	return w
}

// digitCount returns the number of decimal digits in n.
func digitCount(n int64) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		n /= 10
		d++
	}
	return d
}

// printLine prints a single output line with right-aligned counts.
// R2.6: fixed order: lines, words, chars/bytes.
func printLine(c counts, name string, width int, fl showFlags) {
	first := true
	if fl.lines {
		printField(c.lines, width, &first)
	}
	if fl.words {
		printField(c.words, width, &first)
	}
	if fl.chars {
		printField(c.chars, width, &first)
	}
	if fl.bytes {
		printField(c.bytes, width, &first)
	}
	if name != "" {
		fmt.Printf(" %s", name)
	}
	fmt.Println()
}

// printField prints a single right-aligned count field.
func printField(val int64, width int, first *bool) {
	if *first {
		fmt.Printf("%*d", width, val)
		*first = false
	} else {
		fmt.Printf(" %*d", width, val)
	}
}

// displayName returns the display name for output.
// R1.3: stdin has no filename in output.
func displayName(name string) string {
	if name == "-" {
		return ""
	}
	return name
}

// openInput opens a named file or returns stdin for "-".
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError produces a GNU-compatible error message.
func formatOpenError(name string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("%s: %v", name, pathErr.Err)
	}
	return fmt.Errorf("%s: %v", name, err)
}

// parseArgs processes command-line arguments.
// Returns exit >= 0 for early exit (help/version/error).
func parseArgs(args []string) (fl showFlags, files []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			files = append(files, args[i+1:]...)
			return
		}
		exit = parseOneArg(args[i], &fl, &files)
		if exit >= 0 {
			return showFlags{}, nil, exit
		}
	}
	return
}

// parseOneArg handles a single argument. Returns -1 to continue, >= 0 to exit.
func parseOneArg(arg string, fl *showFlags, files *[]string) int {
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "--bytes":
		fl.bytes = true
	case arg == "--chars":
		fl.chars = true
	case arg == "--lines":
		fl.lines = true
	case arg == "--words":
		fl.words = true
	case strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-':
		return parseShortFlags(arg[1:], fl)
	case strings.HasPrefix(arg, "--"):
		fmt.Fprintf(os.Stderr, "wc: unrecognized option '%s'\n", arg)
		return 1
	default:
		*files = append(*files, arg)
	}
	return -1
}

// parseShortFlags handles combined short flags like -lwc.
func parseShortFlags(s string, fl *showFlags) int {
	for _, ch := range s {
		switch ch {
		case 'l':
			fl.lines = true
		case 'w':
			fl.words = true
		case 'c':
			fl.bytes = true
		case 'm':
			fl.chars = true
		default:
			fmt.Fprintf(os.Stderr, "wc: invalid option -- '%c'\n", ch)
			return 1
		}
	}
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: wc [OPTION]... [FILE]...
  or:  wc [OPTION]... --files0-from=F
Print newline, word, and byte counts for each FILE, and a total line if
more than one FILE is specified.  A word is a non-zero-length sequence of
printable characters delimited by white space.

With no FILE, or when FILE is -, read standard input.

The options below may be used to select which counts are printed, always in
the following order: newline, word, character, byte.
  -c, --bytes            print the byte counts
  -m, --chars            print the character counts
  -l, --lines            print the newline counts
  -w, --words            print the word counts
      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "wc (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
