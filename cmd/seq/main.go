// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd019-seq R1.1-R1.5, R2.1-R2.3, R3.1-R3.2:
// cmd/seq prints a sequence of numbers. Supports three argument forms:
// seq LAST (FIRST=1, STEP=1), seq FIRST LAST (STEP=1), and seq FIRST STEP LAST.
// All arguments are floating-point. Supports -f for printf-style format
// strings and -s for custom separators. Output formatting matches GNU seq
// precision based on the textual representation of input arguments.
// Installs SIGPIPE handler per ARCHITECTURE.yaml.
package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU seq format.
const progName = "seq"

// seqOptions holds parsed command-line flags for seq.
type seqOptions struct {
	// format is the printf-style format string from -f/--format.
	format string
	// separator is the string printed between numbers (default "\n"). R2.2.
	separator string
	// hasFormat is true when -f/--format was provided.
	hasFormat bool
	// positional contains the numeric arguments (FIRST, STEP, LAST).
	positional []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])

	if len(opts.positional) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
	if len(opts.positional) > 3 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, opts.positional[3])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// R3.1/R3.2: Validate format string if provided.
	if opts.hasFormat {
		if err := validateFormat(opts.format); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			os.Exit(1)
		}
	}

	var firstStr, stepStr, lastStr string
	switch len(opts.positional) {
	case 1:
		// R1.1: seq LAST — FIRST=1, STEP=1.
		firstStr, stepStr, lastStr = "1", "1", opts.positional[0]
	case 2:
		// R1.1: seq FIRST LAST — STEP=1.
		firstStr, stepStr, lastStr = opts.positional[0], "1", opts.positional[1]
	case 3:
		// R1.1: seq FIRST STEP LAST.
		firstStr, stepStr, lastStr = opts.positional[0], opts.positional[1], opts.positional[2]
	}

	first, err := strconv.ParseFloat(firstStr, 64)
	if err != nil || math.IsNaN(first) {
		fmt.Fprintf(os.Stderr, "%s: invalid floating point argument: '%s'\n", progName, firstStr)
		os.Exit(1)
	}
	step, err := strconv.ParseFloat(stepStr, 64)
	if err != nil || math.IsNaN(step) {
		fmt.Fprintf(os.Stderr, "%s: invalid floating point argument: '%s'\n", progName, stepStr)
		os.Exit(1)
	}
	last, err := strconv.ParseFloat(lastStr, 64)
	if err != nil || math.IsNaN(last) {
		fmt.Fprintf(os.Stderr, "%s: invalid floating point argument: '%s'\n", progName, lastStr)
		os.Exit(1)
	}

	// R1.5: zero step is an error.
	if step == 0 {
		fmt.Fprintf(os.Stderr, "%s: invalid Zero increment value: '%s'\n", progName, stepStr)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// Determine number formatting function.
	var formatNum func(float64) string
	if opts.hasFormat {
		// R3.1: Apply user-provided printf-style format string.
		formatNum = func(val float64) string {
			return fmt.Sprintf(opts.format, val)
		}
	} else {
		// R2.3: Default format based on input argument precision.
		prec := computePrecision(opts.positional)
		formatNum = func(val float64) string {
			return formatNumber(val, prec)
		}
	}

	// R1.4: empty sequence — exit 0 with no output.
	if step > 0 && first > last {
		os.Exit(0)
	}
	if step < 0 && first < last {
		os.Exit(0)
	}

	// Generate and print the sequence.
	w := bufio.NewWriter(os.Stdout)
	printed := false

	for i := range math.MaxInt {
		// R1.2: compute value from initial first to avoid accumulated error.
		val := first + float64(i)*step
		if !inRange(val, last, step) {
			break
		}
		if printed {
			// R2.2: use configured separator (default "\n").
			if _, err := w.WriteString(opts.separator); err != nil {
				break
			}
		}
		if _, err := w.WriteString(formatNum(val)); err != nil {
			break
		}
		printed = true
	}
	if printed {
		w.WriteString("\n") // best-effort trailing newline
	}
	w.Flush() // best-effort flush
}

// inRange returns true if val has not passed last for the given step direction.
// A small tolerance proportional to step handles floating-point rounding.
func inRange(val, last, step float64) bool {
	if step > 0 {
		return val <= last+step*1e-10
	}
	return val >= last+step*1e-10
}

// parseArgs processes command-line arguments, extracting flags and positional args.
// Handles --help, --version, -f/--format, -s/--separator, and -- (end of flags).
func parseArgs(args []string) seqOptions {
	opts := seqOptions{separator: "\n"}
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags {
			opts.positional = append(opts.positional, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--help":
				printHelp()
				os.Exit(0)
			case arg == "--version":
				fmt.Fprintf(os.Stdout, "%s (%s) %s\n", progName, "go-unix-utils", version.Version)
				os.Exit(0)
			case arg == "--format":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--format' requires an argument\n", progName)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
					os.Exit(1)
				}
				opts.format = args[i]
				opts.hasFormat = true
			case strings.HasPrefix(arg, "--format="):
				opts.format = arg[len("--format="):]
				opts.hasFormat = true
			case arg == "--separator":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--separator' requires an argument\n", progName)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
					os.Exit(1)
				}
				opts.separator = args[i]
			case strings.HasPrefix(arg, "--separator="):
				opts.separator = arg[len("--separator="):]
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
			continue
		}

		// Short options: starts with '-' and is not a numeric argument.
		if len(arg) >= 2 && arg[0] == '-' && !isNumericArg(arg) {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'f':
					if j+1 < len(arg) {
						opts.format = arg[j+1:]
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'f'\n", progName)
							fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
							os.Exit(1)
						}
						opts.format = args[i]
					}
					opts.hasFormat = true
					j = len(arg)
				case 's':
					if j+1 < len(arg) {
						opts.separator = arg[j+1:]
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 's'\n", progName)
							fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
							os.Exit(1)
						}
						opts.separator = args[i]
					}
					j = len(arg)
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
					os.Exit(1)
				}
			}
			continue
		}

		opts.positional = append(opts.positional, arg)
	}

	return opts
}

// isNumericArg returns true if s looks like a numeric argument (starts with
// '-' followed by a digit or decimal point, e.g., "-1", "-.5", "-1e3").
func isNumericArg(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	return (s[1] >= '0' && s[1] <= '9') || s[1] == '.'
}

// validateFormat checks that format contains exactly one floating-point
// conversion specifier (%a, %e, %f, %g or uppercase variants). R3.1/R3.2.
func validateFormat(format string) error {
	count := 0
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			i++
			continue
		}
		i++ // skip '%'
		if i >= len(format) {
			break
		}
		if format[i] == '%' {
			i++ // literal '%%'
			continue
		}
		// Skip flags: -, +, #, 0, space.
		for i < len(format) && strings.IndexByte("-+#0 ", format[i]) >= 0 {
			i++
		}
		// Skip width.
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			i++
		}
		// Skip precision.
		if i < len(format) && format[i] == '.' {
			i++
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
		}
		if i >= len(format) {
			break
		}
		conv := format[i]
		i++
		if strings.IndexByte("aefgAEFG", conv) >= 0 {
			count++
		} else {
			return fmt.Errorf("format '%s' has unknown %%%c directive", format, conv)
		}
	}
	if count == 0 {
		return fmt.Errorf("format '%s' has no %% directive", format)
	}
	if count > 1 {
		return fmt.Errorf("format '%s' has too many %% directives", format)
	}
	return nil
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Fprintf(os.Stdout,
		"Usage: %s [OPTION]... LAST\n"+
			"  or:  %s [OPTION]... FIRST LAST\n"+
			"  or:  %s [OPTION]... FIRST INCREMENT LAST\n"+
			"Print numbers from FIRST to LAST, in steps of INCREMENT.\n\n"+
			"Mandatory arguments to long options are mandatory for short options too.\n"+
			"  -f, --format=FORMAT  use printf style floating-point FORMAT\n"+
			"  -s, --separator=STRING  use STRING to separate numbers (default: \\n)\n"+
			"      --help     display this help and exit\n"+
			"      --version  output version information and exit\n",
		progName, progName, progName,
	)
}

// computePrecision determines the output decimal precision from the textual
// representation of user-provided arguments. R2.3: integer sequences produce
// integer output; floating-point sequences use the maximum input precision.
func computePrecision(args []string) int {
	maxPrec := 0
	for _, a := range args {
		p := decimalPrecision(a)
		if p > maxPrec {
			maxPrec = p
		}
	}
	return maxPrec
}

// decimalPrecision counts the number of digits after the decimal point in s.
// Returns 0 if s has no decimal point.
func decimalPrecision(s string) int {
	// Strip leading sign.
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	_, after, found := strings.Cut(s, ".")
	if !found {
		return 0
	}
	// Handle scientific notation: count only digits before 'e'/'E'.
	if eIdx := strings.IndexAny(after, "eE"); eIdx >= 0 {
		return eIdx
	}
	return len(after)
}

// formatNumber formats val with the given decimal precision. Precision 0
// produces integer output (no decimal point).
func formatNumber(val float64, prec int) string {
	s := fmt.Sprintf("%.*f", prec, val)
	// Avoid "-0" output for negative zero.
	if prec == 0 && s == "-0" {
		return "0"
	}
	return s
}
