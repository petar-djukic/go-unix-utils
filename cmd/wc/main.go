// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd005-wc: Count Lines, Words, and Bytes.
// Covers R1.1-R1.4, R2.1-R2.6, R3.1-R3.3, R4.1-R4.4, R5.1-R5.2, R6.1-R6.3.
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

// stdinImplicit is a sentinel filename for implicit stdin (no file args).
const stdinImplicit = ""

// totalMode controls when the total line is printed.
// R3.3: --total=auto|always|only|never.
type totalMode int

const (
	totalAuto   totalMode = iota // print total when >1 file
	totalAlways                  // always print total
	totalOnly                    // print only the total line
	totalNever                   // never print total
)

// columnWidths holds per-column widths for output formatting.
// When uniform is true, all columns use the single width w.
// When uniform is false, each column uses its individual width.
type columnWidths struct {
	uniform bool
	w       int // uniform width (when uniform is true)
	lines   int
	words   int
	chars   int
	bytesW  int
	maxLL   int
}

// counts holds computed counts for a single input.
type counts struct {
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
}

// fileResult holds the result of counting one file.
type fileResult struct {
	name string
	c    counts
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
	total      totalMode
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
// R6.3: uses a buffered writer to detect stdout write errors.
func run(opts options) int {
	files, err := resolveFiles(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wc: %v\n", err)
		return 1
	}
	useStatWidth := canStatWidth(opts)
	if len(files) == 0 {
		// R1.2: no file args means implicit stdin (no name in output).
		files = []string{stdinImplicit}
	}
	eff := opts.flags.effective()
	w := bufio.NewWriter(os.Stdout)
	exitCode := processFiles(files, eff, opts.total, useStatWidth, w)
	// R6.3: exit 1 on stdout write error.
	if err := w.Flush(); err != nil {
		return 1
	}
	return exitCode
}

// canStatWidth returns true when width can be pre-computed from file sizes.
// When --files0-from=- is used, filenames come from stdin so we cannot
// stat them before counting. --total=only outputs a single line so
// pre-stat width is also skipped to match GNU wc behavior.
func canStatWidth(opts options) bool {
	if opts.files0From == "-" {
		return false
	}
	if opts.total == totalOnly {
		return false
	}
	return true
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

// processFiles counts all files, then prints results.
// R3.3: total mode controls when total line appears.
// R6.1/R6.2: exit 0 on success, exit 1 if any file fails.
func processFiles(
	files []string, fl showFlags, tm totalMode,
	useStatWidth bool, w io.Writer,
) int {
	results, total, exitCode := countAllFiles(files)
	cw := selectWidth(files, results, total, useStatWidth)
	printResults(w, results, total, fl, cw, tm, len(files))
	return exitCode
}

// countAllFiles counts every file and returns per-file results and total.
func countAllFiles(files []string) ([]fileResult, counts, int) {
	results := make([]fileResult, 0, len(files))
	var total counts
	exitCode := 0

	for _, name := range files {
		c, err := countFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wc: %v\n", err)
			exitCode = 1
			continue
		}
		addCounts(&total, c)
		results = append(results, fileResult{name: name, c: c})
	}
	return results, total, exitCode
}

// selectWidth chooses column widths based on stat or count heuristic.
func selectWidth(
	files []string, results []fileResult, total counts,
	useStatWidth bool,
) columnWidths {
	if useStatWidth {
		return uniformWidth(computeStatWidth(files))
	}
	return computeCountWidths(results, total)
}

// uniformWidth creates a columnWidths where all columns share one width.
func uniformWidth(w int) columnWidths {
	return columnWidths{uniform: true, w: w}
}

// computeStatWidth determines width from file sizes (pre-counting).
func computeStatWidth(files []string) int {
	var totalSize int64
	anyRegular := false

	for _, name := range files {
		if name == "-" || name == stdinImplicit {
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

// computeCountWidths determines per-column widths from actual counts.
func computeCountWidths(results []fileResult, total counts) columnWidths {
	maxC := total
	for _, r := range results {
		maxC = maxCounts(maxC, r.c)
	}
	return columnWidths{
		lines:  maxDigits(maxC.lines),
		words:  maxDigits(maxC.words),
		chars:  maxDigits(maxC.chars),
		bytesW: maxDigits(maxC.bytes),
		maxLL:  maxDigits(maxC.maxLineLen),
	}
}

// maxCounts returns element-wise maximum of two counts.
func maxCounts(a, b counts) counts {
	return counts{
		lines:      maxInt64(a.lines, b.lines),
		words:      maxInt64(a.words, b.words),
		bytes:      maxInt64(a.bytes, b.bytes),
		chars:      maxInt64(a.chars, b.chars),
		maxLineLen: maxInt64(a.maxLineLen, b.maxLineLen),
	}
}

// maxInt64 returns the larger of two int64 values.
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// maxDigits returns the digit count for a value, minimum 1.
func maxDigits(v int64) int {
	d := digitCount(v)
	if d < 1 {
		return 1
	}
	return d
}

// widthFor returns the column width for a given field.
func (cw columnWidths) widthFor(field int) int {
	if cw.uniform {
		return cw.w
	}
	switch field {
	case fieldLines:
		return cw.lines
	case fieldWords:
		return cw.words
	case fieldChars:
		return cw.chars
	case fieldBytes:
		return cw.bytesW
	case fieldMaxLL:
		return cw.maxLL
	default:
		return 1
	}
}

// field index constants for widthFor.
const (
	fieldLines = 0
	fieldWords = 1
	fieldChars = 2
	fieldBytes = 3
	fieldMaxLL = 4
)

// printResults outputs per-file lines and/or the total line.
func printResults(
	w io.Writer, results []fileResult, total counts,
	fl showFlags, cw columnWidths, tm totalMode, nfiles int,
) {
	if tm != totalOnly {
		for _, r := range results {
			printLine(w, r.c, displayName(r.name), cw, fl)
		}
	}
	if shouldShowTotal(tm, nfiles) {
		printLine(w, total, totalLabel(tm), cw, fl)
	}
}

// totalLabel returns the label for the total line.
// R3.3: --total=only omits the "total" label.
func totalLabel(tm totalMode) string {
	if tm == totalOnly {
		return ""
	}
	return "total"
}

// shouldShowTotal determines whether to print the total line.
// R3.3: auto=multi-file, always=always, only=always, never=never.
func shouldShowTotal(tm totalMode, nfiles int) bool {
	switch tm {
	case totalAlways, totalOnly:
		return true
	case totalNever:
		return false
	default:
		return nfiles > 1
	}
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
// R5.2: under LC_ALL=C, each byte is one char, so -m and -c agree.
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
// R6.3: writes to w; errors are detected on flush.
func printLine(
	w io.Writer, c counts, name string,
	cw columnWidths, fl showFlags,
) {
	first := true
	if fl.lines {
		printField(w, c.lines, cw.widthFor(fieldLines), &first)
	}
	if fl.words {
		printField(w, c.words, cw.widthFor(fieldWords), &first)
	}
	if fl.chars {
		printField(w, c.chars, cw.widthFor(fieldChars), &first)
	}
	if fl.bytes {
		printField(w, c.bytes, cw.widthFor(fieldBytes), &first)
	}
	if fl.maxLineLen {
		printField(w, c.maxLineLen, cw.widthFor(fieldMaxLL), &first)
	}
	if name != "" {
		fmt.Fprintf(w, " %s", name)
	}
	fmt.Fprintln(w)
}

// printField prints a single right-aligned count field.
// R6.3: writes to w; errors propagate through buffered writer.
func printField(w io.Writer, val int64, width int, first *bool) {
	if *first {
		fmt.Fprintf(w, "%*d", width, val)
		*first = false
	} else {
		fmt.Fprintf(w, " %*d", width, val)
	}
}

// displayName returns the display name for output.
// R4.1: "-" is displayed as "-" when explicitly given.
// R1.3: implicit stdin (no file args) has no filename.
func displayName(name string) string {
	if name == stdinImplicit {
		return ""
	}
	return name
}

// openInput opens a named file or returns stdin for "-" and implicit stdin.
// R4.1: "-" means stdin.
func openInput(name string) (*os.File, error) {
	if name == "-" || name == stdinImplicit {
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
	case strings.HasPrefix(arg, "--total="):
		return parseTotalMode(arg[len("--total="):], opts)
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

// parseTotalMode parses --total=MODE and sets options.total.
// R3.3: valid modes are auto, always, only, never.
func parseTotalMode(mode string, opts *options) int {
	switch mode {
	case "auto":
		opts.total = totalAuto
	case "always":
		opts.total = totalAlways
	case "only":
		opts.total = totalOnly
	case "never":
		opts.total = totalNever
	default:
		fmt.Fprintf(os.Stderr,
			"wc: invalid argument '%s' for '--total'\n", mode)
		return 1
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
      --total=WHEN       when to print a line with total counts;
                           WHEN can be: auto, always, only, never
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
