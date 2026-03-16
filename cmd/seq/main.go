// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd019-seq R1.1-R1.4:
// cmd/seq prints a sequence of numbers. Supports three argument forms:
// seq LAST (FIRST=1, STEP=1), seq FIRST LAST (STEP=1), and seq FIRST STEP LAST.
// All arguments are floating-point. Output formatting matches GNU seq precision
// based on the textual representation of input arguments.
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

func main() {
	sys.InstallSIGPIPEHandler()

	positional := parseArgs(os.Args[1:])

	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
	if len(positional) > 3 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, positional[3])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	var firstStr, stepStr, lastStr string
	switch len(positional) {
	case 1:
		// R1.1: seq LAST — FIRST=1, STEP=1.
		firstStr, stepStr, lastStr = "1", "1", positional[0]
	case 2:
		// R1.1: seq FIRST LAST — STEP=1.
		firstStr, stepStr, lastStr = positional[0], "1", positional[1]
	case 3:
		// R1.1: seq FIRST STEP LAST.
		firstStr, stepStr, lastStr = positional[0], positional[1], positional[2]
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

	// Determine output precision from user-provided argument strings.
	prec := computePrecision(positional)

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
			// R2.1: default separator is newline.
			if _, err := w.WriteString("\n"); err != nil {
				break
			}
		}
		if _, err := w.WriteString(formatNumber(val, prec)); err != nil {
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

// parseArgs processes command-line arguments. Handles --help, --version, and --
// (end of flags). All other arguments are returned as positional arguments.
func parseArgs(args []string) []string {
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}

		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", progName, "go-unix-utils", version.Version)
			os.Exit(0)
		}

		positional = append(positional, arg)
	}

	return positional
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Fprintf(os.Stdout,
		"Usage: %s [OPTION]... LAST\n"+
			"  or:  %s [OPTION]... FIRST LAST\n"+
			"  or:  %s [OPTION]... FIRST INCREMENT LAST\n"+
			"Print numbers from FIRST to LAST, in steps of INCREMENT.\n\n"+
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
