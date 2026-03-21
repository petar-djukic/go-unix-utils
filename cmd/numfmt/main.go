// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd071-numfmt R1.1–R1.4: basic number parsing, --from/--to scaling,
// and default pass-through behavior.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "numfmt"

// scaleUnit represents a number scaling convention.
type scaleUnit int

const (
	scaleNone scaleUnit = iota
	scaleSI
	scaleIEC
	scaleIECI
	scaleAuto
)

const (
	siBase  = 1000.0
	iecBase = 1024.0
)

// suffixLetters maps power index to suffix letter (K=0, M=1, ...).
var suffixLetters = [...]string{"K", "M", "G", "T", "P", "E", "Z", "Y"}

// numfmtConfig holds parsed command-line options.
type numfmtConfig struct {
	from scaleUnit
	to   scaleUnit
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and processes input, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, operands, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	w := bufio.NewWriter(stdout)
	var exitCode int
	if len(operands) > 0 {
		exitCode = processOperands(operands, cfg, w, stderr)
	} else {
		exitCode = processStdin(stdin, cfg, w, stderr)
	}
	if err := w.Flush(); err != nil && exitCode == 0 {
		exitCode = 2
	}
	return exitCode
}

// parseArgs separates flags from operands and builds numfmtConfig.
// Returns config, operands, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (numfmtConfig, []string, int) {
	var cfg numfmtConfig
	var operands []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		consumed, code := processFlag(args, i, &cfg, stdout, stderr)
		if code >= 0 {
			return cfg, nil, code
		}
		i += consumed - 1
	}
	return cfg, operands, -1
}

// isFlag returns true if arg looks like a command-line flag.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// processFlag handles a single flag. Returns consumed count and exit code.
func processFlag(args []string, idx int, cfg *numfmtConfig, stdout, stderr io.Writer) (int, int) {
	arg := args[idx]
	switch {
	case arg == "--help":
		printHelp(stdout)
		return 1, 0
	case arg == "--version":
		printVersion(stdout)
		return 1, 0
	case strings.HasPrefix(arg, "--from="):
		return parseUnitFlag(arg[len("--from="):], &cfg.from, true, stderr, 1)
	case strings.HasPrefix(arg, "--to="):
		return parseUnitFlag(arg[len("--to="):], &cfg.to, false, stderr, 1)
	case arg == "--from":
		return handleSpacedFlag(args, idx, &cfg.from, true, stderr)
	case arg == "--to":
		return handleSpacedFlag(args, idx, &cfg.to, false, stderr)
	default:
		return reportUnknownFlag(arg, stderr)
	}
}

// handleSpacedFlag handles --flag VALUE (space-separated) form.
func handleSpacedFlag(args []string, idx int, target *scaleUnit, allowAuto bool, stderr io.Writer) (int, int) {
	if idx+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option '%s' requires an argument\n",
			progName, args[idx])
		return 1, 1
	}
	return parseUnitFlag(args[idx+1], target, allowAuto, stderr, 2)
}

// reportUnknownFlag prints an error for an unrecognized option.
func reportUnknownFlag(arg string, stderr io.Writer) (int, int) {
	fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
	return 1, 1
}

// parseUnitFlag parses a unit value and stores it in target.
func parseUnitFlag(val string, target *scaleUnit, allowAuto bool, stderr io.Writer, consumed int) (int, int) {
	unit, err := parseScaleUnit(val, allowAuto)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return consumed, 1
	}
	*target = unit
	return consumed, -1
}

// parseScaleUnit converts a string to a scaleUnit value.
func parseScaleUnit(s string, allowAuto bool) (scaleUnit, error) {
	switch s {
	case "none":
		return scaleNone, nil
	case "si":
		return scaleSI, nil
	case "iec":
		return scaleIEC, nil
	case "iec-i":
		return scaleIECI, nil
	case "auto":
		if allowAuto {
			return scaleAuto, nil
		}
	}
	return scaleNone, fmt.Errorf("invalid unit size: '%s'", s)
}

// processOperands converts each command-line operand.
func processOperands(operands []string, cfg numfmtConfig, w *bufio.Writer, stderr io.Writer) int {
	exitCode := 0
	for _, op := range operands {
		if err := convertAndWrite(op, cfg, w, stderr); err != nil {
			exitCode = 2
		}
	}
	return exitCode
}

// processStdin reads lines from stdin and converts each one.
func processStdin(stdin io.Reader, cfg numfmtConfig, w *bufio.Writer, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	exitCode := 0
	for scanner.Scan() {
		if err := convertAndWrite(scanner.Text(), cfg, w, stderr); err != nil {
			exitCode = 2
		}
	}
	if err := scanner.Err(); err != nil && exitCode == 0 {
		exitCode = 2
	}
	return exitCode
}

// convertAndWrite converts a single value and writes the result.
func convertAndWrite(s string, cfg numfmtConfig, w *bufio.Writer, stderr io.Writer) error {
	s = strings.TrimSpace(s)
	if s == "" {
		err := fmt.Errorf("invalid number: ''")
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return err
	}
	val, err := parseInputNumber(s, cfg.from)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return err
	}
	fmt.Fprintln(w, formatOutput(val, cfg.to))
	return nil
}

// parseInputNumber parses a number string with optional suffix per unit.
func parseInputNumber(s string, unit scaleUnit) (float64, error) {
	if unit == scaleNone {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: '%s'", s)
		}
		return v, nil
	}
	return parseScaledNumber(s, unit)
}

// parseScaledNumber parses a number with an optional scaling suffix.
func parseScaledNumber(s string, unit scaleUnit) (float64, error) {
	numStr, suffix := splitNumberSuffix(s)
	if numStr == "" {
		return 0, fmt.Errorf("invalid number: '%s'", s)
	}
	v, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: '%s'", s)
	}
	mult, err := suffixMultiplier(suffix, unit)
	if err != nil {
		return 0, err
	}
	return v * mult, nil
}

// splitNumberSuffix splits "123.4Ki" into ("123.4", "Ki").
func splitNumberSuffix(s string) (string, string) {
	i := len(s)
	for i > 0 && isAlpha(s[i-1]) {
		i--
	}
	return s[:i], s[i:]
}

// isAlpha returns true if c is an ASCII letter.
func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// suffixMultiplier returns the multiplier for the given suffix and unit.
func suffixMultiplier(suffix string, unit scaleUnit) (float64, error) {
	if suffix == "" {
		return 1, nil
	}
	if unit == scaleAuto {
		return autoMultiplier(suffix)
	}
	return unitMultiplier(suffix, unit)
}

// unitMultiplier computes the multiplier for a specific (non-auto) unit.
func unitMultiplier(suffix string, unit scaleUnit) (float64, error) {
	letter, hasI := parseSuffixParts(suffix)
	if err := validateSuffixForm(hasI, unit); err != nil {
		return 0, fmt.Errorf("invalid suffix in input: '%s'", suffix)
	}
	idx := suffixIndex(letter)
	if idx < 0 {
		return 0, fmt.Errorf("invalid suffix in input: '%s'", suffix)
	}
	return math.Pow(baseForUnit(unit), float64(idx+1)), nil
}

// validateSuffixForm checks that the "i" presence matches the unit type.
func validateSuffixForm(hasI bool, unit scaleUnit) error {
	if unit == scaleIECI && !hasI {
		return fmt.Errorf("expected 'i' suffix")
	}
	if unit != scaleIECI && hasI {
		return fmt.Errorf("unexpected 'i' suffix")
	}
	return nil
}

// autoMultiplier determines the multiplier for --from=auto.
// Bare letters (K, M, ...) use SI base (1000).
// Letters with "i" suffix (Ki, Mi, ...) use IEC base (1024).
func autoMultiplier(suffix string) (float64, error) {
	letter, hasI := parseSuffixParts(suffix)
	idx := suffixIndex(letter)
	if idx < 0 {
		return 0, fmt.Errorf("invalid suffix in input: '%s'", suffix)
	}
	base := siBase
	if hasI {
		base = iecBase
	}
	return math.Pow(base, float64(idx+1)), nil
}

// parseSuffixParts splits a suffix into its letter and "i" flag.
// "Ki" -> ("K", true), "K" -> ("K", false).
func parseSuffixParts(suffix string) (string, bool) {
	if len(suffix) == 2 && suffix[1] == 'i' {
		return suffix[:1], true
	}
	return suffix, false
}

// suffixIndex returns the 0-based index for a suffix letter, or -1.
func suffixIndex(letter string) int {
	for i, s := range suffixLetters {
		if s == letter {
			return i
		}
	}
	return -1
}

// baseForUnit returns the numeric base for a scale unit.
func baseForUnit(unit scaleUnit) float64 {
	if unit == scaleSI {
		return siBase
	}
	return iecBase
}

// formatOutput formats a numeric value according to the output unit.
func formatOutput(val float64, unit scaleUnit) string {
	if unit == scaleNone {
		return formatRaw(val)
	}
	base := baseForUnit(unit)
	absVal := math.Abs(val)
	idx := findBestScale(absVal, base)
	if idx < 0 {
		return formatRaw(val)
	}
	scale := math.Pow(base, float64(idx+1))
	scaled := val / scale
	return formatScaled(scaled, buildSuffix(idx, unit))
}

// findBestScale returns the suffix index for the largest applicable scale.
// Returns -1 if no suffix is appropriate (value too small).
func findBestScale(absVal, base float64) int {
	for i := len(suffixLetters) - 1; i >= 0; i-- {
		scale := math.Pow(base, float64(i+1))
		if absVal/scale >= 1.0 {
			return i
		}
	}
	return -1
}

// buildSuffix constructs the display suffix for a given index and unit.
// SI kilo uses lowercase 'k'; IEC kilo uses uppercase 'K'.
func buildSuffix(idx int, unit scaleUnit) string {
	s := suffixLetters[idx]
	if unit == scaleSI && idx == 0 {
		s = "k"
	}
	if unit == scaleIECI {
		s += "i"
	}
	return s
}

// formatScaled formats a scaled value with its unit suffix.
// R1.1: values < 10 get one decimal place; values >= 10 get none.
// GNU numfmt default rounding is "from-zero" (away from zero).
func formatScaled(val float64, suffix string) string {
	if math.Abs(val) < 10 {
		rounded := roundFromZero(val, 1)
		return fmt.Sprintf("%.1f%s", rounded, suffix)
	}
	rounded := roundFromZero(val, 0)
	return fmt.Sprintf("%.0f%s", rounded, suffix)
}

// roundFromZero rounds val away from zero to the given decimal places.
// This matches GNU numfmt's default rounding behavior.
func roundFromZero(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	if val >= 0 {
		return math.Ceil(val*pow) / pow
	}
	return math.Floor(val*pow) / pow
}

// formatRaw formats a value without any suffix.
func formatRaw(val float64) string {
	if val == math.Trunc(val) && !math.IsInf(val, 0) {
		return strconv.FormatInt(int64(val), 10)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [NUMBER]...\n", progName)
	fmt.Fprintln(w, "Reformat NUMBER(s), or the numbers from standard input,")
	fmt.Fprintln(w, "if none are specified.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "      --from=UNIT       auto-scale input UNITs; default is 'none'")
	fmt.Fprintln(w, "      --to=UNIT         auto-scale output UNITs; default is 'none'")
	fmt.Fprintln(w, "      --help            display this help and exit")
	fmt.Fprintln(w, "      --version         output version information and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "UNIT options:")
	fmt.Fprintln(w, "  none       no auto-scaling is done; suffixes will trigger an error")
	fmt.Fprintln(w, "  auto       accept optional single/two letter suffix")
	fmt.Fprintln(w, "  si         accept optional single letter suffix (powers of 1000)")
	fmt.Fprintln(w, "  iec        accept optional single letter suffix (powers of 1024)")
	fmt.Fprintln(w, "  iec-i      accept optional two-letter suffix (powers of 1024)")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}
