// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd071-numfmt R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.4:
// number parsing, --from/--to scaling, --format, --padding, --round, --suffix,
// --field, --delimiter, --header, --from-unit, --to-unit, --invalid mode,
// exit codes, and error handling.
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

// roundMode represents a rounding strategy. R2.3.
type roundMode int

const (
	roundFromZero    roundMode = iota // default: away from zero
	roundNearest                      // round half away from zero
	roundUp                           // ceiling (towards +infinity)
	roundDown                         // floor (towards -infinity)
	roundTowardsZero                  // truncation (towards zero)
)

// invalidMode controls behavior on invalid numbers. R4.2.
type invalidMode int

const (
	invalidAbort  invalidMode = iota // default: report error, exit 2
	invalidFail                      // report error, continue, exit 2
	invalidWarn                      // report warning, continue, exit 0
	invalidIgnore                    // silent pass-through, exit 0
)

const (
	siBase  = 1000.0
	iecBase = 1024.0
)

// suffixLetters maps power index to suffix letter (K=0, M=1, ...).
var suffixLetters = [...]string{"K", "M", "G", "T", "P", "E", "Z", "Y"}

// numfmtConfig holds parsed command-line options.
type numfmtConfig struct {
	from        scaleUnit
	to          scaleUnit
	format      string
	fmtSpec     formatSpec
	padding     int
	round       roundMode
	suffix      string
	fields      *fieldSet   // nil = process whole line; R3.1
	delimiter   string      // empty = whitespace; R3.2
	headerLines int         // R3.3
	fromUnit    float64     // R3.4: multiply input by this
	toUnit      float64     // R3.4: divide output by this
	invalid     invalidMode // R4.2: behavior on invalid numbers
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
	cfg := numfmtConfig{fromUnit: 1, toUnit: 1}
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
	if cfg.format != "" {
		spec, err := parseFormatStr(cfg.format)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			return cfg, nil, 1
		}
		cfg.fmtSpec = spec
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
	case strings.HasPrefix(arg, "--format="):
		cfg.format = arg[len("--format="):]
		return 1, -1
	case strings.HasPrefix(arg, "--padding="):
		return parsePaddingFlag(arg[len("--padding="):], &cfg.padding, stderr)
	case strings.HasPrefix(arg, "--round="):
		return parseRoundFlag(arg[len("--round="):], &cfg.round, stderr)
	case strings.HasPrefix(arg, "--suffix="):
		cfg.suffix = arg[len("--suffix="):]
		return 1, -1
	default:
		return processR3Flag(args, idx, cfg, stderr)
	}
}

// processR3Flag handles R3 and R4 flags (field, delimiter, header, unit scaling, invalid).
func processR3Flag(args []string, idx int, cfg *numfmtConfig, stderr io.Writer) (int, int) {
	arg := args[idx]
	switch {
	case strings.HasPrefix(arg, "--field="):
		return parseFieldFlag(arg[len("--field="):], cfg, stderr)
	case strings.HasPrefix(arg, "--delimiter="):
		cfg.delimiter = arg[len("--delimiter="):]
		return 1, -1
	case arg == "-d":
		return handleDelimiterShortFlag(args, idx, cfg, stderr)
	case strings.HasPrefix(arg, "-d") && !strings.HasPrefix(arg, "--"):
		cfg.delimiter = arg[2:]
		return 1, -1
	case arg == "--header":
		cfg.headerLines = 1
		return 1, -1
	case strings.HasPrefix(arg, "--header="):
		return parseHeaderFlag(arg[len("--header="):], cfg, stderr)
	case strings.HasPrefix(arg, "--from-unit="):
		return parseScaleFactorFlag(arg[len("--from-unit="):], &cfg.fromUnit, stderr)
	case strings.HasPrefix(arg, "--to-unit="):
		return parseScaleFactorFlag(arg[len("--to-unit="):], &cfg.toUnit, stderr)
	case strings.HasPrefix(arg, "--invalid="):
		return parseInvalidFlag(arg[len("--invalid="):], &cfg.invalid, stderr)
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

// parsePaddingFlag parses the --padding value. R2.2.
func parsePaddingFlag(val string, target *int, stderr io.Writer) (int, int) {
	n, err := strconv.Atoi(val)
	if err != nil {
		fmt.Fprintf(stderr, "%s: invalid padding value '%s'\n", progName, val)
		return 1, 1
	}
	*target = n
	return 1, -1
}

// parseRoundFlag parses the --round value. R2.3.
func parseRoundFlag(val string, target *roundMode, stderr io.Writer) (int, int) {
	mode, err := parseRoundMode(val)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1, 1
	}
	*target = mode
	return 1, -1
}

// parseFieldFlag parses the --field value. R3.1.
func parseFieldFlag(val string, cfg *numfmtConfig, stderr io.Writer) (int, int) {
	fs, err := parseFieldSpec(val)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1, 1
	}
	cfg.fields = &fs
	return 1, -1
}

// handleDelimiterShortFlag handles -d VALUE form. R3.2.
func handleDelimiterShortFlag(args []string, idx int, cfg *numfmtConfig, stderr io.Writer) (int, int) {
	if idx+1 >= len(args) {
		fmt.Fprintf(stderr, "%s: option '-d' requires an argument\n", progName)
		return 1, 1
	}
	cfg.delimiter = args[idx+1]
	return 2, -1
}

// parseHeaderFlag parses the --header=N value. R3.3.
func parseHeaderFlag(val string, cfg *numfmtConfig, stderr io.Writer) (int, int) {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		fmt.Fprintf(stderr, "%s: invalid header value '%s'\n", progName, val)
		return 1, 1
	}
	cfg.headerLines = n
	return 1, -1
}

// parseScaleFactorFlag parses --from-unit or --to-unit value. R3.4.
func parseScaleFactorFlag(val string, target *float64, stderr io.Writer) (int, int) {
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		fmt.Fprintf(stderr, "%s: invalid unit size: '%s'\n", progName, val)
		return 1, 1
	}
	*target = float64(n)
	return 1, -1
}

// parseInvalidFlag parses the --invalid value. R4.2.
func parseInvalidFlag(val string, target *invalidMode, stderr io.Writer) (int, int) {
	mode, err := parseInvalidMode(val)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		return 1, 1
	}
	*target = mode
	return 1, -1
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

// parseRoundMode converts a string to a roundMode value. R2.3.
func parseRoundMode(s string) (roundMode, error) {
	switch s {
	case "from-zero":
		return roundFromZero, nil
	case "nearest":
		return roundNearest, nil
	case "up":
		return roundUp, nil
	case "down":
		return roundDown, nil
	case "towards-zero":
		return roundTowardsZero, nil
	}
	return roundFromZero, fmt.Errorf("invalid rounding mode: '%s'", s)
}

// parseInvalidMode converts a string to an invalidMode value. R4.2.
func parseInvalidMode(s string) (invalidMode, error) {
	switch s {
	case "abort":
		return invalidAbort, nil
	case "fail":
		return invalidFail, nil
	case "warn":
		return invalidWarn, nil
	case "ignore":
		return invalidIgnore, nil
	}
	return invalidAbort, fmt.Errorf("invalid --invalid mode: '%s'", s)
}

// processOperands converts each command-line operand. R4.1.
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
// R3.3: passes through the first headerLines lines without conversion.
func processStdin(stdin io.Reader, cfg numfmtConfig, w *bufio.Writer, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	exitCode := 0
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if lineNum <= cfg.headerLines {
			fmt.Fprintln(w, line)
			continue
		}
		if err := processLine(line, cfg, w, stderr); err != nil {
			exitCode = 2
		}
	}
	if err := scanner.Err(); err != nil && exitCode == 0 {
		exitCode = 2
	}
	return exitCode
}

// processLine routes a single line to field or whole-line processing.
func processLine(line string, cfg numfmtConfig, w *bufio.Writer, stderr io.Writer) error {
	if cfg.fields != nil {
		return processLineWithFields(line, cfg, cfg.fields, w, stderr)
	}
	return convertAndWrite(line, cfg, w, stderr)
}

// convertValue converts a single number string, returning the formatted result.
// R3.4: applies fromUnit and toUnit scaling.
func convertValue(s string, cfg numfmtConfig) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("invalid number: ''")
	}
	val, err := parseInputNumber(s, cfg.from)
	if err != nil {
		return "", err
	}
	val *= cfg.fromUnit
	val /= cfg.toUnit
	return formatOutput(val, cfg), nil
}

// convertAndWrite converts a single value and writes the result.
// R4.2: respects --invalid mode for error handling.
func convertAndWrite(s string, cfg numfmtConfig, w *bufio.Writer, stderr io.Writer) error {
	result, err := convertValue(s, cfg)
	if err != nil {
		if cfg.invalid != invalidAbort {
			fmt.Fprintln(w, strings.TrimSpace(s))
		}
		return reportInvalid(err, cfg, stderr)
	}
	fmt.Fprintln(w, result)
	return nil
}

// reportInvalid handles an invalid number error per the --invalid mode. R4.2.
// Returns nil for warn/ignore (exit 0), the error for abort/fail (exit 2).
func reportInvalid(err error, cfg numfmtConfig, stderr io.Writer) error {
	if cfg.invalid != invalidIgnore {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
	}
	if cfg.invalid == invalidWarn || cfg.invalid == invalidIgnore {
		return nil
	}
	return err
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
		return 0, fmt.Errorf("invalid suffix in input: '%s'", s)
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

// formatOutput formats a numeric value according to the config.
// R2.1: uses --format spec. R2.2: applies --padding. R2.4: appends --suffix.
func formatOutput(val float64, cfg numfmtConfig) string {
	if cfg.format != "" {
		result := formatWithSpec(val, cfg)
		return applyPadding(result, cfg.padding)
	}
	result := formatDefault(val, cfg.to, cfg.round)
	result += cfg.suffix
	return applyPadding(result, cfg.padding)
}

// formatDefault formats a value with default formatting rules.
func formatDefault(val float64, unit scaleUnit, mode roundMode) string {
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
	return formatScaled(scaled, buildSuffix(idx, unit), mode)
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
func formatScaled(val float64, suffix string, mode roundMode) string {
	if math.Abs(val) < 10 {
		rounded := roundValue(val, 1, mode)
		return fmt.Sprintf("%.1f%s", rounded, suffix)
	}
	rounded := roundValue(val, 0, mode)
	return fmt.Sprintf("%.0f%s", rounded, suffix)
}

// roundValue rounds val to the given decimal places using the specified mode.
// R2.3: supports from-zero, nearest, up, down, towards-zero.
func roundValue(val float64, decimals int, mode roundMode) float64 {
	pow := math.Pow(10, float64(decimals))
	scaled := val * pow
	var rounded float64
	switch mode {
	case roundNearest:
		rounded = math.Round(scaled)
	case roundUp:
		rounded = math.Ceil(scaled)
	case roundDown:
		rounded = math.Floor(scaled)
	case roundTowardsZero:
		rounded = math.Trunc(scaled)
	default: // roundFromZero
		if scaled >= 0 {
			rounded = math.Ceil(scaled)
		} else {
			rounded = math.Floor(scaled)
		}
	}
	return rounded / pow
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
	fmt.Fprintln(w, "      --format=FORMAT   use printf style floating-point FORMAT")
	fmt.Fprintln(w, "      --padding=N       pad the output to N characters")
	fmt.Fprintln(w, "      --round=METHOD    use METHOD for rounding when scaling")
	fmt.Fprintln(w, "      --suffix=SUFFIX   append SUFFIX to output numbers")
	fmt.Fprintln(w, "  -d, --delimiter=X     use X instead of whitespace for field delimiter")
	fmt.Fprintln(w, "      --field=FIELDS    replace the numbers in these input fields")
	fmt.Fprintln(w, "      --from-unit=N     specify the input unit size")
	fmt.Fprintln(w, "      --to-unit=N       the output unit size")
	fmt.Fprintln(w, "      --header[=N]      print (without converting) the first N header lines")
	fmt.Fprintln(w, "      --invalid=MODE    failure mode for invalid numbers:")
	fmt.Fprintln(w, "                          abort (default), fail, warn, ignore")
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
