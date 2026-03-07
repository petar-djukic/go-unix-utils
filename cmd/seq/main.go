// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU seq: print a sequence of numbers.
// Implements prd019-seq R1-R4.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R4: Handle --help and --version before flag parsing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Println("Usage: seq [OPTION]... LAST")
			fmt.Println("  or:  seq [OPTION]... FIRST LAST")
			fmt.Println("  or:  seq [OPTION]... FIRST INCREMENT LAST")
			fmt.Println("Print numbers from FIRST to LAST, in steps of INCREMENT.")
			fmt.Println()
			fmt.Println("Mandatory arguments to long options are mandatory for short options too.")
			fmt.Println("  -f, --format=FORMAT      use printf style floating-point FORMAT")
			fmt.Println("  -s, --separator=STRING   use STRING to separate numbers (default: \\n)")
			fmt.Println("  -w, --equal-width         equalize width by padding with leading zeroes")
			fmt.Println("      --help        display this help and exit")
			fmt.Println("      --version     output version information and exit")
			os.Exit(0)
		case "--version":
			fmt.Println("seq (go-unix-utils) dev")
			os.Exit(0)
		}
	}

	// Parse flags manually, matching cmd/head pattern.
	formatStr := ""
	separator := "\n"
	equalWidth := false
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}

		// Long options with =.
		if strings.HasPrefix(arg, "--format=") {
			formatStr = arg[len("--format="):]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--separator=") {
			separator = arg[len("--separator="):]
			i++
			continue
		}
		if arg == "--format" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "seq: option '%s' requires an argument\n", arg)
				os.Exit(1)
			}
			formatStr = args[i]
			i++
			continue
		}
		if arg == "--separator" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "seq: option '%s' requires an argument\n", arg)
				os.Exit(1)
			}
			separator = args[i]
			i++
			continue
		}
		if arg == "--equal-width" {
			equalWidth = true
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			// Check if this is a negative number (starts with digit or dot after -).
			if arg[1] >= '0' && arg[1] <= '9' || arg[1] == '.' {
				// Negative number, treat as positional.
				positional = append(positional, arg)
				i++
				continue
			}

			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'f':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "seq: option requires an argument -- 'f'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					formatStr = val
					j = len(arg)
				case 's':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "seq: option requires an argument -- 's'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					separator = val
					j = len(arg)
				case 'w':
					equalWidth = true
					j++
				default:
					fmt.Fprintf(os.Stderr, "seq: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			i++
			continue
		}

		// Not a flag; treat as positional.
		positional = append(positional, arg)
		i++
	}

	// Remaining args after -- are positional.
	positional = append(positional, args[i:]...)

	// R1.1: Parse argument forms.
	var firstStr, stepStr, lastStr string
	switch len(positional) {
	case 1:
		firstStr, stepStr, lastStr = "1", "1", positional[0]
	case 2:
		firstStr, stepStr, lastStr = positional[0], "1", positional[1]
	case 3:
		firstStr, stepStr, lastStr = positional[0], positional[1], positional[2]
	default:
		if len(positional) == 0 {
			fmt.Fprintf(os.Stderr, "seq: missing operand\n")
		} else {
			fmt.Fprintf(os.Stderr, "seq: extra operand '%s'\n", positional[3])
		}
		os.Exit(1)
	}

	first, err := strconv.ParseFloat(firstStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seq: invalid floating point argument: '%s'\n", firstStr)
		os.Exit(1)
	}
	step, err := strconv.ParseFloat(stepStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seq: invalid floating point argument: '%s'\n", stepStr)
		os.Exit(1)
	}
	last, err := strconv.ParseFloat(lastStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "seq: invalid floating point argument: '%s'\n", lastStr)
		os.Exit(1)
	}

	// R1.5: Zero step is an error.
	if step == 0 {
		fmt.Fprintf(os.Stderr, "seq: invalid Zero increment value: '%s'\n", stepStr)
		os.Exit(1)
	}

	// When only two args given and FIRST > LAST, GNU seq uses default step=1
	// (no auto-negative). Already handled: step defaults to "1".

	// Validate format string if provided. R3.1, R3.2.
	if formatStr != "" {
		if err := validateFormat(formatStr); err != nil {
			fmt.Fprintf(os.Stderr, "seq: format '%s' has %s\n", formatStr, err.Error())
			os.Exit(1)
		}
	}

	// R2.3: Determine default format based on whether inputs are integers or floats.
	defaultFmt := defaultFormat(firstStr, stepStr, lastStr)

	// R3.3: Equal-width padding.
	if equalWidth && formatStr == "" {
		// Determine the width of the widest endpoint.
		w := equalWidthFormat(first, last, defaultFmt)
		formatStr = w
	}

	// Use the format string or default.
	fmtStr := defaultFmt
	if formatStr != "" {
		fmtStr = formatStr
	}

	// R1.2, R1.4: Generate the sequence.
	printed := false
	val := first
	for {
		if step > 0 && val > last+step*1e-10 {
			break
		}
		if step < 0 && val < last+step*1e-10 {
			break
		}

		if printed {
			fmt.Print(separator)
		}
		fmt.Print(formatNumber(val, fmtStr))
		printed = true

		val += step

		// Avoid infinite loops due to floating-point: check if we passed the endpoint.
		if step > 0 && val > last+step*1e-10 {
			break
		}
		if step < 0 && val < last+step*1e-10 {
			break
		}
	}

	if printed {
		fmt.Println()
	}
}

// defaultFormat returns the printf format string to use when no -f is given.
// R2.3: integer inputs produce integer output; float inputs use the max
// precision from the input arguments.
func defaultFormat(firstStr, stepStr, lastStr string) string {
	p1 := decimalPrecision(firstStr)
	p2 := decimalPrecision(stepStr)
	p3 := decimalPrecision(lastStr)

	maxPrec := max(p1, p2, p3)

	if maxPrec == 0 {
		return "%g"
	}
	return fmt.Sprintf("%%.%df", maxPrec)
}

// decimalPrecision returns the number of digits after the decimal point in s.
// Returns 0 if there is no decimal point.
func decimalPrecision(s string) int {
	// Strip leading minus.
	s = strings.TrimPrefix(s, "-")
	idx := strings.Index(s, ".")
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}

// equalWidthFormat returns a printf format string that zero-pads to the width
// of the widest endpoint. R3.3.
func equalWidthFormat(first, last float64, defaultFmt string) string {
	s1 := formatNumber(first, defaultFmt)
	s2 := formatNumber(last, defaultFmt)
	width := max(len(s1), len(s2))

	// Check if the default format has precision.
	prec := 0
	if strings.Contains(defaultFmt, ".") {
		// Extract precision from format like "%.2f"
		fmt.Sscanf(defaultFmt, "%%.%df", &prec)
	}

	if prec > 0 {
		return fmt.Sprintf("%%0%d.%df", width, prec)
	}
	return fmt.Sprintf("%%0%dg", width)
}

// formatNumber formats a number using the given printf format string.
func formatNumber(val float64, fmtStr string) string {
	// For %g format with integer values, ensure we output integers without decimal.
	if fmtStr == "%g" {
		if val == math.Trunc(val) && !math.IsInf(val, 0) && !math.IsNaN(val) {
			return strconv.FormatInt(int64(val), 10)
		}
		return fmt.Sprintf("%g", val)
	}
	return fmt.Sprintf(fmtStr, val)
}

// validateFormat checks that the format string contains exactly one valid
// floating-point conversion specifier. R3.1, R3.2.
func validateFormat(format string) error {
	count := 0
	i := 0
	for i < len(format) {
		if format[i] == '%' {
			i++
			if i >= len(format) {
				return fmt.Errorf("no conversion specification in suffix")
			}
			if format[i] == '%' {
				// Literal %%.
				i++
				continue
			}
			// Skip flags: -, +, #, 0, space.
			for i < len(format) && (format[i] == '-' || format[i] == '+' || format[i] == '#' || format[i] == '0' || format[i] == ' ') {
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
				return fmt.Errorf("no conversion specification in suffix")
			}
			// Check conversion specifier.
			ch := format[i]
			if ch == 'a' || ch == 'e' || ch == 'f' || ch == 'g' ||
				ch == 'A' || ch == 'E' || ch == 'F' || ch == 'G' {
				count++
				i++
			} else {
				return fmt.Errorf("invalid format specifier: %%%c", ch)
			}
		} else {
			i++
		}
	}

	if count == 0 {
		return fmt.Errorf("no %% directive in format string")
	}
	if count > 1 {
		return fmt.Errorf("too many %% directives in format string")
	}
	return nil
}
