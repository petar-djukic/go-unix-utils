// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/seq: print a sequence of numbers.
// Implements srd019-seq R1.1-R1.5, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "seq"

// allowedConversions lists the valid floating-point conversion specifiers.
const allowedConversions = "aefgAEFG"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// seqConfig holds parsed option flags for the seq command.
type seqConfig struct {
	separator  string
	format     string
	equalWidth bool
	args       []string
}

// run executes the seq logic and returns the exit code.
// R1.1-R1.5, R2.1-R2.4, R3.1-R3.4.
func run(args []string) int {
	cfg, code, done := parseFlags(args)
	if done {
		return code
	}
	return runWithConfig(cfg)
}

// runWithConfig validates options, parses args, and prints the sequence.
func runWithConfig(cfg seqConfig) int {
	if cfg.format != "" && cfg.equalWidth {
		fmt.Fprintf(os.Stderr, "%s: format string may not be specified"+
			" when printing equal width strings\n", progName)
		return 1
	}
	if cfg.format != "" {
		if err := validateFormat(cfg.format); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			return 1
		}
	}
	first, incr, last, prec, err := parseArgs(cfg.args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	format := resolveFormat(cfg, prec, first, last)
	printSequence(first, incr, last, format, cfg.separator)
	return 0
}

// parseFlags extracts option flags and returns remaining positional args.
// R2.2: -s/--separator, R3.1: -f/--format, R3.3: -w/--equal-width.
func parseFlags(args []string) (seqConfig, int, bool) {
	cfg := seqConfig{separator: "\n"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			return cfg, 0, false
		}
		if a == "--version" {
			fmt.Println("seq (go-unix-utils)")
			return cfg, 0, true
		}
		if a == "--help" {
			printHelp()
			return cfg, 0, true
		}
		if a == "--equal-width" || a == "-w" {
			cfg.equalWidth = true
			continue
		}
		handled, skip, hadErr := parseFlagWithArg(&cfg, args, i, a)
		if hadErr {
			return cfg, 1, true
		}
		if handled {
			i += skip
			continue
		}
		cfg.args = append(cfg.args, a)
	}
	return cfg, 0, false
}

// parseFlagWithArg handles flags that take a value argument.
// Returns (handled, extra args consumed, had error).
func parseFlagWithArg(cfg *seqConfig, args []string, i int, a string) (bool, int, bool) {
	switch {
	case a == "-s" || a == "--separator":
		if i+1 >= len(args) {
			printOptErr(a)
			return true, 0, true
		}
		cfg.separator = args[i+1]
		return true, 1, false
	case strings.HasPrefix(a, "--separator="):
		cfg.separator = a[len("--separator="):]
		return true, 0, false
	case a == "-f" || a == "--format":
		if i+1 >= len(args) {
			printOptErr(a)
			return true, 0, true
		}
		cfg.format = args[i+1]
		return true, 1, false
	case strings.HasPrefix(a, "--format="):
		cfg.format = a[len("--format="):]
		return true, 0, false
	}
	return false, 0, false
}

// printOptErr prints a missing-argument error for the given option.
func printOptErr(opt string) {
	fmt.Fprintf(os.Stderr, "%s: option '%s' requires an argument\n", progName, opt)
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: seq [OPTION]... LAST
  or:  seq [OPTION]... FIRST LAST
  or:  seq [OPTION]... FIRST INCREMENT LAST
Print numbers from FIRST to LAST, in steps of INCREMENT.

  -f, --format=FORMAT  use printf style floating-point FORMAT
  -s, --separator=STRING  use STRING to separate numbers (default: \n)
  -w, --equal-width    equalize width by padding with leading zeroes
      --help     display this help and exit
      --version  output version information and exit
`)
}

// validateFormat checks that the format string contains exactly one
// valid floating-point conversion specifier.
// R3.2: reject zero, multiple, or non-float conversions.
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
			continue // literal %%
		}
		// R3.1: skip flags, width, precision
		i = skipFlagsWidthPrec(format, i)
		if i >= len(format) {
			return fmtErr(format)
		}
		if !strings.ContainsRune(allowedConversions, rune(format[i])) {
			return fmtErr(format)
		}
		count++
	}
	if count != 1 {
		return fmtErr(format)
	}
	return nil
}

// skipFlagsWidthPrec advances past flags, width, and precision fields.
func skipFlagsWidthPrec(format string, i int) int {
	for i < len(format) && strings.ContainsRune("-+ #0", rune(format[i])) {
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

// fmtErr returns a format-error for an invalid format string.
func fmtErr(format string) error {
	return fmt.Errorf("format '%s' has no valid floating point conversion", format)
}

// resolveFormat determines the output format string.
// R3.4: -f takes precedence over -w.
func resolveFormat(cfg seqConfig, prec int, first, last float64) string {
	if cfg.format != "" {
		return cfg.format
	}
	if cfg.equalWidth {
		return equalWidthFormat(prec, first, last)
	}
	return buildFormat(prec)
}

// equalWidthFormat computes a zero-padded format string for -w.
// R3.3: width is the widest of FIRST and LAST with default format.
func equalWidthFormat(prec int, first, last float64) string {
	dfmt := buildFormat(prec)
	w1 := len(fmt.Sprintf(dfmt, first))
	w2 := len(fmt.Sprintf(dfmt, last))
	width := maxInt(w1, w2)
	return fmt.Sprintf("%%0%d.%df", width, prec)
}

// parseArgs dispatches to the correct argument form parser.
// R1.1: one-arg (LAST), two-arg (FIRST LAST), three-arg (FIRST STEP LAST).
func parseArgs(args []string) (float64, float64, float64, int, error) {
	switch len(args) {
	case 0:
		return 0, 0, 0, 0, fmt.Errorf("missing operand")
	case 1:
		return parseOneArg(args)
	case 2:
		return parseTwoArgs(args)
	case 3:
		return parseThreeArgs(args)
	default:
		return 0, 0, 0, 0, fmt.Errorf("extra operand '%s'", args[3])
	}
}

// parseOneArg handles seq LAST (FIRST=1, STEP=1).
func parseOneArg(args []string) (float64, float64, float64, int, error) {
	last, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return 0, 0, 0, 0, invalidArg(args[0])
	}
	return 1, 1, last, decimalPrecision(args[0]), nil
}

// parseTwoArgs handles seq FIRST LAST (STEP=1).
func parseTwoArgs(args []string) (float64, float64, float64, int, error) {
	first, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return 0, 0, 0, 0, invalidArg(args[0])
	}
	last, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return 0, 0, 0, 0, invalidArg(args[1])
	}
	prec := maxInt(decimalPrecision(args[0]), decimalPrecision(args[1]))
	return first, 1, last, prec, nil
}

// parseThreeArgs handles seq FIRST STEP LAST.
// R1.5: zero increment produces an error.
func parseThreeArgs(args []string) (float64, float64, float64, int, error) {
	first, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return 0, 0, 0, 0, invalidArg(args[0])
	}
	incr, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return 0, 0, 0, 0, invalidArg(args[1])
	}
	if incr == 0 {
		return 0, 0, 0, 0, zeroIncrErr(args[1])
	}
	last, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return 0, 0, 0, 0, invalidArg(args[2])
	}
	prec := maxInt(decimalPrecision(args[0]),
		maxInt(decimalPrecision(args[1]), decimalPrecision(args[2])))
	return first, incr, last, prec, nil
}

// invalidArg returns a formatted error for a non-numeric argument.
func invalidArg(s string) error {
	return fmt.Errorf("invalid floating point argument: %s", s)
}

// zeroIncrErr returns a formatted error for a zero increment.
func zeroIncrErr(s string) error {
	return fmt.Errorf("invalid Zero increment value: '%s'", s)
}

// decimalPrecision returns the number of digits after the decimal point.
// R2.3: precision of input arguments determines output format.
func decimalPrecision(s string) int {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return 0
	}
	return len(s) - dot - 1
}

// maxInt returns the larger of two integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// buildFormat creates a printf format string from the precision.
// R2.3: integers use no decimal point; floats use input precision.
func buildFormat(prec int) string {
	return fmt.Sprintf("%%.%df", prec)
}

// printSequence outputs the number sequence to stdout.
// R1.2: generates from FIRST to LAST by STEP.
// R1.4: produces no output when the sequence is empty.
// R2.1: separator between numbers, trailing newline after last.
func printSequence(first, incr, last float64, format, sep string) {
	n := numSteps(first, incr, last)
	if n < 0 {
		return
	}
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush; SIGPIPE handler manages pipe errors
	for i := 0; i <= n; i++ {
		if i > 0 {
			w.WriteString(sep) //nolint:errcheck // buffered; checked at flush
		}
		val := first + float64(i)*incr
		fmt.Fprintf(w, format, val)
	}
	w.WriteByte('\n') //nolint:errcheck // trailing newline per R2.1
}

// numSteps computes how many values to print.
// Uses first + i*incr to avoid cumulative floating-point drift.
func numSteps(first, incr, last float64) int {
	if incr == 0 {
		return -1
	}
	if incr > 0 && first > last {
		return -1
	}
	if incr < 0 && first < last {
		return -1
	}
	n := (last - first) / incr
	return int(math.Floor(n + 1e-10))
}
