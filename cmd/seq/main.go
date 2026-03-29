// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/seq implements GNU seq: print a sequence of numbers.
//
// Implements prd019-seq R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
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

const programName = "seq"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// seqOptions holds parsed flag values for seq.
type seqOptions struct {
	format    string
	separator string
	equalWidth bool
}

// run parses arguments and generates the sequence.
func run(args []string, stdout, stderr io.Writer) int {
	opts, positional, code := parseFlags(args, stdout)
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
	if opts.format != "" {
		if err := validateFormat(opts.format); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", programName, err)
			return 1
		}
	}
	return printSequence(stdout, stderr, first, step, last, opts)
}

// parseFlags extracts flags and returns options, positional args, and exit code.
// Returns (opts, positional, -1) on normal flow, or (opts, nil, exitCode) for early exit.
func parseFlags(args []string, stdout io.Writer) (seqOptions, []string, int) {
	var opts seqOptions
	opts.separator = "\n"
	var positional []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if flagsDone {
			positional = append(positional, a)
			continue
		}
		switch {
		case a == "--help":
			printHelp(stdout)
			return opts, nil, 0
		case a == "--version":
			printVersion(stdout)
			return opts, nil, 0
		case a == "--":
			flagsDone = true
		case strings.HasPrefix(a, "--format="):
			opts.format = a[len("--format="):]
		case a == "--format" || a == "-f":
			i, opts.format = consumeNextArg(args, i)
		case strings.HasPrefix(a, "-f"):
			opts.format = a[2:]
		case strings.HasPrefix(a, "--separator="):
			opts.separator = a[len("--separator="):]
		case a == "--separator" || a == "-s":
			i, opts.separator = consumeNextArg(args, i)
		case strings.HasPrefix(a, "-s"):
			opts.separator = a[2:]
		case a == "--equal-width" || a == "-w":
			opts.equalWidth = true
		default:
			positional = append(positional, a)
		}
	}
	return opts, positional, -1
}

// consumeNextArg advances the index and returns the next argument value.
func consumeNextArg(args []string, i int) (int, string) {
	if i+1 < len(args) {
		return i + 1, args[i+1]
	}
	return i, ""
}

// parsePositional interprets 1, 2, or 3 positional arguments.
// R1.1: seq LAST, seq FIRST LAST, seq FIRST INCREMENT LAST.
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
// R2.1: default separator is newline. R2.2: custom separator via -s.
// R2.3: trailing newline after last number.
func printSequence(stdout, stderr io.Writer, first, step, last float64, opts seqOptions) int {
	bw := bufio.NewWriter(stdout)
	n := sequenceLength(first, step, last)
	fmtFunc := buildFormatter(opts, first, last)
	for i := range n {
		v := first + float64(i)*step
		if i > 0 {
			fmt.Fprint(bw, opts.separator)
		}
		fmt.Fprint(bw, fmtFunc(v))
	}
	if n > 0 {
		fmt.Fprint(bw, "\n")
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return 0
}

// buildFormatter returns a function that formats each number.
// R3.4: -f takes precedence over -w.
func buildFormatter(opts seqOptions, first, last float64) func(float64) string {
	if opts.format != "" {
		return func(v float64) string {
			return fmt.Sprintf(opts.format, v)
		}
	}
	if opts.equalWidth {
		width := equalWidth(first, last)
		return func(v float64) string {
			return zeroPad(formatNumber(v), width)
		}
	}
	return formatNumber
}

// equalWidth computes the display width for -w mode.
// D1: width from widest of FIRST and LAST including sign and decimal.
func equalWidth(first, last float64) int {
	w1 := len(formatNumber(first))
	w2 := len(formatNumber(last))
	if w1 > w2 {
		return w1
	}
	return w2
}

// zeroPad pads a formatted number string to width with leading zeros.
// Handles negative numbers by inserting zeros after the minus sign.
func zeroPad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	if s[0] == '-' {
		return "-" + strings.Repeat("0", width-len(s)) + s[1:]
	}
	return strings.Repeat("0", width-len(s)) + s
}

// validateFormat checks that a format string has exactly one valid
// floating-point conversion specifier.
// R3.1/R3.2: must contain exactly one %a, %e, %f, %g (or uppercase).
func validateFormat(format string) error {
	count := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		if i >= len(format) {
			return fmt.Errorf("format '%%' missing conversion specifier")
		}
		if format[i] == '%' {
			continue
		}
		// Skip flags: -, +, space, #, 0
		for i < len(format) && isFormatFlag(format[i]) {
			i++
		}
		// Skip width
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
		// Skip precision
		if i < len(format) && format[i] == '.' {
			i++
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
		}
		if i >= len(format) {
			return fmtConversionError(format)
		}
		if !isFloatConversion(format[i]) {
			return fmtConversionError(format)
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("format '%s' has no %% directive", format)
	}
	if count > 1 {
		return fmt.Errorf("format '%s' has too many %% directives", format)
	}
	return nil
}

// isFormatFlag returns true if c is a printf flag character.
func isFormatFlag(c byte) bool {
	return c == '-' || c == '+' || c == ' ' || c == '#' || c == '0'
}

// isFloatConversion returns true if c is a valid float conversion specifier.
func isFloatConversion(c byte) bool {
	return c == 'a' || c == 'e' || c == 'f' || c == 'g' ||
		c == 'A' || c == 'E' || c == 'F' || c == 'G'
}

// fmtConversionError returns a format conversion error.
func fmtConversionError(format string) error {
	return fmt.Errorf("format '%s' has unknown %%%% directive", format)
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
