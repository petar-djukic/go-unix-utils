// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd019-seq R1.1–R1.5, R2.1–R2.3: numeric sequence generation with
// single, two, and three argument forms, floating-point support, custom
// separators, and integer/float format selection.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "seq"

// seqOpts holds parsed arguments for sequence generation.
type seqOpts struct {
	first     float64
	step      float64
	last      float64
	format    string
	separator string
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the numeric sequence.
// Returns 0 on success, 1 on error.
func run(args []string, stdout, stderr io.Writer) int {
	opts, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	printSequence(stdout, opts)
	return 0
}

// parseArgs separates flags from numeric arguments.
// Returns opts and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (seqOpts, int) {
	sep, numStrs, code := extractNums(args, stdout, stderr)
	if code >= 0 {
		return seqOpts{}, code
	}
	opts, err := buildOpts(numStrs, sep)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		if strings.Contains(err.Error(), "operand") {
			printTryHelp(stderr)
		}
		return seqOpts{}, 1
	}
	return opts, -1
}

// extractNums filters args into numeric strings, handling flags.
// Returns (separator, numStrs, exit code). Exit code -1 means continue.
// R2.2: parses -s STRING and --separator=STRING.
func extractNums(args []string, stdout, stderr io.Writer) (string, []string, int) {
	var numStrs []string
	separator := "\n" // R2.1: default separator is newline
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			numStrs = append(numStrs, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--help" {
			printHelp(stdout)
			return "", nil, 0
		}
		if arg == "--version" {
			printVersion(stdout)
			return "", nil, 0
		}
		// R2.2: --separator=STRING
		if strings.HasPrefix(arg, "--separator=") {
			separator = arg[len("--separator="):]
			continue
		}
		if arg == "--separator" || arg == "-s" {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr,
					"%s: option '%s' requires an argument\n",
					progName, arg)
				printTryHelp(stderr)
				return "", nil, 1
			}
			i++
			separator = args[i]
			continue
		}
		// R2.2: -sSTRING (no space)
		if strings.HasPrefix(arg, "-s") && len(arg) > 2 {
			separator = arg[2:]
			continue
		}
		if isFlag(arg) {
			fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n",
				progName, arg)
			printTryHelp(stderr)
			return "", nil, 1
		}
		numStrs = append(numStrs, arg)
	}
	return separator, numStrs, -1
}

// isFlag returns true if arg looks like a flag (starts with - but is not a number).
func isFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || len(arg) < 2 {
		return false
	}
	return !(arg[1] >= '0' && arg[1] <= '9' || arg[1] == '.')
}

// buildOpts validates numeric strings and constructs seqOpts.
// R1.1: supports 1, 2, or 3 positional numeric arguments.
// R1.5: rejects zero step with an error.
func buildOpts(numStrs []string, separator string) (seqOpts, error) {
	if len(numStrs) == 0 {
		return seqOpts{}, fmt.Errorf("missing operand")
	}
	if len(numStrs) > 3 {
		return seqOpts{}, fmt.Errorf("extra operand '%s'", numStrs[3])
	}
	values, err := parseValues(numStrs)
	if err != nil {
		return seqOpts{}, err
	}
	first, step, last := assignArgs(values)
	// R1.5: STEP must not be zero.
	if step == 0 {
		return seqOpts{}, fmt.Errorf(
			"invalid Zero increment value: '%s'", numStrs[1])
	}
	return seqOpts{
		first:     first,
		step:      step,
		last:      last,
		format:    computeFormat(numStrs),
		separator: separator,
	}, nil
}

// parseValues converts string arguments to float64 values.
func parseValues(numStrs []string) ([]float64, error) {
	values := make([]float64, len(numStrs))
	for i, s := range numStrs {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) {
			return nil, fmt.Errorf(
				"invalid floating point argument: '%s'", s)
		}
		values[i] = v
	}
	return values, nil
}

// assignArgs maps 1, 2, or 3 positional values to first, step, last.
// R1.1: seq LAST → first=1, step=1. seq FIRST LAST → step=1.
func assignArgs(values []float64) (float64, float64, float64) {
	switch len(values) {
	case 1:
		return 1, 1, values[0]
	case 2:
		return values[0], 1, values[1]
	default:
		return values[0], values[1], values[2]
	}
}

// computeFormat determines the printf format from input argument precision.
// R2.3: integers produce integer output; floats use minimum precision.
func computeFormat(args []string) string {
	maxPrec := 0
	for _, arg := range args {
		if prec := decimalPlaces(arg); prec > maxPrec {
			maxPrec = prec
		}
	}
	return fmt.Sprintf("%%.%df", maxPrec)
}

// decimalPlaces returns the number of digits after the decimal point.
func decimalPlaces(s string) int {
	s = strings.TrimPrefix(s, "-")
	idx := strings.Index(s, ".")
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}

// sequenceCount computes how many values the sequence should produce.
// R1.3: FIRST == LAST produces exactly 1 value.
// R1.4: empty sequence when direction opposes step sign.
func sequenceCount(first, step, last float64) int {
	if step > 0 && first > last {
		return 0
	}
	if step < 0 && first < last {
		return 0
	}
	n := (last - first) / step
	// Snap to nearest integer when very close, to handle float rounding.
	if rounded := math.Round(n); math.Abs(n-rounded) < 1e-10 {
		n = rounded
	}
	if n < 0 {
		return 0
	}
	return int(n) + 1
}

// printSequence generates and writes the numeric sequence to w.
// R2.1: numbers separated by separator, trailing newline after last.
// R2.2: custom separator replaces newline between numbers.
func printSequence(w io.Writer, opts seqOpts) {
	bw := bufio.NewWriter(w)
	count := sequenceCount(opts.first, opts.step, opts.last)
	for i := range count {
		if i > 0 {
			fmt.Fprint(bw, opts.separator)
		}
		val := opts.first + float64(i)*opts.step
		fmt.Fprintf(bw, opts.format, val)
	}
	if count > 0 {
		fmt.Fprint(bw, "\n")
	}
	bw.Flush() // best-effort flush
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... LAST\n", progName)
	fmt.Fprintf(w, "  or:  %s [OPTION]... FIRST LAST\n", progName)
	fmt.Fprintf(w, "  or:  %s [OPTION]... FIRST INCREMENT LAST\n", progName)
	fmt.Fprintln(w, "Print numbers from FIRST to LAST, in steps of INCREMENT.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -s, --separator=STRING  use STRING to separate numbers (default: \\n)")
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to w.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
