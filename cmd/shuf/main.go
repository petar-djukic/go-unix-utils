// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/shuf implements prd064-shuf R1.1–R1.4, R2.1–R2.4, R3.1–R3.4: shuffle
// input lines randomly with range mode, head count, repeat mode, output file,
// random source, zero-terminated delimiter, and echo mode support.
package main

import (
	"bufio"
	"bytes"
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
	inputRange   string // R2.1: -i LO-HI
	headCount    int    // R2.2: -n COUNT (-1 = unset)
	repeat       bool   // R2.3: -r
	outputFile   string // R2.4: -o FILE
	randomSource string // R3.1: --random-source=FILE
	zeroTerm     bool   // R3.2: -z
	echo         bool   // R3.3: -e
	files        []string
}

// delim returns the line delimiter byte.
// R3.2: NUL when -z is set, newline otherwise.
func (o options) delim() byte {
	if o.zeroTerm {
		return 0
	}
	return '\n'
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
	rng, err := createRNG(opts)
	if err != nil {
		return err
	}
	w, closer, err := openOutput(opts, stdout)
	if err != nil {
		return err
	}
	defer closer()
	return outputLines(w, lines, opts, rng)
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
		return applyLongValue(args, i, val, hasEq, key, setString(&opts.inputRange))
	case "--head-count":
		return applyLongValue(args, i, val, hasEq, key, parseHeadCount(&opts.headCount))
	case "--repeat":
		opts.repeat = true
		return i + 1, nil
	case "--output":
		return applyLongValue(args, i, val, hasEq, key, setString(&opts.outputFile))
	case "--random-source":
		return applyLongValue(args, i, val, hasEq, key, setString(&opts.randomSource))
	case "--zero-terminated":
		opts.zeroTerm = true
		return i + 1, nil
	case "--echo":
		opts.echo = true
		return i + 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option %q", arg)
	}
}

// applyLongValue extracts and applies a value for a --key=val or --key val flag.
func applyLongValue(
	args []string, i int, val string, hasEq bool, key string, apply func(string) error,
) (int, error) {
	v, err := longFlagValue(args, i, val, hasEq, key)
	if err != nil {
		return 0, err
	}
	if err := apply(v); err != nil {
		return 0, err
	}
	if hasEq {
		return i + 1, nil
	}
	return i + 2, nil
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

// setString returns an apply function that sets a string pointer.
func setString(target *string) func(string) error {
	return func(v string) error {
		*target = v
		return nil
	}
}

// parseHeadCount returns an apply function for head count values.
func parseHeadCount(target *int) func(string) error {
	return func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid line count: %q", v)
		}
		*target = n
		return nil
	}
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
		next, err := applyShortChar(args, i, arg, j, opts)
		if err != nil {
			return 0, err
		}
		if next != 0 {
			return next, nil
		}
		j++
	}
	return i + 1, nil
}

// applyShortChar processes a single short flag character.
// Returns (0, nil) to continue to the next char, or (nextIndex, nil) to stop.
func applyShortChar(args []string, i int, arg string, j int, opts *options) (int, error) {
	switch arg[j] {
	case 'r':
		opts.repeat = true
		return 0, nil
	case 'e':
		opts.echo = true
		return 0, nil
	case 'z':
		opts.zeroTerm = true
		return 0, nil
	case 'i':
		return shortFlagWithValue(args, i, arg, j, setString(&opts.inputRange))
	case 'n':
		return shortFlagWithValue(args, i, arg, j, parseHeadCount(&opts.headCount))
	case 'o':
		return shortFlagWithValue(args, i, arg, j, setString(&opts.outputFile))
	default:
		return 0, fmt.Errorf("invalid option -- '%c'", arg[j])
	}
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
	if opts.inputRange != "" && len(opts.files) > 0 && !opts.echo {
		return fmt.Errorf("cannot combine -i and file arguments")
	}
	if opts.echo && opts.inputRange != "" {
		return fmt.Errorf("cannot combine -e and -i")
	}
	return nil
}

// collectLines gathers input lines based on mode.
// R3.3: -e treats positional args as input lines.
func collectLines(opts options, stdin io.Reader) ([]string, error) {
	if opts.echo {
		return opts.files, nil
	}
	if opts.inputRange != "" {
		return rangeLines(opts.inputRange)
	}
	return readAllLines(opts.files, stdin, opts.delim())
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
// R1.1, R1.2, R1.4: reads all lines respecting the configured delimiter.
func readAllLines(files []string, stdin io.Reader, delim byte) ([]string, error) {
	if len(files) == 0 {
		return scanWithDelim(stdin, delim)
	}
	var all []string
	for _, name := range files {
		lines, err := readFileLines(name, stdin, delim)
		if err != nil {
			return nil, err
		}
		all = append(all, lines...)
	}
	return all, nil
}

// readFileLines reads lines from a single file or stdin for "-".
func readFileLines(name string, stdin io.Reader, delim byte) ([]string, error) {
	if name == "-" {
		return scanWithDelim(stdin, delim)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return scanWithDelim(f, delim)
}

// scanWithDelim reads all records from r using the given delimiter.
// R3.2: when delim is NUL, splits on NUL instead of newline.
func scanWithDelim(r io.Reader, delim byte) ([]string, error) {
	scanner := bufio.NewScanner(r)
	if delim != '\n' {
		scanner.Split(splitByByte(delim))
	}
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// splitByByte returns a bufio.SplitFunc that splits on the given byte.
func splitByByte(delim byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, delim); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
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

// createRNG creates a random number generator based on options.
// R3.1: --random-source uses bytes from FILE as entropy seed.
func createRNG(opts options) (*rand.Rand, error) {
	if opts.randomSource == "" {
		return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())), nil
	}
	data, err := os.ReadFile(opts.randomSource)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q for reading: %v", opts.randomSource, err)
	}
	var seed [32]byte
	copy(seed[:], data)
	return rand.New(rand.NewChaCha8(seed)), nil
}

// outputLines writes shuffled lines, respecting -n and -r flags.
// R3.4: empty input produces no output.
func outputLines(w io.Writer, lines []string, opts options, rng *rand.Rand) error {
	if len(lines) == 0 {
		return nil
	}
	if opts.repeat {
		return writeRepeat(w, lines, opts.headCount, opts.delim(), rng)
	}
	return writePermutation(w, lines, opts.headCount, opts.delim(), rng)
}

// writePermutation shuffles and writes lines, limited by headCount.
// R1.3: each line appears exactly once.
// R2.2: -n COUNT limits output.
func writePermutation(w io.Writer, lines []string, headCount int, delim byte, rng *rand.Rand) error {
	rng.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
	n := len(lines)
	if headCount >= 0 && headCount < n {
		n = headCount
	}
	return writeNLines(w, lines[:n], delim)
}

// writeRepeat writes random selections with replacement.
// R2.3: -r allows duplicates. Without -n, runs indefinitely.
func writeRepeat(w io.Writer, lines []string, headCount int, delim byte, rng *rand.Rand) error {
	bw := bufio.NewWriter(w)
	count := 0
	for headCount < 0 || count < headCount {
		idx := rng.IntN(len(lines))
		if _, err := bw.WriteString(lines[idx]); err != nil {
			return err
		}
		if err := bw.WriteByte(delim); err != nil {
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

// writeNLines writes lines to w separated by the given delimiter.
func writeNLines(w io.Writer, lines []string, delim byte) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		if err := bw.WriteByte(delim); err != nil {
			return err
		}
	}
	return bw.Flush()
}
