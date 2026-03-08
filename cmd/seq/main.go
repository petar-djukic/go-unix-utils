// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the seq utility for printing a sequence of numbers.
//
// Implements prd019-seq: argument forms (R1), output formatting (R2),
// format string and equal-width (R3), exit codes (R4).
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "seq"

// config holds the parsed command-line options.
type config struct {
	separator  string
	format     string
	equalWidth bool
	first      float64
	step       float64
	last       float64
	// Track precision from input arguments for default formatting. R2.3.
	firstPrec int
	stepPrec  int
	lastPrec  int
	isInteger bool // true when all arguments are integers
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (config, error) {
	cfg := config{
		separator: "\n",
		first:     1,
		step:      1,
	}

	var positional []string
	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			positional = append(positional, args[i:]...)
			break
		}

		// Long options with = form.
		if strings.HasPrefix(arg, "--separator=") {
			cfg.separator = arg[len("--separator="):]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--format=") {
			cfg.format = arg[len("--format="):]
			i++
			continue
		}
		if arg == "--equal-width" {
			cfg.equalWidth = true
			i++
			continue
		}
		if arg == "--separator" {
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("option '-s' requires an argument")
			}
			cfg.separator = args[i]
			i++
			continue
		}
		if arg == "--format" {
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("option '-f' requires an argument")
			}
			cfg.format = args[i]
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && !isNumericArg(arg) {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'w':
					cfg.equalWidth = true
					j++
				case 's':
					rest := arg[j+1:]
					if rest != "" {
						cfg.separator = rest
					} else {
						i++
						if i >= len(args) {
							return cfg, fmt.Errorf("option '-s' requires an argument")
						}
						cfg.separator = args[i]
					}
					j = len(arg)
				case 'f':
					rest := arg[j+1:]
					if rest != "" {
						cfg.format = rest
					} else {
						i++
						if i >= len(args) {
							return cfg, fmt.Errorf("option '-f' requires an argument")
						}
						cfg.format = args[i]
					}
					j = len(arg)
				default:
					return cfg, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			i++
			continue
		}

		positional = append(positional, arg)
		i++
	}

	// Parse positional arguments.
	switch len(positional) {
	case 0:
		return cfg, fmt.Errorf("missing operand")
	case 1:
		last, prec, err := parseNumber(positional[0])
		if err != nil {
			return cfg, fmt.Errorf("invalid floating point argument: %s", positional[0])
		}
		cfg.last = last
		cfg.lastPrec = prec
		cfg.firstPrec = 0
		cfg.stepPrec = 0
	case 2:
		first, fp, err := parseNumber(positional[0])
		if err != nil {
			return cfg, fmt.Errorf("invalid floating point argument: %s", positional[0])
		}
		last, lp, err := parseNumber(positional[1])
		if err != nil {
			return cfg, fmt.Errorf("invalid floating point argument: %s", positional[1])
		}
		cfg.first = first
		cfg.last = last
		cfg.firstPrec = fp
		cfg.lastPrec = lp
		cfg.stepPrec = 0
	case 3:
		first, fp, err := parseNumber(positional[0])
		if err != nil {
			return cfg, fmt.Errorf("invalid floating point argument: %s", positional[0])
		}
		step, sp, err := parseNumber(positional[1])
		if err != nil {
			return cfg, fmt.Errorf("invalid floating point argument: %s", positional[1])
		}
		last, lp, err := parseNumber(positional[2])
		if err != nil {
			return cfg, fmt.Errorf("invalid floating point argument: %s", positional[2])
		}
		cfg.first = first
		cfg.step = step
		cfg.last = last
		cfg.firstPrec = fp
		cfg.stepPrec = sp
		cfg.lastPrec = lp
	default:
		return cfg, fmt.Errorf("extra operand '%s'", positional[3])
	}

	// R1.5: zero step is an error.
	if cfg.step == 0 {
		return cfg, fmt.Errorf("invalid Zero increment value: '0'")
	}

	// Check NaN. R4.2.
	if math.IsNaN(cfg.first) || math.IsNaN(cfg.step) || math.IsNaN(cfg.last) {
		return cfg, fmt.Errorf("invalid Not-a-number argument")
	}

	// Determine if all arguments are integers for default formatting. R2.3.
	cfg.isInteger = cfg.firstPrec == 0 && cfg.stepPrec == 0 && cfg.lastPrec == 0

	// Validate format string if provided. R3.1, R3.2.
	if cfg.format != "" {
		if err := validateFormat(cfg.format); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

// isNumericArg returns true if the argument looks like a negative number.
func isNumericArg(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	c := arg[1]
	return c == '.' || (c >= '0' && c <= '9')
}

// parseNumber parses a numeric string and returns its value and decimal precision.
func parseNumber(s string) (float64, int, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, 0, err
	}
	prec := 0
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		prec = len(s) - idx - 1
	}
	return v, prec, nil
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
				return fmt.Errorf("format '%%' has no conversion specifier")
			}
			if format[i] == '%' {
				i++
				continue
			}
			// Skip flags: -+#0 and space.
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
				return fmt.Errorf("format '%%' has incomplete conversion specifier")
			}
			spec := format[i]
			switch spec {
			case 'a', 'e', 'f', 'g', 'A', 'E', 'F', 'G':
				count++
			default:
				return fmt.Errorf("format '%%%c' has unknown %%%c directive", spec, spec)
			}
			i++
		} else {
			i++
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

// run generates the sequence and prints it.
func run(cfg config) error {
	// R1.4: empty sequence cases.
	if cfg.step > 0 && cfg.first > cfg.last {
		return nil
	}
	if cfg.step < 0 && cfg.first < cfg.last {
		return nil
	}

	// Determine the format string to use.
	fmtStr := cfg.format
	if fmtStr == "" {
		fmtStr = defaultFormat(cfg)
	}

	// If equal-width and no explicit format, compute width and use zero-padded format.
	if cfg.equalWidth && cfg.format == "" {
		fmtStr = equalWidthFormat(cfg)
	}

	first := true
	val := cfg.first

	for {
		if cfg.step > 0 && val > cfg.last+cfg.step*1e-10 {
			break
		}
		if cfg.step < 0 && val < cfg.last+cfg.step*1e-10 {
			break
		}

		if !first {
			fmt.Print(cfg.separator)
		}

		fmt.Printf(fmtStr, val)
		first = false

		val += cfg.step

		// Guard against infinite loops from very small steps.
		if cfg.step > 0 && val > cfg.last+cfg.step*0.5 {
			break
		}
		if cfg.step < 0 && val < cfg.last+cfg.step*0.5 {
			break
		}
	}

	if !first {
		fmt.Println()
	}

	return nil
}

// defaultFormat returns the default format string for a number. R2.3.
func defaultFormat(cfg config) string {
	if cfg.isInteger {
		return "%.0f"
	}
	prec := maxInt(cfg.firstPrec, cfg.stepPrec, cfg.lastPrec)
	return fmt.Sprintf("%%.%df", prec)
}

// equalWidthFormat returns a zero-padded format string. R3.3.
func equalWidthFormat(cfg config) string {
	// Determine width from FIRST and LAST formatted with default format.
	dfmt := defaultFormat(cfg)
	firstStr := fmt.Sprintf(dfmt, cfg.first)
	lastStr := fmt.Sprintf(dfmt, cfg.last)
	w := maxInt(len(firstStr), len(lastStr))

	if cfg.isInteger {
		return fmt.Sprintf("%%0%d.0f", w)
	}
	prec := maxInt(cfg.firstPrec, cfg.stepPrec, cfg.lastPrec)
	return fmt.Sprintf("%%0%d.%df", w, prec)
}

func maxInt(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
