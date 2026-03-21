// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd064-shuf R1.1–R1.4: default shuffle behavior for files and stdin.
// Implements prd064-shuf R2.1–R2.4: range mode, head count, repeat, output file.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "shuf"

// options holds parsed command-line flags for shuf.
type options struct {
	inputRange [2]int // R2.1: lo-hi inclusive range
	hasRange   bool
	headCount  int // R2.2: max output lines (-1 = unlimited)
	repeat     bool
	outputFile string // R2.4: output file path (empty = stdout)
	files      []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and shuffles input lines, returning the exit code.
// R1.1: reads each named file in argument order.
// R1.2: reads stdin when no args or "-" is given.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	return executeShuf(opts, stdin, stdout, stderr)
}

// executeShuf runs the shuffle operation based on parsed options.
func executeShuf(opts *options, stdin io.Reader, stdout, stderr io.Writer) int {
	lines, err := collectInput(opts, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	w, cleanup, err := openOutput(opts.outputFile, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	defer cleanup()
	return outputLines(opts, lines, w, stderr)
}

// collectInput gathers lines from files, stdin, or range mode.
func collectInput(opts *options, stdin io.Reader) ([]string, error) {
	if opts.hasRange {
		return generateRange(opts.inputRange[0], opts.inputRange[1]), nil
	}
	files := opts.files
	if len(files) == 0 {
		files = []string{"-"}
	}
	return readAllLines(files, stdin)
}

// outputLines writes shuffled output, respecting -n and -r flags.
func outputLines(opts *options, lines []string, w io.Writer, stderr io.Writer) int {
	if opts.repeat {
		return writeRepeat(lines, opts.headCount, w, stderr)
	}
	shuffleLines(lines)
	count := len(lines)
	if opts.headCount >= 0 && opts.headCount < count {
		count = opts.headCount
	}
	return writeLines(lines[:count], w, stderr)
}

// writeRepeat outputs random lines with replacement. R2.3.
// If headCount < 0, runs indefinitely until write error/signal.
func writeRepeat(lines []string, headCount int, w io.Writer, stderr io.Writer) int {
	if len(lines) == 0 {
		return 0
	}
	bw := bufio.NewWriter(w)
	for i := 0; headCount < 0 || i < headCount; i++ {
		idx := rand.Intn(len(lines))
		if _, err := fmt.Fprintln(bw, lines[idx]); err != nil {
			// write error (e.g., broken pipe)
			return 1
		}
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
		return 1
	}
	return 0
}

// openOutput returns a writer and cleanup function for the output destination.
// R2.4: -o FILE writes to a file instead of stdout.
func openOutput(path string, stdout io.Writer) (io.Writer, func(), error) {
	if path == "" {
		return stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, unwrapPathError(err)
	}
	return f, func() { f.Close() }, nil
}

// generateRange creates a slice of string representations of integers
// from lo to hi inclusive. R2.1.
func generateRange(lo, hi int) []string {
	n := hi - lo + 1
	lines := make([]string, n)
	for i := range n {
		lines[i] = strconv.Itoa(lo + i)
	}
	return lines
}

// parseArgs separates flags from file arguments.
// Returns options and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (*options, int) {
	opts := &options{headCount: -1}
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			opts.files = append(opts.files, arg)
			continue
		}
		if arg == "-" {
			opts.files = append(opts.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		consumed, code := handleFlag(args, i, opts, stdout, stderr)
		if code >= 0 {
			return nil, code
		}
		i += consumed
	}
	if err := validateOptions(opts); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		printTryHelp(stderr)
		return nil, 1
	}
	return opts, -1
}

// handleFlag processes a single flag argument, returning how many extra
// args were consumed and an exit code (-1 = continue).
func handleFlag(args []string, i int, opts *options, stdout, stderr io.Writer) (int, int) {
	arg := args[i]
	switch {
	case arg == "--help":
		printHelp(stdout)
		return 0, 0
	case arg == "--version":
		printVersion(stdout)
		return 0, 0
	case arg == "-r" || arg == "--repeat":
		opts.repeat = true
		return 0, -1
	case arg == "-n" || arg == "--head-count":
		return parseNextArg(args, i, opts, stderr, setHeadCount)
	case strings.HasPrefix(arg, "-n"):
		return 0, parseInlineValue(arg[2:], opts, stderr, setHeadCount)
	case strings.HasPrefix(arg, "--head-count="):
		return 0, parseInlineValue(arg[len("--head-count="):], opts, stderr, setHeadCount)
	case arg == "-i" || arg == "--input-range":
		return parseNextArg(args, i, opts, stderr, setInputRange)
	case strings.HasPrefix(arg, "-i"):
		return 0, parseInlineValue(arg[2:], opts, stderr, setInputRange)
	case strings.HasPrefix(arg, "--input-range="):
		return 0, parseInlineValue(arg[len("--input-range="):], opts, stderr, setInputRange)
	case arg == "-o" || arg == "--output":
		return parseNextArg(args, i, opts, stderr, setOutputFile)
	case strings.HasPrefix(arg, "-o"):
		return 0, parseInlineValue(arg[2:], opts, stderr, setOutputFile)
	case strings.HasPrefix(arg, "--output="):
		return 0, parseInlineValue(arg[len("--output="):], opts, stderr, setOutputFile)
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 0, 1
	}
}

// setter is a function that applies a string value to options.
type setter func(opts *options, value string) error

// parseNextArg reads the value from the next argument.
func parseNextArg(args []string, i int, opts *options, stderr io.Writer, fn setter) (int, int) {
	if i+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option '%s' requires an argument\n", progName, args[i])
		printTryHelp(stderr)
		return 0, 1
	}
	if err := fn(opts, args[i+1]); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		printTryHelp(stderr)
		return 1, 1
	}
	return 1, -1
}

// parseInlineValue applies an inline flag value (e.g., -n5).
func parseInlineValue(val string, opts *options, stderr io.Writer, fn setter) int {
	if err := fn(opts, val); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// setHeadCount parses and sets the head count option. R2.2.
func setHeadCount(opts *options, value string) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid line count: '%s'", value)
	}
	opts.headCount = n
	return nil
}

// setInputRange parses and sets the input range option. R2.1.
func setInputRange(opts *options, value string) error {
	lo, hi, err := parseRange(value)
	if err != nil {
		return err
	}
	opts.hasRange = true
	opts.inputRange = [2]int{lo, hi}
	return nil
}

// parseRange parses a "LO-HI" string into two integers.
func parseRange(s string) (int, int, error) {
	loStr, hiStr, found := strings.Cut(s, "-")
	if !found {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	lo, err := strconv.Atoi(loStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	hi, err := strconv.Atoi(hiStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	return lo, hi, nil
}

// setOutputFile sets the output file option. R2.4.
func setOutputFile(opts *options, value string) error {
	opts.outputFile = value
	return nil
}

// validateOptions checks for conflicting options. R2.1.
func validateOptions(opts *options) error {
	if opts.hasRange && len(opts.files) > 0 {
		return fmt.Errorf("extra operand '%s'", opts.files[0])
	}
	return nil
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]\n", progName)
	fmt.Fprintln(w, "Write a random permutation of the input lines to standard output.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -i, --input-range=LO-HI   treat each number LO through HI as an input line")
	fmt.Fprintln(w, "  -n, --head-count=COUNT     output at most COUNT lines")
	fmt.Fprintln(w, "  -o, --output=FILE          write result to FILE instead of standard output")
	fmt.Fprintln(w, "  -r, --repeat               output lines can repeat")
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// readAllLines reads all lines from the given files, using stdin for "-".
// R1.4: each line is terminated by newline; last line included without trailing newline.
func readAllLines(files []string, stdin io.Reader) ([]string, error) {
	var lines []string
	for _, name := range files {
		fileLines, err := readFileLines(name, stdin)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

// readFileLines reads lines from a single file or stdin.
func readFileLines(name string, stdin io.Reader) ([]string, error) {
	if name == "-" {
		return scanLines(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, unwrapPathError(err)
	}
	defer f.Close() // best-effort close on read-only file
	return scanLines(f)
}

// scanLines reads all lines from r, splitting on newline.
// R1.4: includes the last line even if it lacks a trailing newline.
func scanLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// shuffleLines randomly permutes the slice in place.
// R1.3: each input line appears exactly once (Fisher-Yates shuffle).
func shuffleLines(lines []string) {
	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
}

// writeLines writes shuffled lines to w, one per line.
func writeLines(lines []string, w, stderr io.Writer) int {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := fmt.Fprintln(bw, line); err != nil {
			fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
			return 1
		}
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
		return 1
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
