// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R1.1: lexicographic sort by byte values.
// Implements prd053-sort R1.2: stdin reading when no files or "-".
// Implements prd053-sort R1.3: multiple input files as single stream.
// Implements prd053-sort R1.4: -r/--reverse flag.
// Implements prd053-sort R1.5: -u/--unique flag.
// Implements prd053-sort R1.6: -o/--output flag.
// Implements prd053-sort R1.7: -s/--stable flag.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "sort"

// options holds parsed sort flags.
type options struct {
	reverse    bool   // -r, --reverse (R1.4)
	unique     bool   // -u, --unique (R1.5)
	outputFile string // -o, --output (R1.6)
	stable     bool   // -s, --stable (R1.7)
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and processes files, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	lines, readErr := readAllFiles(files, stdin, stderr)
	sortLines(lines, opts)
	if opts.unique {
		lines = dedup(lines)
	}
	return writeOutput(lines, opts, stdout, stderr, readErr)
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
		if strings.HasPrefix(arg, "--") {
			newI, code := applyLongFlag(&opts, arg, args, i, stdout, stderr)
			if code >= 0 {
				return opts, nil, code
			}
			i = newI
			continue
		}
		newI, code := applyShortFlags(&opts, arg, args, i, stderr)
		if code >= 0 {
			return opts, nil, code
		}
		i = newI
	}
	return opts, files, -1
}

// applyShortFlags processes a short flag group (e.g., "-rus").
// Returns the updated args index and exit code (-1 = continue).
func applyShortFlags(o *options, arg string, args []string, i int, stderr io.Writer) (int, int) {
	for j := 1; j < len(arg); j++ {
		ch := arg[j]
		if ch == 'o' {
			return consumeOutputArg(o, arg, j, args, i, stderr)
		}
		if !applyShortFlag(o, ch) {
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, ch)
			printTryHelp(stderr)
			return i, 2
		}
	}
	return i, -1
}

// consumeOutputArg handles the -o flag which requires an argument.
// The argument is either the rest of the current arg or the next arg.
func consumeOutputArg(o *options, arg string, j int, args []string, i int, stderr io.Writer) (int, int) {
	if j+1 < len(arg) {
		o.outputFile = arg[j+1:]
		return i, -1
	}
	if i+1 < len(args) {
		i++
		o.outputFile = args[i]
		return i, -1
	}
	fmt.Fprintf(stderr, "%s: option requires an argument -- 'o'\n", progName)
	printTryHelp(stderr)
	return i, 2
}

// applyShortFlag applies a single-character flag to options.
// Returns false for unrecognized flags.
func applyShortFlag(o *options, ch byte) bool {
	switch ch {
	case 'r':
		o.reverse = true
	case 'u':
		o.unique = true
	case 's':
		o.stable = true
	default:
		return false
	}
	return true
}

// applyLongFlag handles --long-name flags.
// Returns the updated args index and exit code (-1 = continue).
func applyLongFlag(o *options, arg string, args []string, i int, stdout, stderr io.Writer) (int, int) {
	switch {
	case arg == "--reverse":
		o.reverse = true
	case arg == "--unique":
		o.unique = true
	case arg == "--stable":
		o.stable = true
	case arg == "--output" && i+1 < len(args):
		i++
		o.outputFile = args[i]
	case strings.HasPrefix(arg, "--output="):
		o.outputFile = arg[len("--output="):]
	case arg == "--output":
		fmt.Fprintf(stderr, "%s: option '--output' requires an argument\n", progName)
		printTryHelp(stderr)
		return i, 2
	case arg == "--help":
		printHelp(stdout)
		return i, 0
	case arg == "--version":
		printVersion(stdout)
		return i, 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return i, 2
	}
	return i, -1
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
	fmt.Fprintln(w, "  -o, --output=FILE        write result to FILE instead of standard output")
	fmt.Fprintln(w, "  -r, --reverse            reverse the result of comparisons")
	fmt.Fprintln(w, "  -s, --stable             stabilize sort by disabling last-resort comparison")
	fmt.Fprintln(w, "  -u, --unique             with default sort, output only the first of an equal run")
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

// readLines reads all lines from a reader.
func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// sortLines sorts the lines according to the active options.
// R1.7: uses stable sort when -s is given, unstable otherwise.
func sortLines(lines []string, opts options) {
	less := buildLessFunc(lines, opts)
	if opts.stable {
		sort.SliceStable(lines, less)
	} else {
		sort.Slice(lines, less)
	}
}

// buildLessFunc returns the comparison function for sorting.
// R1.1: lexicographic byte-value sort. R1.4: reverse with -r.
func buildLessFunc(lines []string, opts options) func(i, j int) bool {
	if opts.reverse {
		return func(i, j int) bool { return lines[i] > lines[j] }
	}
	return func(i, j int) bool { return lines[i] < lines[j] }
}

// dedup removes adjacent duplicate lines from a sorted slice.
// R1.5: output only the first of an equal run.
func dedup(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	result := []string{lines[0]}
	for i := 1; i < len(lines); i++ {
		if lines[i] != lines[i-1] {
			result = append(result, lines[i])
		}
	}
	return result
}

// writeOutput writes sorted lines to the appropriate destination.
// R1.6: writes to -o file if specified, otherwise stdout.
func writeOutput(lines []string, opts options, stdout, stderr io.Writer, readErr int) int {
	if opts.outputFile != "" {
		return writeToFile(lines, opts.outputFile, stderr, readErr)
	}
	if writeErr := writeLines(lines, stdout); writeErr != 0 {
		return writeErr
	}
	return readErr
}

// writeToFile writes lines to the named file. R1.6: -o FILE.
func writeToFile(lines []string, path string, stderr io.Writer, readErr int) int {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: open failed: %s: %s\n", progName, path, unwrapPathError(err))
		return 2
	}
	writeErr := writeLines(lines, f)
	if cerr := f.Close(); cerr != nil && writeErr == 0 {
		fmt.Fprintf(stderr, "%s: write failed: %s: %s\n", progName, path, cerr)
		return 2
	}
	if writeErr != 0 {
		return writeErr
	}
	return readErr
}

// writeLines writes all lines to w, each followed by a newline.
func writeLines(lines []string, w io.Writer) int {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return 2
		}
		if err := bw.WriteByte('\n'); err != nil {
			return 2
		}
	}
	if err := bw.Flush(); err != nil {
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
