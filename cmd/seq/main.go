// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the seq utility (prd019-seq R1-R4).
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

	separator := "\n"
	format := ""
	equalWidth := false
	args := os.Args[1:]
	operands := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		switch {
		case arg == "-w", arg == "--equal-width":
			equalWidth = true
		case arg == "-s", arg == "--separator":
			i++
			if i >= len(args) {
				die("option requires an argument -- 's'")
			}
			separator = args[i]
		case strings.HasPrefix(arg, "--separator="):
			separator = arg[len("--separator="):]
		case arg == "-f", arg == "--format":
			i++
			if i >= len(args) {
				die("option requires an argument -- 'f'")
			}
			format = args[i]
		case strings.HasPrefix(arg, "--format="):
			format = arg[len("--format="):]
		case strings.HasPrefix(arg, "-") && len(arg) > 1 && !isNumericArg(arg):
			// Handle combined short flags like -ws, -f%.2f, -s', '
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'w':
					equalWidth = true
				case 's':
					if j+1 < len(arg) {
						separator = arg[j+1:]
					} else {
						i++
						if i >= len(args) {
							die("option requires an argument -- 's'")
						}
						separator = args[i]
					}
					j = len(arg) // done with this arg
				case 'f':
					if j+1 < len(arg) {
						format = arg[j+1:]
					} else {
						i++
						if i >= len(args) {
							die("option requires an argument -- 'f'")
						}
						format = args[i]
					}
					j = len(arg) // done with this arg
				default:
					// Could be a negative number that starts with -
					// If so, treat the whole arg as an operand
					operands = append(operands, arg)
					j = len(arg)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	if format != "" {
		if err := validateFormat(format); err != nil {
			die(err.Error())
		}
	}

	var first, step, last float64
	var firstStr, stepStr, lastStr string
	switch len(operands) {
	case 1:
		lastStr = operands[0]
		last = parseFloat(lastStr)
		first, step = 1, 1
		firstStr, stepStr = "1", "1"
	case 2:
		firstStr, lastStr = operands[0], operands[1]
		first = parseFloat(firstStr)
		last = parseFloat(lastStr)
		step = 1
		stepStr = "1"
	case 3:
		firstStr, stepStr, lastStr = operands[0], operands[1], operands[2]
		first = parseFloat(firstStr)
		step = parseFloat(stepStr)
		last = parseFloat(lastStr)
	default:
		if len(operands) == 0 {
			die("missing operand")
		}
		die("extra operand '" + operands[3] + "'")
	}

	if step == 0 {
		die(fmt.Sprintf("invalid Zero increment value: '%s'", stepStr))
	}

	// Determine the output format.
	fmtStr := format
	if fmtStr == "" {
		fmtStr = defaultFormat(firstStr, stepStr, lastStr, equalWidth)
	}

	// Generate and print the sequence.
	printed := false
	for i := 0; ; i++ {
		val := first + float64(i)*step
		if step > 0 && val > last {
			break
		}
		if step < 0 && val < last {
			break
		}

		if printed {
			fmt.Print(separator)
		}
		fmt.Print(formatNumber(fmtStr, val))
		printed = true
	}
	if printed {
		fmt.Println()
	}
}

// isNumericArg checks if s looks like a negative number (e.g. "-3", "-1.5", "-.5").
func isNumericArg(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	rest := s[1:]
	if rest[0] == '.' || (rest[0] >= '0' && rest[0] <= '9') {
		return true
	}
	return false
}

// parseFloat parses a string as float64 or exits with an error.
func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) {
		die(fmt.Sprintf("invalid floating point argument: '%s'", s))
	}
	return v
}

// decimalPrecision returns the number of digits after the decimal point in s.
func decimalPrecision(s string) int {
	// Strip leading sign.
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if idx := strings.Index(s, "."); idx >= 0 {
		return len(s) - idx - 1
	}
	return 0
}

// defaultFormat determines the printf format string based on the operands.
func defaultFormat(firstStr, stepStr, lastStr string, equalWidth bool) string {
	pFirst := decimalPrecision(firstStr)
	pStep := decimalPrecision(stepStr)
	pLast := decimalPrecision(lastStr)
	prec := max(pFirst, pStep, pLast)

	if prec == 0 && !equalWidth {
		return "%g"
	}
	if prec == 0 && equalWidth {
		// Zero-pad integers to the width of the widest endpoint.
		w := max(len(firstStr), len(lastStr))
		return fmt.Sprintf("%%0%dg", w)
	}
	// Floating-point: use fixed precision.
	if equalWidth {
		w := max(len(firstStr), len(lastStr))
		return fmt.Sprintf("%%0%d.%df", w, prec)
	}
	return fmt.Sprintf("%%.%df", prec)
}

// formatNumber formats a float64 using the given format string.
func formatNumber(fmtStr string, val float64) string {
	return fmt.Sprintf(fmtStr, val)
}

// validateFormat checks that format contains exactly one valid floating-point
// conversion specifier.
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
			return fmt.Errorf("format '%s' has incomplete conversion", format)
		}
		if format[i] == '%' {
			i++ // literal %%
			continue
		}
		// Skip flags: -+#0 and space.
		for i < len(format) && strings.ContainsRune("-+#0 '", rune(format[i])) {
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
			return fmt.Errorf("format '%s' has incomplete conversion", format)
		}
		conv := format[i]
		i++
		if strings.ContainsRune("aefgAEFG", rune(conv)) {
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

func die(msg string) {
	fmt.Fprintf(os.Stderr, "seq: %s\n", msg)
	os.Exit(1)
}
