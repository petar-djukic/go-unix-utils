// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd071-numfmt: Convert Numbers from/to Human-Readable Strings.
// Covers R1.1-R1.4 (core), R2.1-R2.4 (format), R3.1-R3.4 (fields/header).
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

// R2.3: roundMode represents the rounding method for output.
type roundMode int

const (
	roundNearest     roundMode = iota
	roundUp                    // ceiling
	roundDown                  // floor
	roundFromZero              // away from zero
	roundTowardsZero           // truncation
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

// R3.1: fieldRange represents a range of field indices (1-based, inclusive).
type fieldRange struct {
	start int // 0 means open start (from 1)
	end   int // 0 means open end (to infinity)
}

// lineToken is a segment of an input line for whitespace field splitting.
type lineToken struct {
	text    string
	isField bool
}

// numfmtConfig holds parsed command-line options.
type numfmtConfig struct {
	from      unitMode
	to        unitMode
	format    string
	padding   int
	round     roundMode    // R2.3
	suffix    string       // R2.4
	fields    []fieldRange // R3.1
	delimiter string       // R3.2
	header    int          // R3.3
	fromUnit  float64      // R3.4
	toUnit    float64      // R3.4
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
	"T": 1e12, "G": 1e9, "M": 1e6, "K": 1e3, "k": 1e3,
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
func parseArgs(args []string) (numfmtConfig, []string, error) {
	cfg := numfmtConfig{fromUnit: 1, toUnit: 1}
	var operands []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			consumed, err := dispatchFlag(arg, args, i, &cfg)
			if err != nil {
				return cfg, nil, err
			}
			i += consumed
			continue
		}
		// R3.2: handle -d short option for delimiter.
		if strings.HasPrefix(arg, "-d") {
			consumed, err := parseShortDelim(arg, args, i, &cfg.delimiter)
			if err != nil {
				return cfg, nil, err
			}
			i += consumed
			continue
		}
		operands = append(operands, arg)
		i++
	}
	return cfg, operands, nil
}

// parseShortDelim handles the -d short option for delimiter.
func parseShortDelim(arg string, args []string, i int, target *string) (int, error) {
	if len(arg) > 2 {
		*target = arg[2:]
		return 1, nil
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 'd'")
	}
	*target = args[i+1]
	return 2, nil
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
	case matchOpt(arg, "--to-unit"):
		return setFloatOpt(arg, "--to-unit", args, i, &cfg.toUnit)
	case matchOpt(arg, "--from-unit"):
		return setFloatOpt(arg, "--from-unit", args, i, &cfg.fromUnit)
	case matchOpt(arg, "--to"):
		return setUnitOpt(arg, "--to", args, i, &cfg.to)
	case matchOpt(arg, "--from"):
		return setUnitOpt(arg, "--from", args, i, &cfg.from)
	case matchOpt(arg, "--format"):
		return setStringOpt(arg, "--format", args, i, &cfg.format)
	case matchOpt(arg, "--padding"):
		return setIntOpt(arg, "--padding", args, i, &cfg.padding)
	case matchOpt(arg, "--round"):
		return setRoundOpt(arg, args, i, &cfg.round)
	case matchOpt(arg, "--suffix"):
		return setStringOpt(arg, "--suffix", args, i, &cfg.suffix)
	case matchOpt(arg, "--field"):
		return setFieldOpt(arg, args, i, &cfg.fields)
	case matchOpt(arg, "--delimiter"):
		return setStringOpt(arg, "--delimiter", args, i, &cfg.delimiter)
	case matchOpt(arg, "--header"):
		return setHeaderOpt(arg, &cfg.header)
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

// setFloatOpt parses and sets a positive float64 flag value (R3.4).
func setFloatOpt(arg, prefix string, args []string, i int, target *float64) (int, error) {
	val, consumed, err := longOptValue(arg, prefix, args, i)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("invalid %s value: '%s'", prefix, val)
	}
	*target = f
	return consumed, nil
}

// setRoundOpt parses and sets a roundMode flag value (R2.3).
func setRoundOpt(arg string, args []string, i int, target *roundMode) (int, error) {
	val, consumed, err := longOptValue(arg, "--round", args, i)
	if err != nil {
		return 0, err
	}
	m, err := parseRoundMode(val)
	if err != nil {
		return 0, err
	}
	*target = m
	return consumed, nil
}

// setFieldOpt parses and sets field ranges (R3.1).
func setFieldOpt(arg string, args []string, i int, target *[]fieldRange) (int, error) {
	val, consumed, err := longOptValue(arg, "--field", args, i)
	if err != nil {
		return 0, err
	}
	ranges, err := parseFieldRanges(val)
	if err != nil {
		return 0, err
	}
	*target = ranges
	return consumed, nil
}

// setHeaderOpt handles --header (default 1) and --header=N (R3.3).
func setHeaderOpt(arg string, target *int) (int, error) {
	if strings.HasPrefix(arg, "--header=") {
		val := arg[len("--header="):]
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid --header value: '%s'", val)
		}
		*target = n
		return 1, nil
	}
	*target = 1
	return 1, nil
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

// R2.3: parseRoundMode converts a string to a roundMode.
func parseRoundMode(s string) (roundMode, error) {
	switch s {
	case "nearest":
		return roundNearest, nil
	case "up":
		return roundUp, nil
	case "down":
		return roundDown, nil
	case "from-zero":
		return roundFromZero, nil
	case "towards-zero":
		return roundTowardsZero, nil
	default:
		return roundNearest, fmt.Errorf("invalid --round mode: '%s'", s)
	}
}

// --- Field range parsing (R3.1) ---

// parseFieldRanges parses a comma-separated field specification like "1,3-5,7-".
func parseFieldRanges(s string) ([]fieldRange, error) {
	parts := strings.Split(s, ",")
	ranges := make([]fieldRange, 0, len(parts))
	for _, p := range parts {
		r, err := parseOneRange(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

// parseOneRange parses a single field range like "3", "3-5", "-5", or "3-".
func parseOneRange(s string) (fieldRange, error) {
	if s == "" {
		return fieldRange{}, fmt.Errorf("invalid field specification")
	}
	dashIdx := strings.Index(s, "-")
	if dashIdx < 0 {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return fieldRange{}, fmt.Errorf("invalid field value: '%s'", s)
		}
		return fieldRange{start: n, end: n}, nil
	}
	return parseRangeWithDash(s, dashIdx)
}

// parseRangeWithDash handles range forms: "N-M", "N-", "-M".
func parseRangeWithDash(s string, dashIdx int) (fieldRange, error) {
	var r fieldRange
	if dashIdx > 0 {
		n, err := strconv.Atoi(s[:dashIdx])
		if err != nil || n < 1 {
			return fieldRange{}, fmt.Errorf("invalid field value: '%s'", s)
		}
		r.start = n
	}
	if dashIdx < len(s)-1 {
		n, err := strconv.Atoi(s[dashIdx+1:])
		if err != nil || n < 1 {
			return fieldRange{}, fmt.Errorf("invalid field value: '%s'", s)
		}
		r.end = n
	}
	return r, nil
}

// fieldInSet returns true if idx is within any of the given ranges.
func fieldInSet(idx int, ranges []fieldRange) bool {
	for _, r := range ranges {
		start := r.start
		if start == 0 {
			start = 1
		}
		if r.end == 0 {
			if idx >= start {
				return true
			}
		} else if idx >= start && idx <= r.end {
			return true
		}
	}
	return false
}

// --- Processing ---

// run processes all input and returns the exit code.
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
// R3.3: passes through header lines without conversion.
// R3.1: delegates to field processing when --field is set.
func processStdin(cfg numfmtConfig, stdin io.Reader, bw *bufio.Writer, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	exitCode := 0
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if lineNum <= cfg.header {
			fmt.Fprintln(bw, line)
			continue
		}
		result, err := processLine(line, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			exitCode = 2
			continue
		}
		fmt.Fprintln(bw, result)
	}
	return exitCode
}

// processLine handles a single input line with optional field selection.
func processLine(line string, cfg numfmtConfig) (string, error) {
	if len(cfg.fields) > 0 {
		return processLineFields(line, cfg)
	}
	return convertValue(line, cfg)
}

// R3.1: processLineFields converts only the specified fields in a line.
func processLineFields(line string, cfg numfmtConfig) (string, error) {
	if cfg.delimiter != "" {
		return processDelimFields(line, cfg)
	}
	return processWSFields(line, cfg)
}

// processWSFields splits a line on whitespace runs and converts matched fields.
func processWSFields(line string, cfg numfmtConfig) (string, error) {
	tokens := tokenizeLine(line)
	fieldIdx := 0
	var result strings.Builder
	for _, tok := range tokens {
		if !tok.isField {
			result.WriteString(tok.text)
			continue
		}
		fieldIdx++
		if fieldInSet(fieldIdx, cfg.fields) {
			converted, err := convertValue(tok.text, cfg)
			if err != nil {
				return "", err
			}
			result.WriteString(converted)
		} else {
			result.WriteString(tok.text)
		}
	}
	return result.String(), nil
}

// tokenizeLine splits a line into alternating whitespace and non-whitespace tokens.
func tokenizeLine(line string) []lineToken {
	var tokens []lineToken
	i := 0
	for i < len(line) {
		if line[i] == ' ' || line[i] == '\t' {
			j := skipWhitespace(line, i)
			tokens = append(tokens, lineToken{line[i:j], false})
			i = j
		} else {
			j := skipNonWhitespace(line, i)
			tokens = append(tokens, lineToken{line[i:j], true})
			i = j
		}
	}
	return tokens
}

// skipWhitespace returns the index past contiguous whitespace.
func skipWhitespace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// skipNonWhitespace returns the index past contiguous non-whitespace.
func skipNonWhitespace(s string, i int) int {
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		i++
	}
	return i
}

// R3.2: processDelimFields splits on a delimiter and converts matched fields.
func processDelimFields(line string, cfg numfmtConfig) (string, error) {
	parts := strings.Split(line, cfg.delimiter)
	for i := range parts {
		if fieldInSet(i+1, cfg.fields) {
			converted, err := convertValue(parts[i], cfg)
			if err != nil {
				return "", err
			}
			parts[i] = converted
		}
	}
	return strings.Join(parts, cfg.delimiter), nil
}

// --- Conversion ---

// convertValue converts a single number string per the configuration.
func convertValue(s string, cfg numfmtConfig) (string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", fmt.Errorf("invalid number: ''")
	}
	// R2.4: strip custom suffix from input before parsing.
	stripped := stripInputSuffix(trimmed, cfg.suffix)
	if isPassthrough(cfg) {
		if _, err := strconv.ParseFloat(stripped, 64); err != nil {
			return "", fmt.Errorf("invalid number: '%s'", trimmed)
		}
		return applyPadding(trimmed, cfg.padding), nil
	}
	val, err := parseInputValue(stripped, cfg.from)
	if err != nil {
		return "", fmt.Errorf("invalid number: '%s'", trimmed)
	}
	// R3.4: scale input by from-unit.
	val *= cfg.fromUnit
	result := formatOutput(val, cfg)
	return applyPadding(result, cfg.padding), nil
}

// stripInputSuffix removes a custom suffix from the end of an input string.
func stripInputSuffix(s, suffix string) string {
	if suffix != "" && strings.HasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// isPassthrough returns true when no conversion options are set.
func isPassthrough(cfg numfmtConfig) bool {
	return cfg.from == unitNone && cfg.to == unitNone &&
		cfg.format == "" && cfg.suffix == "" &&
		cfg.fromUnit == 1 && cfg.toUnit == 1 &&
		cfg.round == roundNearest
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

// --- Output formatting (R1.1, R2.1, R2.3, R2.4, R3.4) ---

// formatOutput converts a raw value to a formatted output string.
func formatOutput(val float64, cfg numfmtConfig) string {
	// R3.4: scale output by to-unit.
	scaled := val / cfg.toUnit
	if cfg.to == unitNone {
		return formatNoUnit(scaled, cfg)
	}
	return formatWithUnit(scaled, cfg)
}

// formatNoUnit formats a value without unit conversion.
func formatNoUnit(val float64, cfg numfmtConfig) string {
	if cfg.format != "" {
		prec := fmtSpecPrecision(cfg.format)
		if prec >= 0 {
			val = applyRound(val, cfg.round, prec)
		}
		return applyFormatStr(val, "", cfg.format) + cfg.suffix
	}
	return formatRaw(val) + cfg.suffix
}

// formatWithUnit scales a value to the target unit and formats it.
func formatWithUnit(val float64, cfg numfmtConfig) string {
	scaled, unitSfx := scaleToUnit(val, cfg.to)
	if unitSfx == "" {
		return formatRaw(val) + cfg.suffix
	}
	prec := resolveDisplayPrec(scaled, cfg.format)
	scaled = applyRound(scaled, cfg.round, prec)
	if cfg.format != "" {
		return applyFormatStr(scaled, unitSfx, cfg.format) + cfg.suffix
	}
	return defaultScaledFormat(scaled, unitSfx) + cfg.suffix
}

// resolveDisplayPrec determines the decimal precision for display.
func resolveDisplayPrec(scaled float64, format string) int {
	if format != "" {
		prec := fmtSpecPrecision(format)
		if prec >= 0 {
			return prec
		}
	}
	if math.Abs(scaled) < 9.95 {
		return 1
	}
	return 0
}

// fmtSpecPrecision extracts the precision from a format string, or -1.
func fmtSpecPrecision(format string) int {
	spec, err := parseFormatSpec(format)
	if err != nil {
		return -1
	}
	return spec.precision
}

// R2.3: applyRound rounds val at the given precision using the mode.
func applyRound(val float64, mode roundMode, precision int) float64 {
	factor := math.Pow(10, float64(precision))
	v := val * factor
	switch mode {
	case roundUp:
		v = math.Ceil(v)
	case roundDown:
		v = math.Floor(v)
	case roundFromZero:
		if v >= 0 {
			v = math.Ceil(v)
		} else {
			v = math.Floor(v)
		}
	case roundTowardsZero:
		v = math.Trunc(v)
	default:
		v = math.Round(v)
	}
	return v / factor
}

// scaleToUnit finds the appropriate unit suffix and scales the value.
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

  -d, --delimiter=X     use X instead of whitespace for field delimiter
      --field=FIELDS    replace the numbers in these input fields (default=1)
      --format=FORMAT   use printf style floating-point FORMAT
      --from=UNIT       auto-scale input NUMBERs to UNITs
      --from-unit=N     specify the input unit size (instead of the default 1)
      --header[=N]      print (without converting) the first N header lines
      --padding=N       pad the output to N characters
      --round=METHOD    use METHOD for rounding when scaling
      --suffix=SUFFIX   add SUFFIX to output numbers, and accept optional SUFFIX in input numbers
      --to=UNIT         auto-scale output NUMBERs to UNITs
      --to-unit=N       the output unit size (instead of the default 1)
      --help            display this help and exit
      --version         output version information and exit

UNIT options: none, si, iec, iec-i
ROUND options: up, down, from-zero, towards-zero, nearest
`)
}
