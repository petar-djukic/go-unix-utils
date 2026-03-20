// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R1.1: lexicographic sort by byte values.
// Implements prd053-sort R1.2: stdin reading when no files or "-".
// Implements prd053-sort R1.3: multiple input files as single stream.
// Implements prd053-sort R1.4: -r/--reverse flag.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "sort"

// options holds parsed sort flags.
type options struct {
	reverse bool // -r, --reverse (R1.4)
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and processes files, returning the exit code.
// R1.1: sorts lines lexicographically when no flags given.
// R1.2: reads stdin when no file args or "-".
// R1.3: combines lines from multiple files.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	lines, readErr := readAllFiles(files, stdin, stderr)
	if readErr != 0 {
		// Continue with whatever lines were read.
	}
	sortLines(lines, opts)
	writeErr := writeLines(lines, stdout)
	if writeErr != 0 {
		return writeErr
	}
	return readErr
}

// parseArgs separates flags from file arguments.
// Returns parsed options, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (options, []string, int) {
	var opts options
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "-" {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			code := applyLongFlag(&opts, arg, stdout, stderr)
			if code >= 0 {
				return opts, nil, code
			}
			continue
		}
		for j := 1; j < len(arg); j++ {
			if !applyShortFlag(&opts, arg[j]) {
				fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
				printTryHelp(stderr)
				return opts, nil, 2
			}
		}
	}
	return opts, files, -1
}

// applyShortFlag applies a single-character flag to options.
// Returns false for unrecognized flags.
func applyShortFlag(o *options, ch byte) bool {
	switch ch {
	case 'r':
		o.reverse = true
	default:
		return false
	}
	return true
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(o *options, arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--reverse":
		o.reverse = true
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 2
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
	fmt.Fprintln(w, "Write sorted concatenation of all FILE(s) to standard output.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -r, --reverse            reverse the result of comparisons")
	fmt.Fprintln(w, "      --help               display this help and exit")
	fmt.Fprintln(w, "      --version            output version information and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// readAllFiles reads lines from all named files into a single slice.
// R1.2: "-" reads from stdin. R1.3: multiple files combined.
// Returns the collected lines and a non-zero exit code on read error.
func readAllFiles(files []string, stdin io.Reader, stderr io.Writer) ([]string, int) {
	var lines []string
	exitCode := 0
	for _, name := range files {
		fileLines, err := readOneFile(name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 2
			continue
		}
		lines = append(lines, fileLines...)
	}
	return lines, exitCode
}

// readOneFile reads all lines from a single file or stdin.
func readOneFile(name string, stdin io.Reader) ([]string, error) {
	if name == "-" {
		return readLines(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // best-effort close on read-only file
	return readLines(f)
}

// readLines reads all lines from a reader, preserving trailing newlines.
func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// sortLines sorts the lines according to the active options.
// R1.1: lexicographic byte-value sort. R1.4: reverse with -r.
func sortLines(lines []string, opts options) {
	if opts.reverse {
		sort.SliceStable(lines, func(i, j int) bool {
			return lines[i] > lines[j]
		})
		return
	}
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i] < lines[j]
	})
}

// writeLines writes all sorted lines to stdout, each followed by a newline.
func writeLines(lines []string, stdout io.Writer) int {
	w := bufio.NewWriter(stdout)
	for _, line := range lines {
		if _, err := w.WriteString(line); err != nil {
			return 2
		}
		if err := w.WriteByte('\n'); err != nil {
			return 2
		}
	}
	if err := w.Flush(); err != nil {
		return 2
	}
	return 0
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

