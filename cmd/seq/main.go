// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

const helpText = `Usage: seq [OPTION]... LAST
  or:  seq [OPTION]... FIRST LAST
  or:  seq [OPTION]... FIRST INCREMENT LAST
Print numbers from FIRST to LAST, in steps of INCREMENT.

Mandatory arguments to long options are mandatory for short options too.
  -f, --format=FORMAT      use printf style floating-point FORMAT
  -s, --separator=STRING   use STRING to separate numbers (default: \n)
  -w, --equal-width        equalize width by padding with leading zeroes
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = `seq (go-unix-utils) dev
`

type options struct {
	format     string
	separator  string
	equalWidth bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, operands, err := parseArgs(os.Args[1:])
	if err != nil {
		exitWithError(err.Error())
	}
	first, incr, last, err := parseOperands(operands)
	if err != nil {
		exitWithError(err.Error())
	}
	if incr == 0 {
		stepStr := "0"
		if len(operands) == 3 {
			stepStr = operands[1]
		}
		exitWithError(fmt.Sprintf("invalid Zero increment value: '%s'", stepStr))
	}
	if opts.format != "" {
		if opts.equalWidth {
			exitWithError("format string may not be specified when printing equal width strings")
		}
		if err := validateFormat(opts.format); err != nil {
			fmt.Fprintf(os.Stderr, "seq: %s\n", err.Error())
			os.Exit(1)
		}
	}
	printSequence(opts, first, incr, last, operands)
}

func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "seq: %s\n", msg)
	fmt.Fprintf(os.Stderr, "Try 'seq --help' for more information.\n")
	os.Exit(1)
}

func parseArgs(args []string) (options, []string, error) {
	opts := options{separator: "\n"}
	var operands []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			n, err := parseLongFlag(arg, args[i:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += n
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && !isNumStart(arg) {
			n, err := parseShortFlags(arg[1:], args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += 1 + n
			continue
		}
		operands = append(operands, arg)
		i++
	}
	if len(operands) == 0 {
		return opts, nil, fmt.Errorf("missing operand")
	}
	if len(operands) > 3 {
		return opts, nil, fmt.Errorf("extra operand '%s'", operands[3])
	}
	return opts, operands, nil
}

func isNumStart(s string) bool {
	if len(s) < 2 {
		return false
	}
	c := s[1]
	return c == '.' || (c >= '0' && c <= '9')
}

func parseLongFlag(flag string, remaining []string, opts *options) (int, error) {
	switch {
	case flag == "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
		return 0, nil
	case flag == "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
		return 0, nil
	case flag == "--format":
		if len(remaining) < 2 {
			return 0, fmt.Errorf("option '--format' requires an argument")
		}
		opts.format = remaining[1]
		return 2, nil
	case strings.HasPrefix(flag, "--format="):
		opts.format = flag[len("--format="):]
		return 1, nil
	case flag == "--separator":
		if len(remaining) < 2 {
			return 0, fmt.Errorf("option '--separator' requires an argument")
		}
		opts.separator = remaining[1]
		return 2, nil
	case strings.HasPrefix(flag, "--separator="):
		opts.separator = flag[len("--separator="):]
		return 1, nil
	case flag == "--equal-width":
		opts.equalWidth = true
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", flag)
	}
}

func parseShortFlags(flags string, remaining []string, opts *options) (int, error) {
	consumed := 0
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'f':
			var val string
			if rest := flags[j+1:]; rest != "" {
				val = rest
			} else if len(remaining) > consumed {
				val = remaining[consumed]
				consumed++
			} else {
				return 0, fmt.Errorf("option requires an argument -- 'f'")
			}
			opts.format = val
			return consumed, nil
		case 's':
			var val string
			if rest := flags[j+1:]; rest != "" {
				val = rest
			} else if len(remaining) > consumed {
				val = remaining[consumed]
				consumed++
			} else {
				return 0, fmt.Errorf("option requires an argument -- 's'")
			}
			opts.separator = val
			return consumed, nil
		case 'w':
			opts.equalWidth = true
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flags[j])
		}
	}
	return consumed, nil
}

func parseOperands(args []string) (first, incr, last float64, err error) {
	switch len(args) {
	case 1:
		last, err = parseNumber(args[0])
		if err != nil {
			return 0, 0, 0, err
		}
		return 1, 1, last, nil
	case 2:
		first, err = parseNumber(args[0])
		if err != nil {
			return 0, 0, 0, err
		}
		last, err = parseNumber(args[1])
		if err != nil {
			return 0, 0, 0, err
		}
		return first, 1, last, nil
	default:
		first, err = parseNumber(args[0])
		if err != nil {
			return 0, 0, 0, err
		}
		incr, err = parseNumber(args[1])
		if err != nil {
			return 0, 0, 0, err
		}
		last, err = parseNumber(args[2])
		if err != nil {
			return 0, 0, 0, err
		}
		return first, incr, last, nil
	}
}

func parseNumber(s string) (float64, error) {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid floating point argument: '%s'", s)
	}
	if math.IsNaN(n) {
		return 0, fmt.Errorf("invalid 'not-a-number' argument: '%s'", s)
	}
	if math.IsInf(n, 0) {
		return 0, fmt.Errorf("invalid floating point argument: '%s'", s)
	}
	return n, nil
}

func validateFormat(format string) error {
	count := 0
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			i++
			continue
		}
		i++
		if i >= len(format) {
			break
		}
		if format[i] == '%' {
			i++
			continue
		}
		conv, next, err := scanConversion(format, i)
		if err != nil {
			return err
		}
		i = next
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

func scanConversion(format string, i int) (byte, int, error) {
	for i < len(format) && strings.ContainsRune("-+#0 '", rune(format[i])) {
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
	if i >= len(format) {
		return 0, i, fmt.Errorf("format '%s' has no %% directive", format)
	}
	conv := format[i]
	return conv, i + 1, nil
}

func printSequence(opts options, first, incr, last float64, rawArgs []string) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	format := opts.format
	if format == "" {
		format = defaultFormat(rawArgs)
		if opts.equalWidth {
			format = equalWidthFormat(format, first, last)
		}
	}
	started := false
	prev := math.NaN()
	for i := 0; ; i++ {
		val := first + float64(i)*incr
		if val == prev {
			break
		}
		if !inRange(val, incr, last) {
			break
		}
		if started {
			fmt.Fprint(w, opts.separator)
		}
		fmt.Fprintf(w, format, val)
		started = true
		prev = val
	}
	if started {
		fmt.Fprintln(w)
	}
}

func equalWidthFormat(defFmt string, first, last float64) string {
	w1 := len(fmt.Sprintf(defFmt, first))
	w2 := len(fmt.Sprintf(defFmt, last))
	maxW := max(w1, w2)
	prec := 0
	if dot := strings.IndexByte(defFmt, '.'); dot >= 0 {
		prec, _ = strconv.Atoi(defFmt[dot+1 : len(defFmt)-1])
	}
	if prec == 0 {
		return "%0" + strconv.Itoa(maxW) + ".0f"
	}
	return "%0" + strconv.Itoa(maxW) + "." + strconv.Itoa(prec) + "f"
}

func inRange(val, incr, last float64) bool {
	eps := math.Abs(incr) * 1e-10
	if incr >= 0 {
		return val-last <= eps
	}
	return last-val <= eps
}

func defaultFormat(rawArgs []string) string {
	maxPrec := 0
	for _, arg := range rawArgs {
		p := argPrecision(arg)
		if p > maxPrec {
			maxPrec = p
		}
	}
	if maxPrec == 0 {
		return "%.0f"
	}
	return "%." + strconv.Itoa(maxPrec) + "f"
}

func argPrecision(s string) int {
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		s = s[1:]
	}
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return 0
	}
	return len(s) - dot - 1
}
