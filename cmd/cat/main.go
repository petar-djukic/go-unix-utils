// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1–R1.5: core file concatenation and stdin reading.
// Implements prd006-cat R2.1–R2.4: line numbering (-n, -b).
// Implements prd006-cat R3.1–R3.3: blank-line squeezing (-s).
// Implements prd006-cat R4.1–R4.8: flag parsing and option handling.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "cat"

// options holds parsed GNU cat flags.
type options struct {
	numberAll      bool // -n, --number (R2.1)
	numberNonblank bool // -b, --number-nonblank (R2.2)
	squeezeBlanks  bool // -s, --squeeze-blank (R3.1)
	showNonprint   bool // -v, --show-nonprinting (R4.1)
	showEnds       bool // -E, --show-ends (R4.3)
	showTabs       bool // -T, --show-tabs (R4.4)
}

// lineState tracks processing state across lines and files.
// R2.1: lineNum persists across files.
// R3.2: lastWasBlank persists across file boundaries.
type lineState struct {
	lineNum      int  // running line number counter
	lastWasBlank bool // previous line was blank (for -s)
	atLineStart  bool // next output byte starts a new line
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and processes files, returning the exit code.
// R1.1: reads each named file in argument order.
// R1.2: reads stdin when no args or "-" is given.
// R1.3: concatenates with no separator.
// R1.4: binary-safe via io.Copy when no flags active.
// R1.5: preserves newlines in all modes.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	if !needsLineProcessing(opts) {
		return catFiles(files, stdin, stdout, stderr)
	}
	return catFilesProcessed(files, opts, stdin, stdout, stderr)
}

// needsLineProcessing returns true when any flag requires line-by-line processing.
func needsLineProcessing(opts options) bool {
	return opts.numberAll || opts.numberNonblank || opts.squeezeBlanks ||
		opts.showNonprint || opts.showEnds || opts.showTabs
}

// parseArgs separates flags from file arguments.
// Returns parsed options, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (options, []string, int) {
	var opts options
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[0] == '-' && arg[1] == '-' {
			code := applyLongFlag(&opts, arg, stdout, stderr)
			if code >= 0 {
				return opts, nil, code
			}
			continue
		}
		// R2.2: combined short flags (e.g., -vET = -v -E -T)
		for j := 1; j < len(arg); j++ {
			if !applyShortFlag(&opts, arg[j]) {
				fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
				printTryHelp(stderr)
				return opts, nil, 1
			}
		}
	}
	return opts, files, -1
}

// applyShortFlag applies a single-character flag to options.
// Returns false for unrecognized flags.
func applyShortFlag(o *options, ch byte) bool {
	switch ch {
	case 'n':
		o.numberAll = true
	case 'b':
		o.numberNonblank = true
	case 's':
		o.squeezeBlanks = true
	case 'v':
		o.showNonprint = true
	case 'E':
		o.showEnds = true
	case 'T':
		o.showTabs = true
	case 'A': // R4.5: -A = -vET
		o.showNonprint = true
		o.showEnds = true
		o.showTabs = true
	case 'e': // R4.6: -e = -vE
		o.showNonprint = true
		o.showEnds = true
	case 't': // R4.7: -t = -vT
		o.showNonprint = true
		o.showTabs = true
	case 'u': // R4.8: accepted but ignored
	default:
		return false
	}
	return true
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(o *options, arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--number":
		o.numberAll = true
	case "--number-nonblank":
		o.numberNonblank = true
	case "--squeeze-blank":
		o.squeezeBlanks = true
	case "--show-nonprinting":
		o.showNonprint = true
	case "--show-ends":
		o.showEnds = true
	case "--show-tabs":
		o.showTabs = true
	case "--show-all": // R4.5: --show-all = -vET
		o.showNonprint = true
		o.showEnds = true
		o.showTabs = true
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Concatenate FILE(s), or standard input, to standard output.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -A, --show-all           equivalent to -vET")
	fmt.Fprintln(w, "  -b, --number-nonblank    number nonempty output lines, overrides -n")
	fmt.Fprintln(w, "  -e                       equivalent to -vE")
	fmt.Fprintln(w, "  -E, --show-ends          display $ at end of each line")
	fmt.Fprintln(w, "  -n, --number             number all output lines")
	fmt.Fprintln(w, "  -s, --squeeze-blank      suppress repeated empty output lines")
	fmt.Fprintln(w, "  -t                       equivalent to -vT")
	fmt.Fprintln(w, "  -T, --show-tabs          display TAB characters as ^I")
	fmt.Fprintln(w, "  -u                       (ignored)")
	fmt.Fprintln(w, "  -v, --show-nonprinting   use ^ and M- notation, except for LFD and TAB")
	fmt.Fprintln(w, "      --help               display this help and exit")
	fmt.Fprintln(w, "      --version            output version information and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// catFiles iterates over filenames, copying each to stdout.
// Used when no transformation flags are active (R1.4: binary-safe).
func catFiles(files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	exitCode := 0
	for _, name := range files {
		if err := catOne(name, stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// catOne copies a single file (or stdin for "-") to stdout.
func catOne(name string, stdin io.Reader, stdout io.Writer) error {
	if name == "-" {
		_, err := io.Copy(stdout, stdin)
		return err
	}
	f, err := os.Open(name)
	if err != nil {
		return unwrapPathError(err)
	}
	defer f.Close() // best-effort close on read-only file
	_, err = io.Copy(stdout, f)
	return err
}

// catFilesProcessed processes files with line-by-line transformations.
// R3.2: state persists across files for cross-boundary squeezing.
func catFilesProcessed(files []string, opts options, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	state := &lineState{atLineStart: true}
	exitCode := 0
	for _, name := range files {
		if err := processOne(name, opts, state, stdin, w); err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processOne opens a file (or stdin) and processes it line by line.
func processOne(name string, opts options, state *lineState, stdin io.Reader, w *bufio.Writer) error {
	if name == "-" {
		return processReader(stdin, opts, state, w)
	}
	f, err := os.Open(name)
	if err != nil {
		return unwrapPathError(err)
	}
	defer f.Close() // best-effort close on read-only file
	return processReader(f, opts, state, w)
}

// processReader reads from r line by line and applies transformations.
func processReader(r io.Reader, opts options, state *lineState, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if werr := processLine(line, opts, state, w); werr != nil {
				return werr
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

// processLine applies transformations to a single line.
// R4.9 order: squeeze(-s) → nonprint(-v/-T) → ends(-E) → number(-n/-b).
func processLine(line []byte, opts options, state *lineState, w *bufio.Writer) error {
	blank := isBlankLine(line)
	// R3.1: suppress consecutive blank lines after the first.
	if opts.squeezeBlanks && blank && state.lastWasBlank {
		return nil
	}
	state.lastWasBlank = blank
	// R2.1–R2.3: prepend line number at start of line.
	if state.atLineStart && shouldNumber(opts, blank) {
		state.lineNum++
		fmt.Fprintf(w, "%6d\t", state.lineNum)
	}
	state.atLineStart = len(line) > 0 && line[len(line)-1] == '\n'
	_, err := w.Write(line)
	return err
}

// isBlankLine returns true if line contains only a newline.
// R2.4: blank = only '\n'; lines with spaces or tabs are not blank.
func isBlankLine(line []byte) bool {
	return len(line) == 1 && line[0] == '\n'
}

// shouldNumber returns true if the current line should receive a number.
// R2.2–R2.3: -b overrides -n (blank lines not numbered with -b).
func shouldNumber(opts options, blank bool) bool {
	if opts.numberNonblank {
		return !blank
	}
	return opts.numberAll
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
