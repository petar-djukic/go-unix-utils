// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd005-wc: Count Lines, Words, and Bytes.
// Covers R1.1-R1.4, R2.1-R2.6, R3.1-R3.2, R4.1-R4.4, R6.1-R6.2.
package main

import (
	"bufio"
	"bytes"
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
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
}

// showFlags holds which counts to display.
type showFlags struct {
	lines      bool
	words      bool
	bytes      bool
	chars      bool
	maxLineLen bool
}

// options holds parsed command-line options.
type options struct {
	flags      showFlags
	files      []string
	files0From string
}

// noFlagsSet returns true when no counting flags were specified.
func (f showFlags) noFlagsSet() bool {
	return !f.lines && !f.words && !f.bytes && !f.chars && !f.maxLineLen
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
	opts, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(opts))
}

// run processes all inputs and returns the exit code.
func run(opts options) int {
	files, err := resolveFiles(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wc: %v\n", err)
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	width := computeWidth(files)
	eff := opts.flags.effective()
	return processFiles(files, eff, width)
}

// resolveFiles determines the file list from options.
// R3.2: --files0-from with file operands is an error.
func resolveFiles(opts options) ([]string, error) {
	if opts.files0From == "" {
		return opts.files, nil
	}
	if len(opts.files) > 0 {
		return nil, fmt.Errorf(
			"file operands cannot be combined with --files0-from")
	}
	return readFiles0From(opts.files0From)
}

// readFiles0From reads NUL-terminated filenames from a file or stdin.
// R4.4: when FILE is "-", filenames are read from stdin.
func readFiles0From(source string) ([]string, error) {
	r, err := openFiles0Source(source)
	if err != nil {
		return nil, err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}
	return splitNulFiles(data), nil
}

// openFiles0Source opens the --files0-from source.
func openFiles0Source(source string) (*os.File, error) {
	if source == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(source)
	if err != nil {
		return nil, formatOpenError(source, err)
	}
	return f, nil
}

// splitNulFiles splits NUL-terminated data into filenames.
func splitNulFiles(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	data = bytes.TrimRight(data, "\x00")
	if len(data) == 0 {
		return nil
	}
	parts := bytes.Split(data, []byte{0})
	files := make([]string, len(parts))
	for i, p := range parts {
		files[i] = string(p)
	}
	return files
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
// R2.5: maxLineLen takes the maximum, not the sum.
func addCounts(dst *counts, src counts) {
	dst.lines += src.lines
	dst.words += src.words
	dst.bytes += src.bytes
	dst.chars += src.chars
	if src.maxLineLen > dst.maxLineLen {
		dst.maxLineLen = src.maxLineLen
	}
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

// countReader counts lines, words, bytes, chars, and max line length.
// R2.1: lines = newline count. R2.2: words = maximal non-whitespace.
// R2.4: chars = Unicode code points. R2.5: max line length with tab expansion.
func countReader(r io.Reader) (counts, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var c counts
	inWord := false
	var lineLen int64

	for {
		ru, size, err := br.ReadRune()
		if err != nil {
			if err == io.EOF {
				updateMaxLineLen(&c, lineLen)
				return c, nil
			}
			return c, err
		}
		c.bytes += int64(size)
		c.chars++
		updateLineLen(&lineLen, ru, &c)
		updateWordState(ru, &inWord, &c)
	}
}

// updateLineLen updates the current line length and max on newline.
// R2.5: tab advances to the next multiple of 8.
func updateLineLen(lineLen *int64, ru rune, c *counts) {
	switch ru {
	case '\n':
		c.lines++
		updateMaxLineLen(c, *lineLen)
		*lineLen = 0
	case '\t':
		*lineLen += 8 - (*lineLen % 8)
	default:
		*lineLen++
	}
}

// updateWordState tracks word boundaries for word counting.
func updateWordState(ru rune, inWord *bool, c *counts) {
	if unicode.IsSpace(ru) {
		*inWord = false
	} else if !*inWord {
		c.words++
		*inWord = true
	}
}

// updateMaxLineLen updates the max line length if current is larger.
func updateMaxLineLen(c *counts, lineLen int64) {
	if lineLen > c.maxLineLen {
		c.maxLineLen = lineLen
	}
}

// computeWidth determines the column width for output formatting.
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
// R2.6: fixed order: lines, words, chars/bytes, max-line-length.
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
	if fl.maxLineLen {
		printField(c.maxLineLen, width, &first)
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
// R4.1: "-" means stdin.
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
func parseArgs(args []string) (options, int) {
	var opts options
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			return opts, -1
		}
		exit := parseOneArg(args[i], &opts)
		if exit >= 0 {
			return options{}, exit
		}
	}
	return opts, -1
}

// parseOneArg handles a single argument.
func parseOneArg(arg string, opts *options) int {
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "--bytes":
		opts.flags.bytes = true
	case arg == "--chars":
		opts.flags.chars = true
	case arg == "--lines":
		opts.flags.lines = true
	case arg == "--words":
		opts.flags.words = true
	case arg == "--max-line-length":
		opts.flags.maxLineLen = true
	case strings.HasPrefix(arg, "--files0-from="):
		opts.files0From = arg[len("--files0-from="):]
	case isShortFlag(arg):
		return parseShortFlags(arg[1:], &opts.flags)
	case strings.HasPrefix(arg, "--"):
		fmt.Fprintf(os.Stderr, "wc: unrecognized option '%s'\n", arg)
		return 1
	default:
		opts.files = append(opts.files, arg)
	}
	return -1
}

// isShortFlag returns true if arg looks like a short flag cluster.
func isShortFlag(arg string) bool {
	return strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-'
}

// parseShortFlags handles combined short flags like -lwcL.
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
		case 'L':
			fl.maxLineLen = true
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
the following order: newline, word, character, byte, maximum line length.
  -c, --bytes            print the byte counts
  -m, --chars            print the character counts
  -l, --lines            print the newline counts
  -L, --max-line-length  print the maximum display width
  -w, --words            print the word counts
      --files0-from=F    read input from the files specified by
                           NUL-terminated names in file F;
                           If F is - then read names from standard input
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
