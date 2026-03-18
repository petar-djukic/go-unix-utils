// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1–R1.4: core file concatenation.
// Implements prd006-cat R2.1–R2.4, R3.1, R4.1–R4.8: flag parsing and option handling.
package main

import (
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

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and processes files, returning the exit code.
// R1.1: reads each named file in argument order.
// R1.2: reads stdin when no args or "-" is given.
// R1.3: concatenates with no separator.
// R1.4: binary-safe via io.Copy.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	_ = opts // flags parsed; behavior implemented in future tasks
	if len(files) == 0 {
		files = []string{"-"}
	}
	return catFiles(files, stdin, stdout, stderr)
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

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
