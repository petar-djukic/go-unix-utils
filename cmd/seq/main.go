// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/seq: print a sequence of numbers.
// Implements srd019-seq R1.1-R1.5, R2.1-R2.3.
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

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// seqConfig holds parsed option flags for the seq command.
type seqConfig struct {
	separator string
	args      []string
}

// run executes the seq logic and returns the exit code.
// R1.1-R1.5, R2.1-R2.3: parse flags and arguments, generate sequence.
func run(args []string) int {
	cfg, code, done := parseFlags(args)
	if done {
		return code
	}
	first, incr, last, prec, err := parseArgs(cfg.args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	format := buildFormat(prec)
	printSequence(first, incr, last, format, cfg.separator)
	return 0
}

// parseFlags extracts option flags and returns remaining positional args.
// R2.2: -s/--separator, R2.2/R2.3: --version/--help.
func parseFlags(args []string) (seqConfig, int, bool) {
	cfg := seqConfig{separator: "\n"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			return cfg, 0, false
		}
		switch {
		case a == "--version":
			fmt.Println("seq (go-unix-utils)")
			return cfg, 0, true
		case a == "--help":
			printHelp()
			return cfg, 0, true
		case a == "--separator":
			if i+1 >= len(args) {
				printOptErr("--separator")
				return cfg, 1, true
			}
			i++
			cfg.separator = args[i]
		case strings.HasPrefix(a, "--separator="):
			cfg.separator = a[len("--separator="):]
		case a == "-s":
			if i+1 >= len(args) {
				printOptErr("-s")
				return cfg, 1, true
			}
			i++
			cfg.separator = args[i]
		default:
			cfg.args = append(cfg.args, a)
		}
	}
	return cfg, 0, false
}

// printOptErr prints a missing-argument error for the given option.
func printOptErr(opt string) {
	fmt.Fprintf(os.Stderr, "%s: option '%s' requires an argument\n", progName, opt)
}

// printHelp writes usage information to stdout.
// R2.3: --help prints usage and exits 0.
func printHelp() {
	fmt.Print(`Usage: seq [OPTION]... LAST
  or:  seq [OPTION]... FIRST LAST
  or:  seq [OPTION]... FIRST INCREMENT LAST
Print numbers from FIRST to LAST, in steps of INCREMENT.

  -s, --separator=STRING  use STRING to separate numbers (default: \n)
      --help     display this help and exit
      --version  output version information and exit
`)
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
