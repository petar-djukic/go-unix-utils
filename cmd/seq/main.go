// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/seq implements GNU seq: print a sequence of numbers.
//
// Implements prd019-seq R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "seq"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses arguments and generates the sequence.
func run(args []string, stdout, stderr io.Writer) int {
	positional, code := parseFlags(args, stdout)
	if code >= 0 {
		return code
	}
	first, step, last, err := parsePositional(positional)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if step == 0 {
		fmt.Fprintf(stderr, "%s: invalid Zero increment value: '%s'\n",
			programName, positional[1])
		return 1
	}
	return printSequence(stdout, stderr, first, step, last)
}

// parseFlags extracts --help/--version and returns positional args.
// Returns (positional, -1) on normal flow, or (nil, exitCode) for early exit.
func parseFlags(args []string, stdout io.Writer) ([]string, int) {
	var positional []string
	flagsDone := false
	for _, a := range args {
		if flagsDone {
			positional = append(positional, a)
			continue
		}
		switch a {
		case "--help":
			printHelp(stdout)
			return nil, 0
		case "--version":
			printVersion(stdout)
			return nil, 0
		case "--":
			flagsDone = true
		default:
			positional = append(positional, a)
		}
	}
	return positional, -1
}

// parsePositional interprets 1, 2, or 3 positional arguments.
// R1.1: seq LAST, seq FIRST LAST, seq FIRST INCREMENT LAST.
// R1.4: defaults FIRST=1 and INCREMENT=1 when omitted.
func parsePositional(args []string) (first, step, last float64, err error) {
	switch len(args) {
	case 0:
		return 0, 0, 0, fmt.Errorf("missing operand")
	case 1:
		last, err = parseNumber(args[0])
		return 1, 1, last, err
	case 2:
		first, err = parseNumber(args[0])
		if err != nil {
			return 0, 0, 0, err
		}
		last, err = parseNumber(args[1])
		return first, 1, last, err
	case 3:
		first, err = parseNumber(args[0])
		if err != nil {
			return 0, 0, 0, err
		}
		step, err = parseNumber(args[1])
		if err != nil {
			return 0, 0, 0, err
		}
		last, err = parseNumber(args[2])
		return first, step, last, err
	default:
		return 0, 0, 0, fmt.Errorf("extra operand '%s'", args[3])
	}
}

// parseNumber parses a string as a floating-point number.
func parseNumber(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid floating point argument: '%s'", s)
	}
	if math.IsNaN(v) {
		return 0, fmt.Errorf("invalid floating point argument: '%s'", s)
	}
	return v, nil
}

// sequenceLength computes how many values to print.
// R1.3: FIRST==LAST prints one number.
// R1.4: empty when direction mismatches step sign.
func sequenceLength(first, step, last float64) int {
	if step > 0 && first > last {
		return 0
	}
	if step < 0 && first < last {
		return 0
	}
	return int(math.Floor((last-first)/step)) + 1
}

// printSequence writes the number sequence to stdout.
// R1.2: generates numbers from FIRST by STEP, stopping at LAST.
func printSequence(stdout, stderr io.Writer, first, step, last float64) int {
	bw := bufio.NewWriter(stdout)
	n := sequenceLength(first, step, last)
	for i := range n {
		v := first + float64(i)*step
		fmt.Fprintln(bw, formatNumber(v))
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return 0
}

// formatNumber formats a number for output, omitting decimal for integers.
// R2.3: integer sequences produce integers without decimal points.
func formatNumber(v float64) string {
	if v == math.Trunc(v) && !math.IsInf(v, 0) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// printHelp prints usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: %s [OPTION]... LAST
  or:  %s [OPTION]... FIRST LAST
  or:  %s [OPTION]... FIRST INCREMENT LAST
Print numbers from FIRST to LAST, in steps of INCREMENT.

Mandatory arguments to long options are mandatory for short options too.
  -f, --format=FORMAT      use printf style floating-point FORMAT
  -s, --separator=STRING   use STRING to separate numbers (default: \n)
  -w, --equal-width        equalize width by padding with leading zeroes
      --help        display this help and exit
      --version     output version information and exit

If FIRST or INCREMENT is omitted, it defaults to 1.  That is, an
omitted INCREMENT defaults to 1 even when LAST is smaller than FIRST.
The sequence of numbers ends when the sum of the current number and
INCREMENT would become greater than LAST.
FIRST, INCREMENT, and LAST are interpreted as floating point values.
INCREMENT is usually positive if FIRST is smaller than LAST, and
INCREMENT is usually negative if FIRST is greater than LAST.
INCREMENT must not be 0; none of FIRST, INCREMENT and LAST may be NaN.
`, programName, programName, programName)
}

// printVersion prints version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", programName)
}
