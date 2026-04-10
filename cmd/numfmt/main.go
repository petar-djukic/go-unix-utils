// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/numfmt: convert numbers from/to human-readable strings.
// Implements srd071-numfmt R1.1 (input/output conversion), R1.2 (--from/--to with
// SI and IEC suffixes), R1.3 (passthrough), R1.4 (stdin/operand input),
// R3.4 (--from-unit/--to-unit multipliers).
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

// progName is used in diagnostic messages.
const progName = "numfmt"

// unitMode represents the --from/--to unit scaling type.
// R1.2: none, auto, si, iec, iec-i.
type unitMode int

const (
	unitNone unitMode = iota
	unitAuto
	unitSI
	unitIEC
	unitIECI
)

// Base values for SI and IEC scaling.
const (
	siBase  = 1000.0
	iecBase = 1024.0
)

// Suffix tables for output formatting.
// R1.1: SI uses base 1000, IEC/IEC-I use base 1024.
var (
	siSuffixes   = []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
	iecSuffixes  = []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
	ieciSuffixes = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
)

// suffixPower maps suffix first-letter to its power index (1-based).
// R1.2: K=1, M=2, G=3, T=4, P=5, E=6, Z=7, Y=8.
var suffixPower = map[byte]int{
	'K': 1, 'k': 1, 'M': 2, 'G': 3, 'T': 4,
	'P': 5, 'E': 6, 'Z': 7, 'Y': 8,
}

// config holds parsed command-line options.
type config struct {
	from     unitMode
	to       unitMode
	fromUnit float64 // R3.4: --from-unit multiplier (default 1)
	toUnit   float64 // R3.4: --to-unit divisor (default 1)
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the numfmt logic and returns the exit code.
// R1.4: reads from stdin when no operands are given.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, operands, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(operands) > 0 {
		return processOperands(operands, cfg, stdout, stderr)
	}
	return processStdin(stdin, cfg, stdout, stderr)
}

// parseArgs parses command-line arguments. Returns config, operands, and
// exit code (-1 means continue processing).
// R1.4: --help and --version exit 0.
func parseArgs(args []string, stdout, stderr io.Writer) (config, []string, int) {
	cfg := config{fromUnit: 1, toUnit: 1}
	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if code := handleInfoFlags(arg, stdout); code >= 0 {
			return cfg, nil, code
		}
		if !strings.HasPrefix(arg, "--") {
			operands = append(operands, arg)
			continue
		}
		if err := handleLongFlag(arg, args, &i, &cfg); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
			return cfg, nil, 1
		}
	}
	return cfg, operands, -1
}

// handleInfoFlags handles --help and --version. Returns exit code or -1.
func handleInfoFlags(arg string, stdout io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	}
	return -1
}

// handleLongFlag dispatches a long flag to the appropriate handler.
func handleLongFlag(arg string, args []string, i *int, cfg *config) error {
	if v, ok := extractFlagValue(arg, args, i, "--from-unit"); ok {
		return parseFromUnit(v, cfg)
	}
	if v, ok := extractFlagValue(arg, args, i, "--to-unit"); ok {
		return parseToUnit(v, cfg)
	}
	if v, ok := extractFlagValue(arg, args, i, "--from"); ok {
		return parseFromMode(v, cfg)
	}
	if v, ok := extractFlagValue(arg, args, i, "--to"); ok {
		return parseToMode(v, cfg)
	}
	return fmt.Errorf("unrecognized option '%s'", arg)
}

// extractFlagValue extracts the value from --flag=VALUE or --flag VALUE form.
// Returns ("", false) when arg does not match the flag name.
func extractFlagValue(arg string, args []string, i *int, flag string) (string, bool) {
	prefix := flag + "="
	if strings.HasPrefix(arg, prefix) {
		return arg[len(prefix):], true
	}
	if arg == flag {
		if *i+1 < len(args) {
			*i++
			return args[*i], true
		}
		return "", true
	}
	return "", false
}

// parseFromMode sets cfg.from from a string value.
func parseFromMode(v string, cfg *config) error {
	mode, err := parseUnitMode(v)
	if err != nil {
		return fmt.Errorf("invalid --from argument '%s'", v)
	}
	cfg.from = mode
	return nil
}

// parseToMode sets cfg.to from a string value.
func parseToMode(v string, cfg *config) error {
	mode, err := parseUnitMode(v)
	if err != nil {
		return fmt.Errorf("invalid --to argument '%s'", v)
	}
	cfg.to = mode
	return nil
}

// parseFromUnit sets cfg.fromUnit from a string value.
func parseFromUnit(v string, cfg *config) error {
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid --from-unit argument '%s'", v)
	}
	cfg.fromUnit = n
	return nil
}

// parseToUnit sets cfg.toUnit from a string value.
func parseToUnit(v string, cfg *config) error {
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid --to-unit argument '%s'", v)
	}
	cfg.toUnit = n
	return nil
}

// parseUnitMode converts a string to a unitMode.
// R1.2: recognizes none, auto, si, iec, iec-i.
func parseUnitMode(s string) (unitMode, error) {
	switch s {
	case "none":
		return unitNone, nil
	case "auto":
		return unitAuto, nil
	case "si":
		return unitSI, nil
	case "iec":
		return unitIEC, nil
	case "iec-i":
		return unitIECI, nil
	}
	return unitNone, fmt.Errorf("unknown unit: %s", s)
}

// processOperands converts command-line operands and prints results.
// R1.4: processes operands directly.
func processOperands(operands []string, cfg config, stdout, stderr io.Writer) int {
	exitCode := 0
	for _, op := range operands {
		if err := convertAndPrint(op, cfg, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			exitCode = 2
		}
	}
	return exitCode
}

// processStdin reads lines from stdin and converts each.
// R1.4: reads one number per line.
func processStdin(stdin io.Reader, cfg config, stdout, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	exitCode := 0
	for scanner.Scan() {
		line := scanner.Text()
		if err := convertAndPrint(line, cfg, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			exitCode = 2
		}
	}
	return exitCode
}

// convertAndPrint converts a single input string and prints the result.
// R1.1, R1.2, R1.3, R3.4: parse → scale → format pipeline.
func convertAndPrint(input string, cfg config, w io.Writer) error {
	input = strings.TrimSpace(input)
	val, err := parseInputNumber(input, cfg.from)
	if err != nil {
		return err
	}
	val *= cfg.fromUnit
	val /= cfg.toUnit
	output := formatOutputNumber(val, cfg.to)
	_, err = fmt.Fprintln(w, output)
	return err
}

// parseInputNumber parses a number string with optional suffix.
// R1.2: interprets suffixes based on the from unit mode.
// R1.3: unitNone requires plain numeric input.
func parseInputNumber(s string, mode unitMode) (float64, error) {
	if mode == unitNone {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: '%s'", s)
		}
		return v, nil
	}
	numStr, suffix := splitNumSuffix(s)
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: '%s'", s)
	}
	if suffix == "" {
		return val, nil
	}
	mult, err := suffixMultiplier(suffix, mode)
	if err != nil {
		return 0, fmt.Errorf("invalid suffix in input '%s'", s)
	}
	return val * mult, nil
}

// splitNumSuffix splits a string into its numeric part and suffix.
// The suffix is the trailing alphabetic characters.
func splitNumSuffix(s string) (string, string) {
	i := len(s)
	for i > 0 && isASCIILetter(s[i-1]) {
		i--
	}
	return s[:i], s[i:]
}

// isASCIILetter reports whether c is an ASCII letter.
func isASCIILetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// suffixMultiplier returns the multiplier for a suffix in the given mode.
// R1.2: dispatches to mode-specific suffix handlers.
func suffixMultiplier(suffix string, mode unitMode) (float64, error) {
	switch mode {
	case unitSI:
		return scaledPower(suffix, false, siBase)
	case unitIEC:
		return scaledPower(suffix, false, iecBase)
	case unitIECI:
		return scaledPower(suffix, true, iecBase)
	case unitAuto:
		return autoSuffixMultiplier(suffix)
	}
	return 0, fmt.Errorf("unknown suffix '%s'", suffix)
}

// scaledPower returns base^power for a suffix letter.
// When expectI is true, the suffix must end with 'i' (e.g., "Ki").
func scaledPower(suffix string, expectI bool, base float64) (float64, error) {
	idx := lookupSuffixPower(suffix, expectI)
	if idx < 0 {
		return 0, fmt.Errorf("invalid suffix '%s'", suffix)
	}
	return math.Pow(base, float64(idx)), nil
}

// lookupSuffixPower returns the power index for a suffix string.
// Returns -1 if the suffix is not recognized.
func lookupSuffixPower(suffix string, expectI bool) int {
	if expectI {
		if len(suffix) != 2 || suffix[1] != 'i' {
			return -1
		}
		idx, ok := suffixPower[suffix[0]]
		if !ok {
			return -1
		}
		return idx
	}
	if len(suffix) != 1 {
		return -1
	}
	idx, ok := suffixPower[suffix[0]]
	if !ok {
		return -1
	}
	return idx
}

// autoSuffixMultiplier detects the suffix type and returns the multiplier.
// R1.2: single letter (K) → SI (1000-based), letter+i (Ki) → IEC (1024-based).
func autoSuffixMultiplier(suffix string) (float64, error) {
	if len(suffix) == 2 && suffix[1] == 'i' {
		idx := lookupSuffixPower(suffix, true)
		if idx >= 0 {
			return math.Pow(iecBase, float64(idx)), nil
		}
	}
	if len(suffix) == 1 {
		idx := lookupSuffixPower(suffix, false)
		if idx >= 0 {
			return math.Pow(siBase, float64(idx)), nil
		}
	}
	return 0, fmt.Errorf("invalid suffix '%s'", suffix)
}

// formatOutputNumber formats a value with the appropriate suffix.
// R1.1: --to=si uses base 1000, --to=iec uses base 1024, --to=iec-i uses
// base 1024 with 'i' suffix. R1.3: unitNone outputs plain number.
func formatOutputNumber(val float64, mode unitMode) string {
	if mode == unitNone {
		return formatPlainNumber(val)
	}
	base, suffixes := outputParams(mode)
	return formatWithSuffix(val, base, suffixes)
}

// outputParams returns the base and suffix table for an output mode.
func outputParams(mode unitMode) (float64, []string) {
	switch mode {
	case unitSI:
		return siBase, siSuffixes
	case unitIEC:
		return iecBase, iecSuffixes
	case unitIECI:
		return iecBase, ieciSuffixes
	}
	return 1, []string{""}
}

// formatPlainNumber formats a value as a plain number without suffix.
// Outputs integer form when the value has no fractional part.
func formatPlainNumber(val float64) string {
	if val == math.Trunc(val) && !math.IsInf(val, 0) && !math.IsNaN(val) {
		return strconv.FormatInt(int64(val), 10)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// formatWithSuffix formats a value using the largest appropriate suffix.
// R1.1: finds the highest unit where scaled value >= 1.
func formatWithSuffix(val float64, base float64, suffixes []string) string {
	neg := val < 0
	absVal := math.Abs(val)
	idx := 0
	scaled := absVal
	for idx+1 < len(suffixes) && scaled >= base {
		scaled /= base
		idx++
	}
	prefix := ""
	if neg {
		prefix = "-"
	}
	if idx == 0 {
		return prefix + formatPlainNumber(absVal)
	}
	return prefix + formatScaledValue(scaled, suffixes[idx])
}

// formatScaledValue formats a scaled value with its suffix.
// GNU numfmt convention: one decimal if < 10, no decimal if >= 10.
func formatScaledValue(val float64, suffix string) string {
	if val < 10 {
		return fmt.Sprintf("%.1f%s", val, suffix)
	}
	return fmt.Sprintf("%.0f%s", val, suffix)
}

// printHelp prints usage information to stdout. R1.4.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [NUMBER]...\n", progName)
	fmt.Fprintln(w, "Reformat NUMBER(s), or the numbers from standard input if none are specified.")
	fmt.Fprintln(w)
	printHelpFlags(w)
	fmt.Fprintln(w)
	printHelpUnits(w)
}

// printHelpFlags prints the flag descriptions for --help output.
func printHelpFlags(w io.Writer) {
	fmt.Fprintln(w, "      --from=UNIT       auto-scale input numbers to UNITs; default is 'none';")
	fmt.Fprintln(w, "                          see UNIT below")
	fmt.Fprintln(w, "      --from-unit=N     specify the input unit size (instead of the default 1)")
	fmt.Fprintln(w, "      --to=UNIT         auto-scale output numbers to UNITs; default is 'none';")
	fmt.Fprintln(w, "                          see UNIT below")
	fmt.Fprintln(w, "      --to-unit=N       the output unit size (instead of the default 1)")
	fmt.Fprintln(w, "      --help            display this help and exit")
	fmt.Fprintln(w, "      --version         output version information and exit")
}

// printHelpUnits prints the UNIT options section for --help output.
func printHelpUnits(w io.Writer) {
	fmt.Fprintln(w, "UNIT options:")
	fmt.Fprintln(w, "  none       no auto-scaling is done; suffixes will trigger an error")
	fmt.Fprintln(w, "  auto       accept optional single/two letter suffix:")
	fmt.Fprintln(w, "               1K = 1000, 1Ki = 1024, 1M = 1000000, 1Mi = 1048576, ...")
	fmt.Fprintln(w, "  si         accept optional single letter suffix:")
	fmt.Fprintln(w, "               1K = 1000, 1M = 1000000, ...")
	fmt.Fprintln(w, "  iec        accept optional single letter suffix:")
	fmt.Fprintln(w, "               1K = 1024, 1M = 1048576, ...")
	fmt.Fprintln(w, "  iec-i      accept optional two letter suffix:")
	fmt.Fprintln(w, "               1Ki = 1024, 1Mi = 1048576, ...")
}

// printVersion prints version information to stdout. R1.4.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}
