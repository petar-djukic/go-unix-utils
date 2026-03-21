// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd055-tail R1.1–R1.4: line-count mode (default and -n),
// stdin reading, and +NUM offset mode.
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
	progName     = "tail"
	defaultLines = 10
)

// tailOpts holds parsed command-line options.
type tailOpts struct {
	count   int
	fromTop bool // true when +NUM: start from line NUM
	files   []string
	quiet   bool // suppress headers
	verbose bool // force headers
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
// R1.3: +NUM starts from line NUM. Otherwise, prints last N lines.
func tailContent(r io.Reader, opts tailOpts, w *bufio.Writer) error {
	if opts.fromTop {
		return tailLinesFromTop(r, opts.count, w)
	}
	return tailLinesFromEnd(r, opts.count, w)
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
		return parseCountValue(arg[len("--lines="):], opts, stderr)
	case arg == "--lines":
		return parseCountNextArg(args, i, 'n', opts, stderr)
	case strings.HasPrefix(arg, "-n"):
		return parseShortCount(args, i, arg, opts, stderr)
	default:
		return applyShortFlags(arg, opts, stderr)
	}
}

// parseShortCount handles -n NUM and -nNUM forms.
func parseShortCount(args []string, i int, arg string, opts *tailOpts, stderr io.Writer) (int, int) {
	if len(arg) > 2 {
		return parseCountValue(arg[2:], opts, stderr)
	}
	return parseCountNextArg(args, i, 'n', opts, stderr)
}

// parseCountValue parses a count value and sets opts.
// R1.2: -n NUM for last NUM lines.
// R1.3: -n +NUM to start from line NUM.
func parseCountValue(val string, opts *tailOpts, stderr io.Writer) (int, int) {
	n, fromTop, err := parseNum(val)
	if err != nil {
		fmt.Fprintf(stderr, "%s: invalid number of lines: '%s'\n", progName, val)
		return 0, 1
	}
	opts.count = n
	opts.fromTop = fromTop
	return 0, -1
}

// parseCountNextArg reads the count from the next argument.
func parseCountNextArg(args []string, i int, flag byte, opts *tailOpts, stderr io.Writer) (int, int) {
	if i+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option requires an argument -- '%c'\n", progName, flag)
		printTryHelp(stderr)
		return 0, 1
	}
	consumed, code := parseCountValue(args[i+1], opts, stderr)
	return consumed + 1, code
}

// parseNum parses an integer with optional + prefix.
// R1.3: +NUM means start from line NUM (fromTop=true).
func parseNum(val string) (int, bool, error) {
	fromTop := false
	s := val
	if strings.HasPrefix(s, "+") {
		fromTop = true
		s = s[1:]
	}
	if len(s) == 0 {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false, fmt.Errorf("invalid number: '%s'", val)
	}
	return n, fromTop, nil
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
