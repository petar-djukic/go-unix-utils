// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd019-seq R1.1–R1.5, R2.1–R2.3
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
		} else if strings.HasPrefix(arg, "-s") {
			// -sFOO form
			separator = arg[2:]
		} else if strings.HasPrefix(arg, "--separator=") {
			separator = arg[len("--separator="):]
		} else {
			positional = append(positional, arg)
		}
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
	default:
		fmt.Fprintf(os.Stderr, "seq: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
		os.Exit(1)
	}

	// R1.5: zero step is an error.
	if increment == 0 {
		fmt.Fprintf(os.Stderr, "seq: invalid Zero increment value: '%s'\n", incrementStr)
		fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
		os.Exit(1)
	}

	// R2.3: Determine output format based on input precision.
	format := defaultFormat(firstStr, lastStr)

	// R3.3: Equal-width zero padding.
	if equalWidth {
		format = equalWidthFormat(first, last, firstStr, lastStr)
	}

	w := bufio.NewWriter(os.Stdout)

	// R1.2: Generate the sequence.
	printed := false
	for val := first; ; val += increment {
		// R1.2: Stop when next value exceeds LAST.
		if increment > 0 && val > last*(1+1e-14)+1e-14 {
			break
		}
		if increment < 0 && val < last*(1+1e-14)-1e-14 {
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

// defaultFormat returns a printf format string that preserves the decimal
// precision of the input arguments. R2.3: integer inputs produce integer
// output; floating-point inputs use the maximum precision of the inputs.
func defaultFormat(firstStr, lastStr string) string {
	p1 := decimalPrecision(firstStr)
	p2 := decimalPrecision(lastStr)
	prec := max(p1, p2)
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
func equalWidthFormat(first, last float64, firstStr, lastStr string) string {
	base := defaultFormat(firstStr, lastStr)
	s1 := fmt.Sprintf(base, first)
	s2 := fmt.Sprintf(base, last)
	width := max(len(s1), len(s2))
	prec := max(decimalPrecision(firstStr), decimalPrecision(lastStr))
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

  -s, --separator=STRING  use STRING to separate numbers (default: \n)
  -w, --equal-width       equalize width by padding with leading zeroes
      --help              display this help and exit
      --version           output version information and exit
`)
}
