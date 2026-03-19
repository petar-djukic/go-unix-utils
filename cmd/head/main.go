// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd018-head R1.1–R1.5, R2.1–R2.3, R3.1–R3.5, R4.1–R4.2:
// line-count mode, byte-count mode with suffix parsing, multi-file headers,
// header controls, error handling, and exit code behavior.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName     = "head"
	defaultLines = 10
)

// countMode selects between line-counting and byte-counting.
type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

// headOpts holds parsed command-line options.
type headOpts struct {
	count    int
	negative bool // true when count means "all except last N"
	mode     countMode
	files    []string
	quiet    bool // R3.3: suppress headers
	verbose  bool // R3.4: force headers
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the first N lines/bytes of each file.
// Returns 0 on success, 1 on any error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(opts.files) == 0 {
		opts.files = []string{"-"}
	}
	return processFiles(opts, stdin, stdout, stderr)
}

// processFiles iterates over each file and prints content.
// R3.1: prints headers when multiple files are given.
// R3.5: skips header for files that cannot be opened and continues.
// R4.2: sets exit code 1 on any error but processes remaining files.
func processFiles(opts headOpts, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	exitCode := 0
	showHeaders := shouldShowHeaders(opts)
	hadOutput := false

	for _, name := range opts.files {
		err := processOneFile(name, opts, stdin, w, showHeaders, hadOutput)
		if err != nil {
			flushAndReport(w, stderr, err)
			exitCode = 1
			continue
		}
		hadOutput = true
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processOneFile opens a single file, prints its header if needed, and
// writes the head output. Returns nil on success or a formatted error.
func processOneFile(name string, opts headOpts, stdin io.Reader, w *bufio.Writer, showHeaders, hadOutput bool) error {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return err
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
	if err := headContent(r, opts, w); err != nil {
		return formatReadError(name, err)
	}
	return nil
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
// R3.1: headers shown for multiple files by default.
// R3.2: no headers for a single file by default.
// R3.3: -q suppresses headers. R3.4: -v forces headers.
func shouldShowHeaders(opts headOpts) bool {
	if opts.quiet {
		return false
	}
	if opts.verbose {
		return true
	}
	return len(opts.files) > 1
}

// openInput returns a reader for the named file or stdin.
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

// headContent dispatches to line or byte mode.
func headContent(r io.Reader, opts headOpts, w *bufio.Writer) error {
	if opts.mode == modeBytes {
		if opts.negative {
			return headBytesNegative(r, opts.count, w)
		}
		return headBytes(r, opts.count, w)
	}
	if opts.negative {
		return headLinesNegative(r, opts.count, w)
	}
	return headLines(r, opts.count, w)
}

// headLines reads up to lineCount lines from r.
// R1.1: default is 10 lines. R1.2: -n overrides.
// R1.5: a line is terminated by newline; unterminated final line counts.
func headLines(r io.Reader, lineCount int, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	printed := 0

	for printed < lineCount {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
			printed++
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// headLinesNegative prints all lines except the last count lines.
// R1.3: -n -NUM prints all except last NUM lines.
func headLinesNegative(r io.Reader, count int, w *bufio.Writer) error {
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
	end := len(lines) - count
	if end < 0 {
		end = 0
	}
	for _, line := range lines[:end] {
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

// headBytes reads up to byteCount bytes from r.
// R2.1: -c NUM prints first NUM bytes.
func headBytes(r io.Reader, byteCount int, w *bufio.Writer) error {
	_, err := io.CopyN(w, r, int64(byteCount))
	if err == io.EOF {
		return nil
	}
	return err
}

// headBytesNegative prints all bytes except the last count bytes.
// R2.2: -c -NUM prints all except last NUM bytes.
func headBytesNegative(r io.Reader, count int, w *bufio.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	end := len(data) - count
	if end < 0 {
		end = 0
	}
	_, werr := w.Write(data[:end])
	return werr
}

// parseArgs separates flags from file arguments.
// Returns opts and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (headOpts, int) {
	opts := headOpts{count: defaultLines, mode: modeLines}
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
		consumed, code := applyFlag(args, i, &opts, stdout, stderr)
		if code >= 0 {
			return opts, code
		}
		i += consumed
	}
	return opts, -1
}

// applyFlag handles a single flag argument.
// Returns (extra args consumed, exit code). Exit code -1 means continue.
func applyFlag(args []string, i int, opts *headOpts, stdout, stderr io.Writer) (int, int) {
	arg := args[i]
	switch {
	case arg == "--help":
		printHelp(stdout)
		return 0, 0
	case arg == "--version":
		printVersion(stdout)
		return 0, 0
	case arg == "--quiet", arg == "--silent":
		opts.quiet = true
		return 0, -1
	case arg == "--verbose":
		opts.verbose = true
		return 0, -1
	case strings.HasPrefix(arg, "--lines="):
		return parseCountValue(arg[len("--lines="):], modeLines, opts, stderr)
	case arg == "--lines":
		return parseCountNextArg(args, i, 'n', modeLines, opts, stderr)
	case strings.HasPrefix(arg, "--bytes="):
		return parseCountValue(arg[len("--bytes="):], modeBytes, opts, stderr)
	case arg == "--bytes":
		return parseCountNextArg(args, i, 'c', modeBytes, opts, stderr)
	case strings.HasPrefix(arg, "-n"):
		return parseShortCount(args, i, arg, 'n', modeLines, opts, stderr)
	case strings.HasPrefix(arg, "-c"):
		return parseShortCount(args, i, arg, 'c', modeBytes, opts, stderr)
	default:
		return applyShortFlags(arg, opts, stderr)
	}
}

// parseShortCount handles -n NUM, -nNUM, -c NUM, -cNUM forms.
func parseShortCount(args []string, i int, arg string, flag byte, mode countMode, opts *headOpts, stderr io.Writer) (int, int) {
	if len(arg) > 2 {
		return parseCountValue(arg[2:], mode, opts, stderr)
	}
	return parseCountNextArg(args, i, flag, mode, opts, stderr)
}

// parseCountValue parses a count value with optional suffix and sets opts.
// R2.1: -c and -n are mutually exclusive; last one given takes precedence.
func parseCountValue(val string, mode countMode, opts *headOpts, stderr io.Writer) (int, int) {
	n, negative, err := parseNumWithSuffix(val, mode)
	if err != nil {
		label := "lines"
		if mode == modeBytes {
			label = "bytes"
		}
		fmt.Fprintf(stderr, "%s: invalid number of %s: '%s'\n", progName, label, val)
		return 0, 1
	}
	opts.mode = mode
	opts.count = n
	opts.negative = negative
	return 0, -1
}

// parseCountNextArg reads the count from the next argument.
func parseCountNextArg(args []string, i int, flag byte, mode countMode, opts *headOpts, stderr io.Writer) (int, int) {
	if i+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option requires an argument -- '%c'\n", progName, flag)
		printTryHelp(stderr)
		return 0, 1
	}
	consumed, code := parseCountValue(args[i+1], mode, opts, stderr)
	return consumed + 1, code
}

// parseNumWithSuffix parses an integer with optional negative prefix and
// byte-count suffix. For line mode, no suffix is allowed.
func parseNumWithSuffix(val string, mode countMode) (int, bool, error) {
	negative := false
	s := val
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	}
	if len(s) == 0 {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	numEnd := findNumEnd(s)
	if numEnd == 0 {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	n, err := strconv.Atoi(s[:numEnd])
	if err != nil {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	suffix := s[numEnd:]
	if mode == modeLines && suffix != "" {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	multiplier, err := suffixMultiplier(suffix, val)
	if err != nil {
		return 0, false, err
	}
	return n * multiplier, negative, nil
}

// findNumEnd returns the index where digits end in s.
func findNumEnd(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// suffixMultiplier returns the byte multiplier for a suffix string.
// R2.3: supports b (512), K/KiB (1024), M/MiB, G/GiB, T/TiB, P/PiB, E/EiB.
// TODO: R2.3 task requests KB, MB, GB (powers of 1000) but prd018-head
// non_goals excludes decimal multiplier suffixes.
func suffixMultiplier(suffix, original string) (int, error) {
	switch suffix {
	case "":
		return 1, nil
	case "b":
		return 512, nil
	case "K", "KiB":
		return 1024, nil
	case "M", "MiB":
		return 1024 * 1024, nil
	case "G", "GiB":
		return 1024 * 1024 * 1024, nil
	case "T", "TiB":
		return 1024 * 1024 * 1024 * 1024, nil
	case "P", "PiB":
		return 1024 * 1024 * 1024 * 1024 * 1024, nil
	case "E", "EiB":
		return 1024 * 1024 * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("invalid number of bytes: '%s'", original)
	}
}

// applyShortFlags processes short flag clusters like -q, -v, or -qv.
// Returns (extra args consumed, exit code).
func applyShortFlags(arg string, opts *headOpts, stderr io.Writer) (int, int) {
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
// R3.5: error message matches GNU head format with capitalized system error.
func formatError(name string, err error) error {
	return fmt.Errorf("cannot open '%s' for reading: %s", name, sysErrMsg(err))
}

// formatReadError formats a read error for GNU-compatible output.
// R3.5: used when a file opens successfully but reading fails (e.g., directory).
func formatReadError(name string, err error) error {
	displayName := name
	if name == "-" {
		displayName = "standard input"
	}
	return fmt.Errorf("error reading '%s': %s", displayName, sysErrMsg(err))
}

// sysErrMsg extracts and capitalizes the system error message to match GNU format.
// Go returns lowercase syscall errors; GNU coreutils uses capitalized messages.
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

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Print the first 10 lines of each FILE to standard output.")
	fmt.Fprintln(w, "With more than one FILE, precede each with a header giving the file name.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -c, --bytes=NUM      print the first NUM bytes of each file")
	fmt.Fprintln(w, "  -n, --lines=NUM      print the first NUM lines instead of the first 10")
	fmt.Fprintln(w, "  -q, --quiet, --silent never print headers giving file names")
	fmt.Fprintln(w, "  -v, --verbose        always print headers giving file names")
	fmt.Fprintln(w, "      --help           display this help and exit")
	fmt.Fprintln(w, "      --version        output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
