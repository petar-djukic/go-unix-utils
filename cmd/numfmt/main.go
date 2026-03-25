// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd071-numfmt: Convert Numbers from/to Human-Readable Strings.
// Covers R1.1 (--to conversion), R1.2 (--from conversion), R1.3 (passthrough),
// R1.4 (stdin/operand input), R2.1 (--format), R2.2 (--padding).
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

// unitMode represents the unit system for number conversion.
type unitMode int

const (
	unitNone unitMode = iota
	unitSI
	unitIEC
	unitIECI
)

// suffixEntry pairs a suffix with its multiplier for --to output.
type suffixEntry struct {
	suffix string
	value  float64
}

// fmtSpec holds a parsed --format directive.
type fmtSpec struct {
	prefix    string
	leftAlign bool
	zeroPad   bool
	width     int
	precision int // -1 means not specified
	suffix    string
}

// numfmtConfig holds parsed command-line options.
type numfmtConfig struct {
	from    unitMode
	to      unitMode
	format  string
	padding int
}

// R1.1: SI suffix table (powers of 1000), sorted largest to smallest.
var siTable = []suffixEntry{
	{"Y", 1e24}, {"Z", 1e21}, {"E", 1e18}, {"P", 1e15},
	{"T", 1e12}, {"G", 1e9}, {"M", 1e6}, {"k", 1e3},
}

// R1.1: IEC suffix table (powers of 1024), sorted largest to smallest.
var iecTable = []suffixEntry{
	{"Y", math.Pow(1024, 8)}, {"Z", math.Pow(1024, 7)},
	{"E", math.Pow(1024, 6)}, {"P", math.Pow(1024, 5)},
	{"T", math.Pow(1024, 4)}, {"G", math.Pow(1024, 3)},
	{"M", math.Pow(1024, 2)}, {"K", 1024},
}

// R1.1: IEC-I suffix table (powers of 1024 with 'i' suffix).
var ieciTable = []suffixEntry{
	{"Yi", math.Pow(1024, 8)}, {"Zi", math.Pow(1024, 7)},
	{"Ei", math.Pow(1024, 6)}, {"Pi", math.Pow(1024, 5)},
	{"Ti", math.Pow(1024, 4)}, {"Gi", math.Pow(1024, 3)},
	{"Mi", math.Pow(1024, 2)}, {"Ki", 1024},
}

// R1.2: suffix-to-multiplier maps for --from parsing.
var siFromMap = map[string]float64{
	"Y": 1e24, "Z": 1e21, "E": 1e18, "P": 1e15,
	"T": 1e12, "G": 1e9, "M": 1e6, "K": 1e3,
}

var iecFromMap = map[string]float64{
	"Y": math.Pow(1024, 8), "Z": math.Pow(1024, 7),
	"E": math.Pow(1024, 6), "P": math.Pow(1024, 5),
	"T": math.Pow(1024, 4), "G": math.Pow(1024, 3),
	"M": math.Pow(1024, 2), "K": 1024,
}

var ieciFromMap = map[string]float64{
	"Yi": math.Pow(1024, 8), "Zi": math.Pow(1024, 7),
	"Ei": math.Pow(1024, 6), "Pi": math.Pow(1024, 5),
	"Ti": math.Pow(1024, 4), "Gi": math.Pow(1024, 3),
	"Mi": math.Pow(1024, 2), "Ki": 1024,
	"Y": math.Pow(1024, 8), "Z": math.Pow(1024, 7),
	"E": math.Pow(1024, 6), "P": math.Pow(1024, 5),
	"T": math.Pow(1024, 4), "G": math.Pow(1024, 3),
	"M": math.Pow(1024, 2), "K": 1024,
}

func main() {
	// D1: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()
	cfg, operands, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		os.Exit(2)
	}
	os.Exit(run(cfg, operands, os.Stdin, os.Stdout, os.Stderr))
}

// --- Flag parsing ---

// parseArgs parses numfmt flags and operand arguments.
// R1.4: operands are numbers to convert; stdin used when none given.
func parseArgs(args []string) (numfmtConfig, []string, error) {
	var cfg numfmtConfig
	var operands []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") {
			operands = append(operands, arg)
			i++
			continue
		}
		consumed, err := dispatchFlag(arg, args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
		i += consumed
	}
	return cfg, operands, nil
}

// dispatchFlag handles a single long-option flag argument.
func dispatchFlag(arg string, args []string, i int, cfg *numfmtConfig) (int, error) {
	switch {
	case arg == "--help":
		printHelp()
		os.Exit(0)
		return 0, nil
	case arg == "--version":
		fmt.Println("numfmt (go-unix-utils)")
		os.Exit(0)
		return 0, nil
	case matchOpt(arg, "--to"):
		return setUnitOpt(arg, "--to", args, i, &cfg.to)
	case matchOpt(arg, "--from"):
		return setUnitOpt(arg, "--from", args, i, &cfg.from)
	case matchOpt(arg, "--format"):
		return setStringOpt(arg, "--format", args, i, &cfg.format)
	case matchOpt(arg, "--padding"):
		return setIntOpt(arg, "--padding", args, i, &cfg.padding)
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// matchOpt returns true if arg matches --flag or --flag=VALUE.
func matchOpt(arg, prefix string) bool {
	return arg == prefix || strings.HasPrefix(arg, prefix+"=")
}

// longOptValue extracts the value from --flag=VALUE or --flag VALUE.
func longOptValue(arg, prefix string, args []string, i int) (string, int, error) {
	if strings.HasPrefix(arg, prefix+"=") {
		return arg[len(prefix)+1:], 1, nil
	}
	if i+1 >= len(args) {
		return "", 0, fmt.Errorf("option '%s' requires an argument", prefix)
	}
	return args[i+1], 2, nil
}

// setUnitOpt parses and sets a unitMode flag value.
func setUnitOpt(arg, prefix string, args []string, i int, target *unitMode) (int, error) {
	val, consumed, err := longOptValue(arg, prefix, args, i)
	if err != nil {
		return 0, err
	}
	m, err := parseUnitMode(val)
	if err != nil {
		return 0, err
	}
	*target = m
	return consumed, nil
}

// setStringOpt parses and sets a string flag value.
func setStringOpt(arg, prefix string, args []string, i int, target *string) (int, error) {
	val, consumed, err := longOptValue(arg, prefix, args, i)
	if err != nil {
		return 0, err
	}
	*target = val
	return consumed, nil
}

// setIntOpt parses and sets an integer flag value.
func setIntOpt(arg, prefix string, args []string, i int, target *int) (int, error) {
	val, consumed, err := longOptValue(arg, prefix, args, i)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: '%s'", prefix, val)
	}
	*target = n
	return consumed, nil
}

// parseUnitMode converts a string to a unitMode.
func parseUnitMode(s string) (unitMode, error) {
	switch s {
	case "none":
		return unitNone, nil
	case "si":
		return unitSI, nil
	case "iec":
		return unitIEC, nil
	case "iec-i":
		return unitIECI, nil
	default:
		return unitNone, fmt.Errorf("invalid unit: '%s'", s)
	}
}

// --- Processing ---

// run processes all input and returns the exit code.
// R4.1: exit 0 on success; D2: exit 2 on errors.
func run(cfg numfmtConfig, operands []string, stdin io.Reader, stdout, stderr io.Writer) int {
	bw := bufio.NewWriter(stdout)
	var exitCode int
	if len(operands) > 0 {
		exitCode = processOperands(cfg, operands, bw, stderr)
	} else {
		exitCode = processStdin(cfg, stdin, bw, stderr)
	}
	if err := bw.Flush(); err != nil && exitCode == 0 {
		exitCode = 2
	}
	return exitCode
}

// processOperands converts command-line operand numbers.
// R1.4: operands are processed directly.
func processOperands(cfg numfmtConfig, operands []string, bw *bufio.Writer, stderr io.Writer) int {
	exitCode := 0
	for _, op := range operands {
		result, err := convertValue(op, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			exitCode = 2
			continue
		}
		fmt.Fprintln(bw, result)
	}
	return exitCode
}

// processStdin converts numbers read line by line from stdin.
// R1.4: reads one number per line from stdin.
func processStdin(cfg numfmtConfig, stdin io.Reader, bw *bufio.Writer, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	exitCode := 0
	for scanner.Scan() {
		line := scanner.Text()
		result, err := convertValue(line, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			exitCode = 2
			continue
		}
		fmt.Fprintln(bw, result)
	}
	return exitCode
}

// --- Conversion ---

// convertValue converts a single number string per the configuration.
func convertValue(s string, cfg numfmtConfig) (string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", fmt.Errorf("invalid number: ''")
	}
	// R1.3: passthrough when no conversion needed.
	if cfg.from == unitNone && cfg.to == unitNone && cfg.format == "" {
		if _, err := strconv.ParseFloat(trimmed, 64); err != nil {
			return "", fmt.Errorf("invalid number: '%s'", trimmed)
		}
		return applyPadding(trimmed, cfg.padding), nil
	}
	val, err := parseInputValue(trimmed, cfg.from)
	if err != nil {
		return "", err
	}
	result := formatOutput(val, cfg)
	return applyPadding(result, cfg.padding), nil
}

// --- Input parsing (R1.2) ---

// parseInputValue parses a number with optional suffix per --from mode.
func parseInputValue(s string, mode unitMode) (float64, error) {
	if mode == unitNone {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: '%s'", s)
		}
		return v, nil
	}
	return parseWithSuffix(s, mode)
}

// parseWithSuffix splits a suffixed number and applies the multiplier.
func parseWithSuffix(s string, mode unitMode) (float64, error) {
	numPart, mult := splitSuffix(s, mode)
	v, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: '%s'", s)
	}
	return v * mult, nil
}

// splitSuffix separates the numeric part from a trailing unit suffix.
// R1.2: tries 2-char suffix first (for iec-i), then 1-char.
func splitSuffix(s string, mode unitMode) (string, float64) {
	m := fromSuffixMap(mode)
	if m == nil {
		return s, 1
	}
	if len(s) >= 3 {
		if mult, ok := m[s[len(s)-2:]]; ok {
			return s[:len(s)-2], mult
		}
	}
	if len(s) >= 2 {
		if mult, ok := m[s[len(s)-1:]]; ok {
			return s[:len(s)-1], mult
		}
	}
	return s, 1
}

// fromSuffixMap returns the suffix lookup map for a --from mode.
func fromSuffixMap(mode unitMode) map[string]float64 {
	switch mode {
	case unitSI:
		return siFromMap
	case unitIEC:
		return iecFromMap
	case unitIECI:
		return ieciFromMap
	default:
		return nil
	}
}

// --- Output formatting (R1.1, R2.1) ---

// formatOutput converts a raw value to a formatted output string.
func formatOutput(val float64, cfg numfmtConfig) string {
	if cfg.to == unitNone && cfg.format == "" {
		return formatRaw(val)
	}
	if cfg.to == unitNone {
		return applyFormatStr(val, "", cfg.format)
	}
	scaled, suffix := scaleToUnit(val, cfg.to)
	if cfg.format != "" {
		return applyFormatStr(scaled, suffix, cfg.format)
	}
	if suffix == "" {
		return formatRaw(val)
	}
	return defaultScaledFormat(scaled, suffix)
}

// scaleToUnit finds the appropriate unit suffix and scales the value.
// R1.1: selects the largest unit where abs(val) >= unit value.
func scaleToUnit(val float64, mode unitMode) (float64, string) {
	table := toSuffixTable(mode)
	absVal := math.Abs(val)
	for _, e := range table {
		if absVal >= e.value {
			return val / e.value, e.suffix
		}
	}
	return val, ""
}

// toSuffixTable returns the output suffix table for a --to mode.
func toSuffixTable(mode unitMode) []suffixEntry {
	switch mode {
	case unitSI:
		return siTable
	case unitIEC:
		return iecTable
	case unitIECI:
		return ieciTable
	default:
		return nil
	}
}

// defaultScaledFormat formats a scaled value with its suffix.
// R1.1: uses 1 decimal place when abs < 10, 0 decimals otherwise.
func defaultScaledFormat(scaled float64, suffix string) string {
	if math.Abs(scaled) < 9.95 {
		return fmt.Sprintf("%.1f%s", scaled, suffix)
	}
	return fmt.Sprintf("%.0f%s", scaled, suffix)
}

// formatRaw formats a float64 as integer if whole, else minimal decimal.
func formatRaw(val float64) string {
	if val == math.Trunc(val) && math.Abs(val) < 1e18 {
		return strconv.FormatInt(int64(val), 10)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// --- Format string handling (R2.1) ---

// applyFormatStr parses the format string and renders the value.
func applyFormatStr(val float64, unitSuffix, format string) string {
	spec, err := parseFormatSpec(format)
	if err != nil {
		if unitSuffix != "" {
			return defaultScaledFormat(val, unitSuffix)
		}
		return formatRaw(val)
	}
	return renderFormatSpec(val, unitSuffix, spec)
}

// parseFormatSpec parses a printf-style format string.
// R2.1: supports PREFIX%[-][0][width][.precision]fSUFFIX syntax.
func parseFormatSpec(format string) (fmtSpec, error) {
	pctIdx := strings.Index(format, "%")
	if pctIdx < 0 {
		return fmtSpec{}, fmt.Errorf("no %% directive")
	}
	spec := fmtSpec{prefix: format[:pctIdx], precision: -1}
	return parseDirective(format[pctIdx+1:], spec)
}

// parseDirective parses the format directive after the '%' character.
func parseDirective(s string, spec fmtSpec) (fmtSpec, error) {
	i := parseDirectiveFlags(s, &spec)
	i = parseDirectiveWidth(s, i, &spec)
	i = parseDirectivePrecision(s, i, &spec)
	if i >= len(s) || s[i] != 'f' {
		return fmtSpec{}, fmt.Errorf("expected 'f' conversion")
	}
	spec.suffix = s[i+1:]
	return spec, nil
}

// parseDirectiveFlags parses '-' and '0' flags from a format directive.
func parseDirectiveFlags(s string, spec *fmtSpec) int {
	i := 0
	for i < len(s) && (s[i] == '-' || s[i] == '0') {
		if s[i] == '-' {
			spec.leftAlign = true
		} else if spec.width == 0 {
			spec.zeroPad = true
		}
		i++
	}
	return i
}

// parseDirectiveWidth parses the width field from a format directive.
func parseDirectiveWidth(s string, i int, spec *fmtSpec) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		spec.width = spec.width*10 + int(s[i]-'0')
		i++
	}
	return i
}

// parseDirectivePrecision parses the .precision from a format directive.
func parseDirectivePrecision(s string, i int, spec *fmtSpec) int {
	if i < len(s) && s[i] == '.' {
		i++
		spec.precision = 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			spec.precision = spec.precision*10 + int(s[i]-'0')
			i++
		}
	}
	return i
}

// renderFormatSpec formats a value using the parsed format spec.
func renderFormatSpec(val float64, unitSuffix string, spec fmtSpec) string {
	numStr := formatForSpec(val, unitSuffix, spec.precision)
	core := numStr + unitSuffix
	return spec.prefix + padCore(core, spec) + spec.suffix
}

// formatForSpec formats the numeric part using the given precision.
func formatForSpec(val float64, unitSuffix string, precision int) string {
	if precision >= 0 {
		return strconv.FormatFloat(val, 'f', precision, 64)
	}
	if unitSuffix != "" {
		if math.Abs(val) < 9.95 {
			return strconv.FormatFloat(val, 'f', 1, 64)
		}
		return strconv.FormatFloat(val, 'f', 0, 64)
	}
	return formatRaw(val)
}

// padCore pads a core string to the spec's width.
func padCore(s string, spec fmtSpec) string {
	if spec.width <= 0 || len(s) >= spec.width {
		return s
	}
	gap := spec.width - len(s)
	if spec.leftAlign {
		return s + strings.Repeat(" ", gap)
	}
	if spec.zeroPad {
		return strings.Repeat("0", gap) + s
	}
	return strings.Repeat(" ", gap) + s
}

// --- Padding (R2.2) ---

// applyPadding pads a string to the specified width.
// R2.2: positive N right-aligns, negative N left-aligns.
func applyPadding(s string, padding int) string {
	if padding == 0 {
		return s
	}
	width := padding
	if width < 0 {
		width = -width
	}
	if len(s) >= width {
		return s
	}
	gap := strings.Repeat(" ", width-len(s))
	if padding < 0 {
		return s + gap
	}
	return gap + s
}

// --- Help ---

// printHelp outputs usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: numfmt [OPTION]... [NUMBER]...
Reformat NUMBER(s), or the numbers from standard input if none are specified.

      --from=UNIT       auto-scale input NUMBERs to UNITs
      --to=UNIT         auto-scale output NUMBERs to UNITs
      --format=FORMAT   use printf style floating-point FORMAT
      --padding=N       pad the output to N characters
      --help            display this help and exit
      --version         output version information and exit

UNIT options: none, si, iec, iec-i
`)
}
