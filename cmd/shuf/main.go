// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/shuf implements prd064-shuf R1.1–R1.4, R2.1–R2.4: shuffle input lines
// randomly with range mode, head count, repeat mode, and output file support.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// options holds parsed command-line flags.
type options struct {
	inputRange string // R2.1: -i LO-HI
	headCount  int    // R2.2: -n COUNT (-1 = unset)
	repeat     bool   // R2.3: -r
	outputFile string // R2.4: -o FILE
	files      []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
		os.Exit(1)
	}
}

// run implements the core shuf logic.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	_ = stderr
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if err := validateOpts(opts); err != nil {
		return err
	}
	lines, err := collectLines(opts, stdin)
	if err != nil {
		return err
	}
	w, closer, err := openOutput(opts, stdout)
	if err != nil {
		return err
	}
	defer closer()
	return outputLines(w, lines, opts)
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (options, error) {
	opts := options{headCount: -1}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			opts.files = append(opts.files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			next, err := parseLongFlag(args, i, &opts)
			if err != nil {
				return options{}, err
			}
			i = next
			continue
		}
		next, err := parseShortFlags(args, i, &opts)
		if err != nil {
			return options{}, err
		}
		i = next
	}
	return opts, nil
}

// parseLongFlag handles --long-form flags. Returns next index.
func parseLongFlag(args []string, i int, opts *options) (int, error) {
	arg := args[i]
	key, val, hasEq := strings.Cut(arg, "=")
	switch key {
	case "--input-range":
		v, err := longFlagValue(args, i, val, hasEq, key)
		if err != nil {
			return 0, err
		}
		opts.inputRange = v
		if hasEq {
			return i + 1, nil
		}
		return i + 2, nil
	case "--head-count":
		v, err := longFlagValue(args, i, val, hasEq, key)
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("invalid line count: %q", v)
		}
		opts.headCount = n
		if hasEq {
			return i + 1, nil
		}
		return i + 2, nil
	case "--repeat":
		opts.repeat = true
		return i + 1, nil
	case "--output":
		v, err := longFlagValue(args, i, val, hasEq, key)
		if err != nil {
			return 0, err
		}
		opts.outputFile = v
		if hasEq {
			return i + 1, nil
		}
		return i + 2, nil
	default:
		return 0, fmt.Errorf("unrecognized option %q", arg)
	}
}

// longFlagValue extracts the value for a --key=val or --key val flag.
func longFlagValue(args []string, i int, val string, hasEq bool, key string) (string, error) {
	if hasEq {
		return val, nil
	}
	if i+1 >= len(args) {
		return "", fmt.Errorf("option %q requires an argument", key)
	}
	return args[i+1], nil
}

// parseShortFlags handles short flags, including combined flags like -rn5.
func parseShortFlags(args []string, i int, opts *options) (int, error) {
	arg := args[i]
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		opts.files = append(opts.files, arg)
		return i + 1, nil
	}
	j := 1
	for j < len(arg) {
		switch arg[j] {
		case 'r':
			opts.repeat = true
			j++
		case 'i':
			return shortFlagWithValue(args, i, arg, j, func(v string) error {
				opts.inputRange = v
				return nil
			})
		case 'n':
			return shortFlagWithValue(args, i, arg, j, func(v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid line count: %q", v)
				}
				opts.headCount = n
				return nil
			})
		case 'o':
			return shortFlagWithValue(args, i, arg, j, func(v string) error {
				opts.outputFile = v
				return nil
			})
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", arg[j])
		}
	}
	return i + 1, nil
}

// shortFlagWithValue extracts the value for a short flag that takes an argument.
// The value is either the remainder of the current arg or the next arg.
func shortFlagWithValue(
	args []string, i int, arg string, j int, apply func(string) error,
) (int, error) {
	if j+1 < len(arg) {
		if err := apply(arg[j+1:]); err != nil {
			return 0, err
		}
		return i + 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%c'", arg[j])
	}
	if err := apply(args[i+1]); err != nil {
		return 0, err
	}
	return i + 2, nil
}

// validateOpts checks for conflicting options.
func validateOpts(opts options) error {
	if opts.inputRange != "" && len(opts.files) > 0 {
		return fmt.Errorf("cannot combine -i and file arguments")
	}
	return nil
}

// collectLines gathers input lines based on mode.
func collectLines(opts options, stdin io.Reader) ([]string, error) {
	if opts.inputRange != "" {
		return rangeLines(opts.inputRange)
	}
	return readAllLines(opts.files, stdin)
}

// rangeLines generates integer strings from LO to HI inclusive.
// R2.1: -i LO-HI generates integers in [LO, HI].
func rangeLines(spec string) ([]string, error) {
	lo, hi, err := parseRange(spec)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		lines = append(lines, strconv.Itoa(i))
	}
	return lines, nil
}

// parseRange parses "LO-HI" into two integers.
func parseRange(spec string) (int, int, error) {
	idx := strings.Index(spec, "-")
	if idx <= 0 {
		return 0, 0, fmt.Errorf("invalid input range: %q", spec)
	}
	lo, err := strconv.Atoi(spec[:idx])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid input range: %q", spec)
	}
	hi, err := strconv.Atoi(spec[idx+1:])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid input range: %q", spec)
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("invalid input range: %q", spec)
	}
	return lo, hi, nil
}

// readAllLines reads lines from the given files, or stdin if no files given.
// R1.1: reads all lines from each file.
// R1.2: reads from stdin when no files given or "-" specified.
// R1.4: last line need not end with newline but is included.
func readAllLines(files []string, stdin io.Reader) ([]string, error) {
	if len(files) == 0 {
		return scanLines(stdin)
	}
	var all []string
	for _, name := range files {
		lines, err := readFileLines(name, stdin)
		if err != nil {
			return nil, err
		}
		all = append(all, lines...)
	}
	return all, nil
}

// readFileLines reads lines from a single file or stdin for "-".
func readFileLines(name string, stdin io.Reader) ([]string, error) {
	if name == "-" {
		return scanLines(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return scanLines(f)
}

// scanLines reads all lines from r using a scanner.
func scanLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// openOutput returns a writer and closer for the output destination.
// R2.4: -o FILE writes to FILE instead of stdout.
func openOutput(opts options, stdout io.Writer) (io.Writer, func(), error) {
	if opts.outputFile == "" {
		return stdout, func() {}, nil
	}
	f, err := os.Create(opts.outputFile)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// outputLines writes shuffled lines, respecting -n and -r flags.
func outputLines(w io.Writer, lines []string, opts options) error {
	if len(lines) == 0 {
		return nil
	}
	if opts.repeat {
		return writeRepeat(w, lines, opts.headCount)
	}
	return writePermutation(w, lines, opts.headCount)
}

// writePermutation shuffles and writes lines, limited by headCount.
// R1.3: each line appears exactly once.
// R2.2: -n COUNT limits output.
func writePermutation(w io.Writer, lines []string, headCount int) error {
	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
	n := len(lines)
	if headCount >= 0 && headCount < n {
		n = headCount
	}
	return writeNLines(w, lines[:n])
}

// writeRepeat writes random selections with replacement.
// R2.3: -r allows duplicates. Without -n, runs indefinitely.
func writeRepeat(w io.Writer, lines []string, headCount int) error {
	bw := bufio.NewWriter(w)
	count := 0
	for headCount < 0 || count < headCount {
		idx := rand.IntN(len(lines))
		if _, err := bw.WriteString(lines[idx]); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
		count++
		if count%4096 == 0 {
			if err := bw.Flush(); err != nil {
				return err
			}
		}
	}
	return bw.Flush()
}

// writeNLines writes n lines to w.
func writeNLines(w io.Writer, lines []string) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return bw.Flush()
}
