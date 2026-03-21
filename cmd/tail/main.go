// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail R1.1–R1.4: line-count mode (default and -n),
// stdin reading, and +NUM offset mode.
// Implements prd055-tail R2.1–R2.3: byte-count mode (-c) with suffix multipliers.
// Implements prd055-tail R3.1–R3.4: multi-file headers, -q/--quiet/--silent, -v/--verbose.
// Implements prd055-tail R4.1–R4.4: exit codes, error reporting, and continued processing.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName     = "tail"
	defaultLines = 10
)

// tailOpts holds parsed command-line options.
type tailOpts struct {
	count    int64
	fromTop  bool // true when +NUM: start from line/byte NUM
	byteMode bool // R2.1: true when -c is used instead of -n
	files    []string
	quiet    bool // suppress headers
	verbose  bool // force headers
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the last N lines of each file.
// Returns 0 on success, 1 on any error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, code := parseArgs(args, stderr)
	if code >= 0 {
		return code
	}
	if len(opts.files) == 0 {
		opts.files = []string{"-"}
	}
	return processFiles(opts, stdin, stdout, stderr)
}

// processFiles iterates over each file and prints tail content.
// R1.1: prints the last 10 lines by default.
func processFiles(opts tailOpts, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	exitCode := 0
	showHeaders := shouldShowHeaders(opts)
	hadOutput := false

	for _, name := range opts.files {
		produced, err := processOneFile(name, opts, stdin, w, showHeaders, hadOutput)
		if produced {
			hadOutput = true
		}
		if err != nil {
			flushAndReport(w, stderr, err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processOneFile opens a single file, prints its header if needed,
// and writes the tail output. Returns (outputProduced, error).
func processOneFile(name string, opts tailOpts, stdin io.Reader, w *bufio.Writer, showHeaders, hadOutput bool) (bool, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return false, err
	}
	if closer != nil {
		defer closer.Close() // best-effort close on read-only file
	}
	if showHeaders {
		if hadOutput {
			fmt.Fprint(w, "\n")
		}
		printHeader(w, name)
	}
	if err := tailContent(r, opts, w); err != nil {
		return true, formatReadError(name, err)
	}
	return true, nil
}

// flushAndReport flushes buffered output then writes the error to stderr.
func flushAndReport(w *bufio.Writer, stderr io.Writer, err error) {
	w.Flush() // best-effort flush before writing to stderr
	fmt.Fprintf(stderr, "%s: %s\n", progName, err)
}

// printHeader writes the GNU-format file header.
func printHeader(w *bufio.Writer, name string) {
	displayName := name
	if name == "-" {
		displayName = "standard input"
	}
	fmt.Fprintf(w, "==> %s <==\n", displayName)
}

// shouldShowHeaders determines whether to print file headers.
func shouldShowHeaders(opts tailOpts) bool {
	if opts.quiet {
		return false
	}
	if opts.verbose {
		return true
	}
	return len(opts.files) > 1
}

// openInput returns a reader for the named file or stdin.
// R1.4: reads from stdin when name is "-".
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, formatError(name, err)
	}
	return f, f, nil
}

// tailContent dispatches to the appropriate tail mode.
// R1.3: +NUM starts from line NUM. R2.1: byteMode uses bytes.
func tailContent(r io.Reader, opts tailOpts, w *bufio.Writer) error {
	if opts.byteMode {
		return tailBytes(r, opts, w)
	}
	return tailLines(r, opts, w)
}

// tailLines dispatches line-based tail.
func tailLines(r io.Reader, opts tailOpts, w *bufio.Writer) error {
	if opts.fromTop {
		return tailLinesFromTop(r, int(opts.count), w)
	}
	return tailLinesFromEnd(r, int(opts.count), w)
}

// tailBytes dispatches byte-based tail.
// R2.1: -c NUM prints last NUM bytes. R2.2: -c +NUM from byte NUM.
func tailBytes(r io.Reader, opts tailOpts, w *bufio.Writer) error {
	if opts.fromTop {
		return tailBytesFromTop(r, opts.count, w)
	}
	return tailBytesFromEnd(r, opts.count, w)
}

// tailLinesFromEnd reads all lines and prints the last count lines.
// R1.1: default is 10 lines. R1.2: -n overrides.
func tailLinesFromEnd(r io.Reader, count int, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return writeLastLines(lines, count, w)
}

// writeLastLines writes the last count lines from the collected lines.
func writeLastLines(lines [][]byte, count int, w *bufio.Writer) error {
	start := len(lines) - count
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

// tailLinesFromTop prints starting from line number startLine (1-indexed).
// R1.3: -n +5 prints from line 5 onward.
func tailLinesFromTop(r io.Reader, startLine int, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	lineNum := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lineNum++
			if lineNum >= startLine {
				if _, werr := w.Write(line); werr != nil {
					return werr
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// tailBytesFromEnd reads all input and prints the last count bytes.
// R2.1: -c NUM prints the last NUM bytes.
func tailBytesFromEnd(r io.Reader, count int64, w *bufio.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	start := int64(len(data)) - count
	if start < 0 {
		start = 0
	}
	_, werr := w.Write(data[start:])
	return werr
}

// tailBytesFromTop skips the first (startByte-1) bytes and prints the rest.
// R2.2: -c +100 prints from byte 100 onward (1-indexed).
func tailBytesFromTop(r io.Reader, startByte int64, w *bufio.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	skip := startByte - 1
	if skip < 0 {
		skip = 0
	}
	if skip >= int64(len(data)) {
		return nil
	}
	_, werr := w.Write(data[skip:])
	return werr
}

// parseArgs separates flags from file arguments.
// Returns opts and exit code (-1 = continue).
func parseArgs(args []string, stderr io.Writer) (tailOpts, int) {
	opts := tailOpts{count: defaultLines}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			opts.files = append(opts.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			opts.files = append(opts.files, arg)
			continue
		}
		consumed, code := applyFlag(args, i, &opts, stderr)
		if code >= 0 {
			return opts, code
		}
		i += consumed
	}
	return opts, -1
}

// applyFlag handles a single flag argument.
// Returns (extra args consumed, exit code). Exit code -1 means continue.
func applyFlag(args []string, i int, opts *tailOpts, stderr io.Writer) (int, int) {
	arg := args[i]
	switch {
	case arg == "--quiet", arg == "--silent":
		opts.quiet = true
		return 0, -1
	case arg == "--verbose":
		opts.verbose = true
		return 0, -1
	case strings.HasPrefix(arg, "--lines="):
		return parseLineCountValue(arg[len("--lines="):], opts, stderr)
	case arg == "--lines":
		return parseNextArg(args, i, 'n', false, opts, stderr)
	case strings.HasPrefix(arg, "--bytes="):
		return parseByteCountValue(arg[len("--bytes="):], opts, stderr)
	case arg == "--bytes":
		return parseNextArg(args, i, 'c', true, opts, stderr)
	case strings.HasPrefix(arg, "-n"):
		return parseShortCount(args, i, arg, false, opts, stderr)
	case strings.HasPrefix(arg, "-c"):
		return parseShortCount(args, i, arg, true, opts, stderr)
	default:
		return applyShortFlags(arg, opts, stderr)
	}
}

// parseShortCount handles -n NUM, -nNUM, -c NUM, -cNUM forms.
func parseShortCount(args []string, i int, arg string, byteMode bool, opts *tailOpts, stderr io.Writer) (int, int) {
	flag := byte('n')
	if byteMode {
		flag = 'c'
	}
	if len(arg) > 2 {
		return parseCountValue(arg[2:], byteMode, opts, stderr)
	}
	return parseNextArg(args, i, flag, byteMode, opts, stderr)
}

// parseLineCountValue parses a line count value and sets opts.
func parseLineCountValue(val string, opts *tailOpts, stderr io.Writer) (int, int) {
	return parseCountValue(val, false, opts, stderr)
}

// parseByteCountValue parses a byte count value and sets opts.
func parseByteCountValue(val string, opts *tailOpts, stderr io.Writer) (int, int) {
	return parseCountValue(val, true, opts, stderr)
}

// parseCountValue parses a count value with optional +prefix and suffixes.
// R2.1: -c and -n set byteMode accordingly. R2.3: suffix multipliers.
func parseCountValue(val string, byteMode bool, opts *tailOpts, stderr io.Writer) (int, int) {
	n, fromTop, err := parseNum(val, byteMode)
	if err != nil {
		label := "lines"
		if byteMode {
			label = "bytes"
		}
		fmt.Fprintf(stderr, "%s: invalid number of %s: '%s'\n", progName, label, val)
		return 0, 1
	}
	opts.count = n
	opts.fromTop = fromTop
	opts.byteMode = byteMode
	return 0, -1
}

// parseNextArg reads the count from the next argument.
func parseNextArg(args []string, i int, flag byte, byteMode bool, opts *tailOpts, stderr io.Writer) (int, int) {
	if i+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option requires an argument -- '%c'\n", progName, flag)
		printTryHelp(stderr)
		return 0, 1
	}
	consumed, code := parseCountValue(args[i+1], byteMode, opts, stderr)
	return consumed + 1, code
}

// parseNum parses an integer with optional + prefix and suffix multipliers.
// R1.3: +NUM means start from line/byte NUM (fromTop=true).
// R2.3: suffix multipliers (b, K, M, G, etc.).
func parseNum(val string, withSuffix bool) (int64, bool, error) {
	fromTop := false
	s := val
	if strings.HasPrefix(s, "+") {
		fromTop = true
		s = s[1:]
	}
	if len(s) == 0 {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	if withSuffix {
		return parseNumWithSuffix(s, fromTop, val)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	return n, fromTop, nil
}

// parseNumWithSuffix parses the numeric part and applies suffix multiplier.
// R2.3: b=512, kB=1000, K/KiB=1024, MB=1000000, M/MiB=1048576, etc.
func parseNumWithSuffix(s string, fromTop bool, origVal string) (int64, bool, error) {
	numEnd := findNumEnd(s)
	if numEnd == 0 {
		return 0, false, fmt.Errorf("invalid number: '%s'", origVal)
	}
	n, err := strconv.ParseInt(s[:numEnd], 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid number: '%s'", origVal)
	}
	suffix := s[numEnd:]
	if suffix == "" {
		return n, fromTop, nil
	}
	mult, ok := suffixMultiplier(suffix)
	if !ok {
		return 0, false, fmt.Errorf("invalid number: '%s'", origVal)
	}
	return n * mult, fromTop, nil
}

// findNumEnd returns the index where digits end in s.
func findNumEnd(s string) int {
	for i, ch := range s {
		if !unicode.IsDigit(ch) {
			return i
		}
	}
	return len(s)
}

// suffixMultiplier returns the multiplier for a GNU coreutils suffix.
// R2.3: b=512, kB=1000, K/KiB=1024, MB=10^6, M/MiB=2^20,
// GB=10^9, G/GiB=2^30, TB=10^12, T/TiB=2^40.
func suffixMultiplier(suffix string) (int64, bool) {
	switch suffix {
	case "b":
		return 512, true
	case "kB":
		return 1000, true
	case "K", "KiB":
		return 1024, true
	case "MB":
		return 1000 * 1000, true
	case "M", "MiB":
		return 1024 * 1024, true
	case "GB":
		return 1000 * 1000 * 1000, true
	case "G", "GiB":
		return 1024 * 1024 * 1024, true
	case "TB":
		return 1000 * 1000 * 1000 * 1000, true
	case "T", "TiB":
		return 1024 * 1024 * 1024 * 1024, true
	default:
		return 0, false
	}
}

// applyShortFlags processes short flag clusters like -q, -v.
func applyShortFlags(arg string, opts *tailOpts, stderr io.Writer) (int, int) {
	for _, ch := range arg[1:] {
		switch ch {
		case 'q':
			opts.quiet = true
		case 'v':
			opts.verbose = true
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, ch)
			printTryHelp(stderr)
			return 0, 1
		}
	}
	return 0, -1
}

// formatError formats a file open error for GNU-compatible output.
func formatError(name string, err error) error {
	return fmt.Errorf("cannot open '%s' for reading: %s", name, sysErrMsg(err))
}

// formatReadError formats a read error for GNU-compatible output.
func formatReadError(name string, err error) error {
	displayName := name
	if name == "-" {
		displayName = "standard input"
	}
	return fmt.Errorf("error reading '%s': %s", displayName, sysErrMsg(err))
}

// sysErrMsg extracts and capitalizes the system error message.
func sysErrMsg(err error) string {
	var msg string
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	} else {
		msg = err.Error()
	}
	return capitalizeFirst(msg)
}

// capitalizeFirst uppercases the first byte of s.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
