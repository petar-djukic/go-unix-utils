// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/numfmt converts numbers between raw numeric and human-readable formats.
// Implements prd071-numfmt R1.1, R1.2, R1.3, R1.4.
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

const (
	unitNone = "none"
	unitSI   = "si"
	unitIEC  = "iec"
	unitIECI = "iec-i"

	baseSI  = 1000.0
	baseIEC = 1024.0

	progName = "numfmt"
)

// R1.1: suffixes for --to output formatting.
var scaleSuffixes = []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
var scaleSISuffixes = []string{"", "k", "M", "G", "T", "P", "E", "Z", "Y"}
var scaleIECISuffixes = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}

func main() {
	sys.InstallSIGPIPEHandler()
	fromUnit, toUnit, operands := parseArgs()
	os.Exit(run(fromUnit, toUnit, operands))
}

// parseArgs extracts --from, --to flags and remaining operands.
func parseArgs() (string, string, []string) {
	fromUnit := unitNone
	toUnit := unitNone
	var operands []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case strings.HasPrefix(arg, "--from="):
			fromUnit = arg[len("--from="):]
		case strings.HasPrefix(arg, "--to="):
			toUnit = arg[len("--to="):]
		case arg == "--":
			operands = append(operands, os.Args[i+1:]...)
			return fromUnit, toUnit, operands
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
			os.Exit(1)
		default:
			operands = append(operands, arg)
		}
	}
	return fromUnit, toUnit, operands
}

func isValidUnit(u string) bool {
	return u == unitNone || u == unitSI || u == unitIEC || u == unitIECI
}

// run processes all operands or stdin and returns the exit code.
func run(fromUnit, toUnit string, operands []string) int {
	if !isValidUnit(fromUnit) {
		fmt.Fprintf(os.Stderr, "%s: invalid --from argument: '%s'\n", progName, fromUnit)
		return 1
	}
	if !isValidUnit(toUnit) {
		fmt.Fprintf(os.Stderr, "%s: invalid --to argument: '%s'\n", progName, toUnit)
		return 1
	}
	if len(operands) > 0 {
		return processOperands(operands, fromUnit, toUnit)
	}
	return processStdin(fromUnit, toUnit)
}

func processOperands(operands []string, fromUnit, toUnit string) int {
	exitCode := 0
	for _, op := range operands {
		if err := convertAndPrint(op, fromUnit, toUnit); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 2
		}
	}
	return exitCode
}

// R1.4: read numbers from stdin when no operands given.
func processStdin(fromUnit, toUnit string) int {
	exitCode := 0
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if err := convertAndPrint(scanner.Text(), fromUnit, toUnit); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 2
		}
	}
	return exitCode
}

func convertAndPrint(input, fromUnit, toUnit string) error {
	input = strings.TrimSpace(input)
	// R1.3: passthrough when neither --from nor --to specified.
	if fromUnit == unitNone && toUnit == unitNone {
		if _, err := strconv.ParseFloat(input, 64); err != nil {
			return fmt.Errorf("invalid number: '%s'", input)
		}
		fmt.Println(input)
		return nil
	}
	value, err := parseNumber(input, fromUnit)
	if err != nil {
		return err
	}
	fmt.Println(formatNumber(value, toUnit))
	return nil
}

// R1.2: parse input number, optionally interpreting suffixes.
func parseNumber(s string, unit string) (float64, error) {
	if unit == unitNone {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: '%s'", s)
		}
		return v, nil
	}
	return parseWithSuffix(s, unit)
}

func parseWithSuffix(s string, unit string) (float64, error) {
	numStr, multiplier := splitSuffix(s, unit)
	v, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: '%s'", s)
	}
	return v * multiplier, nil
}

// splitSuffix separates the numeric part from suffix and returns the multiplier.
func splitSuffix(s string, unit string) (string, float64) {
	if unit == unitIECI {
		return splitIECISuffix(s)
	}
	base := baseSI
	if unit == unitIEC {
		base = baseIEC
	}
	return splitSimpleSuffix(s, base)
}

func splitIECISuffix(s string) (string, float64) {
	if len(s) >= 2 {
		tail := s[len(s)-2:]
		if exp, ok := ieciSuffixExp(tail); ok {
			return s[:len(s)-2], math.Pow(baseIEC, float64(exp))
		}
	}
	return s, 1
}

func splitSimpleSuffix(s string, base float64) (string, float64) {
	if len(s) == 0 {
		return s, 1
	}
	exp := singleCharSuffixExp(s[len(s)-1])
	if exp > 0 {
		return s[:len(s)-1], math.Pow(base, float64(exp))
	}
	return s, 1
}

func singleCharSuffixExp(c byte) int {
	switch c {
	case 'K', 'k':
		return 1
	case 'M':
		return 2
	case 'G':
		return 3
	case 'T':
		return 4
	case 'P':
		return 5
	case 'E':
		return 6
	case 'Z':
		return 7
	case 'Y':
		return 8
	default:
		return 0
	}
}

func ieciSuffixExp(s string) (int, bool) {
	switch s {
	case "Ki":
		return 1, true
	case "Mi":
		return 2, true
	case "Gi":
		return 3, true
	case "Ti":
		return 4, true
	case "Pi":
		return 5, true
	case "Ei":
		return 6, true
	case "Zi":
		return 7, true
	case "Yi":
		return 8, true
	default:
		return 0, false
	}
}

// R1.1: format number for output, with optional suffix.
func formatNumber(value float64, unit string) string {
	if unit == unitNone {
		return formatRaw(value)
	}
	return formatScaled(value, unit)
}

// formatRaw outputs an integer if the value has no fractional part.
func formatRaw(value float64) string {
	if value == math.Trunc(value) && !math.IsInf(value, 0) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// formatScaled converts a number to human-readable form with a suffix.
func formatScaled(value float64, unit string) string {
	base := baseSI
	if unit == unitIEC || unit == unitIECI {
		base = baseIEC
	}
	sfx := scaleSuffixes
	if unit == unitSI {
		sfx = scaleSISuffixes
	} else if unit == unitIECI {
		sfx = scaleIECISuffixes
	}
	negative := value < 0
	abs := math.Abs(value)
	level, scaled := findScaleLevel(abs, base, len(sfx)-1)
	return buildScaledOutput(scaled, negative, level, sfx)
}

func findScaleLevel(abs, base float64, maxLevel int) (int, float64) {
	level := 0
	scaled := abs
	for level < maxLevel && scaled >= base {
		scaled /= base
		level++
	}
	return level, scaled
}

func buildScaledOutput(scaled float64, negative bool, level int, sfx []string) string {
	prefix := ""
	if negative {
		prefix = "-"
	}
	if level == 0 {
		return prefix + formatRaw(scaled)
	}
	if scaled < 10 {
		return fmt.Sprintf("%s%.1f%s", prefix, scaled, sfx[level])
	}
	return fmt.Sprintf("%s%.0f%s", prefix, scaled, sfx[level])
}
