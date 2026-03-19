// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd018-head R1.1–R1.4: core line-count mode with default 10 lines,
// explicit -n count, multi-file headers, and stdin reading.
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

// headOpts holds parsed command-line options.
type headOpts struct {
	lineCount int
	files     []string
	quiet     bool // R3.3: suppress headers
	verbose   bool // R3.4: force headers
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the first N lines of each file.
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

// processFiles iterates over each file and prints the first N lines.
// R1.3/R3.1: prints headers when multiple files are given.
func processFiles(opts headOpts, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	exitCode := 0
	showHeaders := shouldShowHeaders(opts)

	for i, name := range opts.files {
		if showHeaders {
			if i > 0 {
				fmt.Fprint(w, "\n")
			}
			displayName := name
			if name == "-" {
				displayName = "standard input"
			}
			fmt.Fprintf(w, "==> %s <==\n", displayName)
		}
		if err := headOne(name, opts.lineCount, stdin, w); err != nil {
			// flush before writing to stderr so output order is correct
			w.Flush() // best-effort flush
			fmt.Fprintf(stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
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

// headOne prints the first lineCount lines from a file or stdin.
func headOne(name string, lineCount int, stdin io.Reader, w *bufio.Writer) error {
	if name == "-" {
		return headReader(stdin, lineCount, w)
	}
	f, err := os.Open(name)
	if err != nil {
		return formatError(name, err)
	}
	defer f.Close() // best-effort close on read-only file
	return headReader(f, lineCount, w)
}

// headReader reads up to lineCount lines from r and writes them to w.
// R1.1: default is 10 lines. R1.2: -n overrides.
// R1.5: a line is terminated by newline; unterminated final line counts.
func headReader(r io.Reader, lineCount int, w *bufio.Writer) error {
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

// parseArgs separates flags from file arguments.
// Returns opts and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (headOpts, int) {
	opts := headOpts{lineCount: defaultLines}
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
		return parseLinesValue(arg[len("--lines="):], opts, stderr)
	case arg == "--lines":
		return parseLinesNextArg(args, i, opts, stderr)
	case strings.HasPrefix(arg, "-n"):
		return parseShortN(args, i, arg, opts, stderr)
	default:
		return applyShortFlags(arg, opts, stderr)
	}
}

// parseShortN handles -n NUM or -nNUM forms.
func parseShortN(args []string, i int, arg string, opts *headOpts, stderr io.Writer) (int, int) {
	if len(arg) > 2 {
		// -nNUM form
		return parseLinesValue(arg[2:], opts, stderr)
	}
	// -n NUM form
	return parseLinesNextArg(args, i, opts, stderr)
}

// parseLinesValue parses a line count value string and sets opts.lineCount.
func parseLinesValue(val string, opts *headOpts, stderr io.Writer) (int, int) {
	n, err := strconv.Atoi(val)
	if err != nil {
		fmt.Fprintf(stderr, "%s: invalid number of lines: '%s'\n", progName, val)
		return 0, 1
	}
	opts.lineCount = n
	return 0, -1
}

// parseLinesNextArg reads the line count from the next argument.
func parseLinesNextArg(args []string, i int, opts *headOpts, stderr io.Writer) (int, int) {
	if i+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option requires an argument -- 'n'\n", progName)
		printTryHelp(stderr)
		return 0, 1
	}
	consumed, code := parseLinesValue(args[i+1], opts, stderr)
	return consumed + 1, code
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
func formatError(name string, err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Errorf("cannot open '%s' for reading: %s", name, pe.Err)
	}
	return fmt.Errorf("cannot open '%s' for reading: %s", name, err)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Print the first 10 lines of each FILE to standard output.")
	fmt.Fprintln(w, "With more than one FILE, precede each with a header giving the file name.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w, "")
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
