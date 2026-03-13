// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd019-seq R1.1–R1.5, R2.1–R2.4, R3.1–R3.4
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

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// R4.1, R4.2: Handle --version and --help before flag parsing.
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break
		}
		if arg == "--version" {
			fmt.Println("seq (go-unix-utils)")
			os.Exit(0)
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
	}

	// Parse options manually to match GNU seq behavior with intermixed options
	// and positional arguments (e.g., seq -w 1 10, seq -s ', ' 3).
	separator := "\n"
	equalWidth := false
	var formatStr string
	hasFormat := false
	var positional []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if arg == "-w" || arg == "--equal-width" {
			equalWidth = true
		} else if arg == "-s" || arg == "--separator" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "seq: option requires an argument -- 's'\n")
				os.Exit(1)
			}
			separator = args[i]
		} else if strings.HasPrefix(arg, "-s") && !isNumeric(arg) {
			// -sFOO form
			separator = arg[2:]
		} else if strings.HasPrefix(arg, "--separator=") {
			separator = arg[len("--separator="):]
		} else if arg == "-f" || arg == "--format" {
			// R3.1: -f FORMAT form.
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "seq: option requires an argument -- 'f'\n")
				os.Exit(1)
			}
			formatStr = args[i]
			hasFormat = true
		} else if strings.HasPrefix(arg, "-f") && !isNumeric(arg) {
			// R3.1: -fFORMAT combined form.
			formatStr = arg[2:]
			hasFormat = true
		} else if strings.HasPrefix(arg, "--format=") {
			// R3.1: --format=FORMAT long form.
			formatStr = arg[len("--format="):]
			hasFormat = true
		} else {
			positional = append(positional, arg)
		}
	}

	// R3.1, R3.2: Validate format string if provided.
	if hasFormat {
		goFmt, err := parseFormat(formatStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "seq: %v\n", err)
			os.Exit(1)
		}
		formatStr = goFmt
	}

	// R1.1: Parse positional arguments.
	var first, increment, last float64
	var firstStr, incrementStr, lastStr string

	switch len(positional) {
	case 1:
		// seq LAST
		last = parseNumber(positional[0])
		first = 1
		increment = 1
		firstStr = "1"
		incrementStr = "1"
		lastStr = positional[0]
	case 2:
		// seq FIRST LAST
		first = parseNumber(positional[0])
		last = parseNumber(positional[1])
		increment = 1
		firstStr = positional[0]
		incrementStr = "1"
		lastStr = positional[1]
	case 3:
		// seq FIRST INCREMENT LAST
		first = parseNumber(positional[0])
		increment = parseNumber(positional[1])
		last = parseNumber(positional[2])
		firstStr = positional[0]
		incrementStr = positional[1]
		lastStr = positional[2]
	case 0:
		fmt.Fprintf(os.Stderr, "seq: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "seq: extra operand '%s'\n", positional[3])
		fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
		os.Exit(1)
	}

	// R1.5: zero step is an error.
	if increment == 0 {
		fmt.Fprintf(os.Stderr, "seq: invalid Zero increment value: '%s'\n", incrementStr)
		fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
		os.Exit(1)
	}

	// R3.4: -f and -w are mutually exclusive. GNU seq errors when both are given.
	if hasFormat && equalWidth {
		fmt.Fprintf(os.Stderr, "seq: format string may not be specified when printing equal width strings\n")
		fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
		os.Exit(1)
	}

	// Determine output format.
	var format string
	if hasFormat {
		format = formatStr
	} else if equalWidth {
		// R3.3: Equal-width zero padding.
		format = equalWidthFormat(first, last, firstStr, incrementStr, lastStr)
	} else {
		// R2.3, R3.2: Determine output format based on input precision.
		format = defaultFormat(firstStr, incrementStr, lastStr)
	}

	// D3: Compute epsilon for floating-point boundary comparison to avoid
	// skipping the final value due to IEEE 754 rounding.
	maxVal := math.Max(math.Abs(first), math.Max(math.Abs(last), math.Abs(increment)))
	epsilon := maxVal * 1e-12
	if epsilon < 1e-15 {
		epsilon = 1e-15
	}

	w := bufio.NewWriter(os.Stdout)

	// R1.2: Generate the sequence using multiplication to avoid error accumulation.
	printed := false
	for i := 0; ; i++ {
		val := first + float64(i)*increment
		// R1.2: Stop when next value exceeds LAST.
		if increment > 0 && val-last > epsilon {
			break
		}
		if increment < 0 && last-val > epsilon {
			break
		}

		if printed {
			fmt.Fprint(w, separator)
		}
		fmt.Fprintf(w, format, val)
		printed = true
	}

	// R2.1: trailing newline after last number.
	if printed {
		fmt.Fprint(w, "\n")
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "seq: write error: %v\n", err)
		os.Exit(1)
	}
}

// isNumeric returns true if s looks like a numeric argument (starts with a
// digit, '.', or '-' followed by a digit or '.').
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] >= '0' && s[0] <= '9' || s[0] == '.' {
		return true
	}
	if s[0] == '-' && len(s) > 1 && (s[1] >= '0' && s[1] <= '9' || s[1] == '.') {
		return true
	}
	return false
}

// parseNumber parses a string as a float64, exiting with an error on failure.
func parseNumber(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) {
		fmt.Fprintf(os.Stderr, "seq: invalid floating point argument: '%s'\n", s)
		fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
		os.Exit(1)
	}
	return v
}

// parseFormat validates a printf format string per R3.1/R3.2 and converts
// C-style %a/%A to Go-style %x/%X for hex floats. Returns the Go-compatible
// format string or an error if the format is invalid.
func parseFormat(format string) (string, error) {
	result := []byte(format)
	count := 0
	for i := 0; i < len(result); i++ {
		if result[i] != '%' {
			continue
		}
		i++
		if i >= len(result) {
			return "", fmt.Errorf("format '%s' ends in %%", format)
		}
		if result[i] == '%' {
			continue
		}
		// Skip flags: -, +, #, 0, space, ' (thousand separator).
		for i < len(result) && strings.ContainsRune("-+ #0'", rune(result[i])) {
			i++
		}
		// Skip width.
		for i < len(result) && result[i] >= '0' && result[i] <= '9' {
			i++
		}
		// Skip precision.
		if i < len(result) && result[i] == '.' {
			i++
			for i < len(result) && result[i] >= '0' && result[i] <= '9' {
				i++
			}
		}
		if i >= len(result) {
			return "", fmt.Errorf("format '%s' ends in %%", format)
		}
		conv := result[i]
		switch conv {
		case 'a':
			result[i] = 'x' // Go uses %x for hex float.
		case 'A':
			result[i] = 'X' // Go uses %X for hex float.
		case 'e', 'f', 'g', 'E', 'F', 'G':
			// Valid as-is in Go fmt.
		default:
			return "", fmt.Errorf("format '%s' has unknown %%%c directive", format, conv)
		}
		count++
	}
	if count == 0 {
		return "", fmt.Errorf("format '%s' has no %% directive", format)
	}
	if count > 1 {
		return "", fmt.Errorf("format '%s' has too many %% directives", format)
	}
	return string(result), nil
}

// defaultFormat returns a printf format string that preserves the decimal
// precision of the input arguments. R2.3: integer inputs produce integer
// output; floating-point inputs use the maximum precision of all operands.
// R3.2: precision is auto-detected from the operand with the most decimal places.
func defaultFormat(firstStr, incrementStr, lastStr string) string {
	p1 := decimalPrecision(firstStr)
	p2 := decimalPrecision(incrementStr)
	p3 := decimalPrecision(lastStr)
	prec := max(p1, max(p2, p3))
	if prec == 0 {
		return "%g"
	}
	return fmt.Sprintf("%%.%df", prec)
}

// decimalPrecision returns the number of digits after the decimal point in s.
func decimalPrecision(s string) int {
	// Strip leading minus sign.
	s = strings.TrimPrefix(s, "-")
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return 0
	}
	return len(s) - dot - 1
}

// equalWidthFormat returns a zero-padded format string. R3.3: width is
// determined by the widest of FIRST and LAST formatted with the default format.
func equalWidthFormat(first, last float64, firstStr, incrementStr, lastStr string) string {
	base := defaultFormat(firstStr, incrementStr, lastStr)
	s1 := fmt.Sprintf(base, first)
	s2 := fmt.Sprintf(base, last)
	width := max(len(s1), len(s2))
	prec := max(decimalPrecision(firstStr), max(decimalPrecision(incrementStr), decimalPrecision(lastStr)))
	if prec == 0 {
		return fmt.Sprintf("%%0%dg", width)
	}
	return fmt.Sprintf("%%0%d.%df", width, prec)
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: seq [OPTION]... LAST
  or:  seq [OPTION]... FIRST LAST
  or:  seq [OPTION]... FIRST INCREMENT LAST
Print numbers from FIRST to LAST, in steps of INCREMENT.

  -f, --format=FORMAT   use printf style floating-point FORMAT
  -s, --separator=STRING  use STRING to separate numbers (default: \n)
  -w, --equal-width       equalize width by padding with leading zeroes
      --help              display this help and exit
      --version           output version information and exit
`)
}
