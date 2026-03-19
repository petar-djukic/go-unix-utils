// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd019-seq R1.1–R1.5, R2.1–R2.4, R3.1–R3.4: numeric sequence
// generation with format strings, equal-width padding, and error handling.
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

// parsedFlags holds extracted flag values from argument parsing.
type parsedFlags struct {
	separator  string
	format     string
	equalWidth bool
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
	flags, numStrs, code := extractFlags(args, stdout, stderr)
	if code >= 0 {
		return seqOpts{}, code
	}
	// R3.2: validate format string if provided.
	if flags.format != "" {
		if err := validateFormat(flags.format); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", progName, err)
			return seqOpts{}, 1
		}
	}
	opts, err := buildOpts(numStrs, flags)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		if strings.Contains(err.Error(), "operand") {
			printTryHelp(stderr)
		}
		return seqOpts{}, 1
	}
	return opts, -1
}

// extractFlags filters args into numeric strings, handling flags.
// Returns (flags, numStrs, exit code). Exit code -1 means continue.
func extractFlags(args []string, stdout, stderr io.Writer) (parsedFlags, []string, int) {
	var numStrs []string
	flags := parsedFlags{separator: "\n"} // R2.1: default separator
	flagsDone := false
	for i := 0; i < len(args); i++ {
		if flagsDone {
			numStrs = append(numStrs, args[i])
			continue
		}
		if args[i] == "--" {
			flagsDone = true
			continue
		}
		handled, code := handleFlag(args, &i, &flags, stdout, stderr)
		if code >= 0 {
			return parsedFlags{}, nil, code
		}
		if handled {
			continue
		}
		numStrs = append(numStrs, args[i])
	}
	return flags, numStrs, -1
}

// handleFlag processes one potential flag at args[*i].
// Returns (handled, exit code). code -1 means continue.
func handleFlag(args []string, i *int, flags *parsedFlags, stdout, stderr io.Writer) (bool, int) {
	arg := args[*i]
	if arg == "--help" {
		printHelp(stdout)
		return true, 0
	}
	if arg == "--version" {
		printVersion(stdout)
		return true, 0
	}
	// R3.3: -w/--equal-width
	if arg == "-w" || arg == "--equal-width" {
		flags.equalWidth = true
		return true, -1
	}
	if handled, code := handleValueFlag(args, i, flags, stderr); handled {
		return true, code
	}
	if isFlag(arg) {
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return true, 1
	}
	return false, -1
}

// handleValueFlag handles flags that take a value: -s, -f, --separator, --format.
// Returns (handled, exit code). R2.2, R3.1.
func handleValueFlag(args []string, i *int, flags *parsedFlags, stderr io.Writer) (bool, int) {
	arg := args[*i]
	// Long form with =
	if strings.HasPrefix(arg, "--separator=") {
		flags.separator = arg[len("--separator="):]
		return true, -1
	}
	if strings.HasPrefix(arg, "--format=") {
		flags.format = arg[len("--format="):]
		return true, -1
	}
	// Merged short form: -sSTRING, -fFORMAT
	if len(arg) > 2 && arg[0] == '-' && (arg[1] == 's' || arg[1] == 'f') {
		if arg[1] == 's' {
			flags.separator = arg[2:]
		} else {
			flags.format = arg[2:]
		}
		return true, -1
	}
	// Separate value form: -s STRING, -f FORMAT, --separator STRING, --format FORMAT
	return handleSplitValueFlag(args, i, flags, stderr)
}

// handleSplitValueFlag handles -s/-f/--separator/--format with separate value arg.
func handleSplitValueFlag(args []string, i *int, flags *parsedFlags, stderr io.Writer) (bool, int) {
	arg := args[*i]
	isSep := arg == "-s" || arg == "--separator"
	isFmt := arg == "-f" || arg == "--format"
	if !isSep && !isFmt {
		return false, -1
	}
	if *i+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option '%s' requires an argument\n",
			progName, arg)
		printTryHelp(stderr)
		return true, 1
	}
	*i++
	if isSep {
		flags.separator = args[*i]
	} else {
		flags.format = args[*i]
	}
	return true, -1
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
func buildOpts(numStrs []string, flags parsedFlags) (seqOpts, error) {
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
		format:    resolveFormat(numStrs, flags, first, last),
		separator: flags.separator,
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

// resolveFormat determines the output format based on flags and arguments.
// R3.4: -f overrides -w.
func resolveFormat(numStrs []string, flags parsedFlags, first, last float64) string {
	if flags.format != "" {
		return translateFormat(flags.format)
	}
	prec := maxPrecision(numStrs)
	defaultFmt := fmt.Sprintf("%%.%df", prec)
	if !flags.equalWidth {
		return defaultFmt
	}
	return equalWidthFormat(first, last, prec)
}

// maxPrecision returns the maximum number of decimal places across args.
// R2.3: determines minimum precision for floating-point sequences.
func maxPrecision(args []string) int {
	maxPrec := 0
	for _, arg := range args {
		if prec := decimalPlaces(arg); prec > maxPrec {
			maxPrec = prec
		}
	}
	return maxPrec
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

// equalWidthFormat computes a zero-padded format matching the widest endpoint.
// R3.3: width determined by widest of FIRST and LAST.
func equalWidthFormat(first, last float64, prec int) string {
	defaultFmt := fmt.Sprintf("%%.%df", prec)
	firstStr := fmt.Sprintf(defaultFmt, first)
	lastStr := fmt.Sprintf(defaultFmt, last)
	width := max(len(firstStr), len(lastStr))
	return fmt.Sprintf("%%0%d.%df", width, prec)
}

// validateFormat checks that format has exactly one valid floating-point
// conversion specifier from the set {a,e,f,g,A,E,F,G}. R3.1, R3.2.
func validateFormat(format string) error {
	count := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		if i >= len(format) {
			break
		}
		if format[i] == '%' {
			continue
		}
		i = skipFormatMods(format, i)
		if i >= len(format) {
			break
		}
		if !isValidSpecifier(format[i]) {
			return fmt.Errorf("format '%s' has unknown %%%c directive",
				format, format[i])
		}
		count++
	}
	return checkSpecifierCount(format, count)
}

// checkSpecifierCount validates the specifier count in a format string.
func checkSpecifierCount(format string, count int) error {
	if count == 0 {
		return fmt.Errorf("format '%s' has no %% directive", format)
	}
	if count > 1 {
		return fmt.Errorf("format '%s' has too many %% directives", format)
	}
	return nil
}

// skipFormatMods advances past flags (-+#0 space), width, and precision.
func skipFormatMods(format string, i int) int {
	for i < len(format) && strings.ContainsRune("-+#0 ", rune(format[i])) {
		i++
	}
	for i < len(format) && format[i] >= '0' && format[i] <= '9' {
		i++
	}
	if i < len(format) && format[i] == '.' {
		i++
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
	}
	return i
}

// isValidSpecifier returns true if ch is a valid seq format specifier.
func isValidSpecifier(ch byte) bool {
	return strings.ContainsRune("aefgAEFG", rune(ch))
}

// translateFormat converts %a/%A to %x/%X for Go's fmt package.
func translateFormat(format string) string {
	result := []byte(format)
	for i := 0; i < len(result); i++ {
		if result[i] != '%' {
			continue
		}
		i++
		if i >= len(result) || result[i] == '%' {
			continue
		}
		i = skipFormatMods(string(result), i)
		if i >= len(result) {
			break
		}
		switch result[i] {
		case 'a':
			result[i] = 'x'
		case 'A':
			result[i] = 'X'
		}
	}
	return string(result)
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
	fmt.Fprintln(w, "  -f, --format=FORMAT  use printf style floating-point FORMAT")
	fmt.Fprintln(w, "  -s, --separator=STRING  use STRING to separate numbers (default: \\n)")
	fmt.Fprintln(w, "  -w, --equal-width    equalize width by padding with leading zeroes")
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
