// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd019-seq: Print a Sequence of Numbers.
// Covers R1.1-R1.5 (argument forms, sequence generation, zero step),
// R2.1 (default newline separator), and R3.1 (-f/--format).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds parsed flag state for seq.
type config struct {
	format    string // -f/--format printf-style format
	separator string // R2.1: default newline
}

func main() {
	// D1: SIGPIPE handler for piped output.
	sys.InstallSIGPIPEHandler()

	cfg, positional, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg, positional))
}

// run parses positional arguments into FIRST, INCREMENT, LAST and generates the sequence.
func run(cfg config, positional []string) int {
	first, incr, last, err := parsePositional(positional)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seq: %v\n", err)
		return 1
	}

	// R1.5: STEP must not be zero.
	if incr == 0 {
		fmt.Fprintf(os.Stderr, "seq: invalid Zero increment value: '0'\n")
		return 1
	}

	fmtStr := cfg.format
	if fmtStr == "" {
		fmtStr = defaultFormat(positional)
	}

	return generate(first, incr, last, fmtStr, cfg.separator)
}

// parsePositional interprets 1, 2, or 3 positional args as FIRST, INCREMENT, LAST.
// D2: 1 arg = LAST, 2 args = FIRST LAST, 3 args = FIRST INCREMENT LAST.
func parsePositional(args []string) (first, incr, last float64, err error) {
	switch len(args) {
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
		incr, err = parseNumber(args[1])
		if err != nil {
			return 0, 0, 0, err
		}
		last, err = parseNumber(args[2])
		return first, incr, last, err
	default:
		return 0, 0, 0, fmt.Errorf("missing operand")
	}
}

// parseNumber parses a string as a float64.
// D3: uses strconv.ParseFloat to handle both integers and floating-point.
func parseNumber(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) {
		return 0, fmt.Errorf("invalid floating point argument: '%s'", s)
	}
	return v, nil
}

// generate outputs the sequence and returns the exit code.
func generate(first, incr, last float64, fmtStr, sep string) int {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush

	printed := false
	for i := first; shouldContinue(i, last, incr); i += incr {
		if printed {
			fmt.Fprint(w, sep)
		}
		fmt.Fprintf(w, fmtStr, i)
		printed = true
	}

	if printed {
		fmt.Fprintln(w)
	}
	return 0
}

// shouldContinue returns true if the current value is within bounds.
// R1.2: stops when next value would exceed LAST (positive) or fall below LAST (negative).
func shouldContinue(current, last, incr float64) bool {
	if incr > 0 {
		return current <= last+incr*1e-10
	}
	return current >= last+incr*1e-10
}

// defaultFormat computes the printf format string from the input arguments.
// D4: auto-detects precision from input decimal places.
// R2.3: integer sequences produce integers; float sequences use minimum precision.
func defaultFormat(args []string) string {
	maxPrec := 0
	for _, a := range args {
		p := decimalPrecision(a)
		if p > maxPrec {
			maxPrec = p
		}
	}
	if maxPrec == 0 {
		return "%g"
	}
	return fmt.Sprintf("%%.%df", maxPrec)
}

// decimalPrecision returns the number of decimal places in a numeric string.
func decimalPrecision(s string) int {
	// Strip leading sign.
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}

// parseArgs processes command-line flags and returns configuration plus positional args.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (cfg config, positional []string, exit int) {
	cfg.separator = "\n"
	exit = -1

	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			positional = append(positional, args[i+1:]...)
			return
		}
		exit = parseOneArg(args, &i, &cfg, &positional)
		if exit >= 0 {
			return config{}, nil, exit
		}
	}
	return
}

// parseOneArg handles a single argument. Returns exit code (-1 to continue).
func parseOneArg(args []string, i *int, cfg *config, positional *[]string) int {
	arg := args[*i]
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "-f" || arg == "--format":
		return consumeFormat(args, i, cfg)
	case strings.HasPrefix(arg, "-f"):
		cfg.format = arg[2:]
	case strings.HasPrefix(arg, "--format="):
		cfg.format = arg[len("--format="):]
	case arg == "-s" || arg == "--separator":
		return consumeSeparator(args, i, cfg)
	case strings.HasPrefix(arg, "-s"):
		cfg.separator = arg[2:]
	case strings.HasPrefix(arg, "--separator="):
		cfg.separator = arg[len("--separator="):]
	case arg == "-w" || arg == "--equal-width":
		// -w is noted but not in scope for this task (R3.3)
	default:
		*positional = append(*positional, arg)
	}
	return -1
}

// consumeFormat reads the next argument as a format string.
func consumeFormat(args []string, i *int, cfg *config) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "seq: option requires an argument -- 'f'\n")
		return 1
	}
	*i++
	cfg.format = args[*i]
	return -1
}

// consumeSeparator reads the next argument as a separator string.
func consumeSeparator(args []string, i *int, cfg *config) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "seq: option requires an argument -- 's'\n")
		return 1
	}
	*i++
	cfg.separator = args[*i]
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := os.Stdout.WriteString(`Usage: seq [OPTION]... LAST
  or:  seq [OPTION]... FIRST LAST
  or:  seq [OPTION]... FIRST INCREMENT LAST
Print numbers from FIRST to LAST, in steps of INCREMENT.

  -f, --format=FORMAT      use printf style floating-point FORMAT
  -s, --separator=STRING   use STRING to separate numbers (default: \n)
  -w, --equal-width        equalize width by padding with leading zeroes
      --help     display this help and exit
      --version  output version information and exit

If FIRST or INCREMENT is omitted, it defaults to 1.  That is, an
omitted INCREMENT defaults to 1 even when LAST is smaller than FIRST.
The sequence of numbers ends when the sum of the current number and
INCREMENT would become greater than LAST.
FIRST, INCREMENT, and LAST are interpreted as floating point values.
INCREMENT is usually positive if FIRST is smaller than LAST, and
INCREMENT is usually negative if FIRST is greater than LAST.
FORMAT must be suitable for printing one argument of type 'double';
it defaults to %.PRECf if FIRST, INCREMENT, and LAST are all fixed point
decimal numbers with maximum precision PREC, and to %g otherwise.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "seq (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
