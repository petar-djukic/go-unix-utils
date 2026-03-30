// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/numfmt converts numbers between raw numeric and human-readable formats.
// Implements prd071-numfmt R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
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

	roundFromZero    = "from-zero"
	roundTowardsZero = "towards-zero"
	roundUp          = "up"
	roundDown        = "down"
	roundNearest     = "nearest"
)

// R1.1: suffixes for --to output formatting.
var (
	scaleSuffixes     = []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
	scaleSISuffixes   = []string{"", "k", "M", "G", "T", "P", "E", "Z", "Y"}
	scaleIECISuffixes = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
)

// config holds all command-line options.
type config struct {
	fromUnit string
	toUnit   string
	format   string // R2.1: printf-style format string
	padding  int    // R2.2: output padding width
	round    string // R2.3: rounding method
	suffix   string // R2.4: suffix appended after unit
}

// fmtSpec holds parsed --format components (R2.1).
type fmtSpec struct {
	prefix    string
	leftAlign bool
	width     int
	precision int // -1 = unset (use default)
	tail      string
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, operands := parseArgs()
	os.Exit(run(cfg, operands))
}

func parseArgs() (config, []string) {
	cfg := config{fromUnit: unitNone, toUnit: unitNone, round: roundFromZero}
	var operands []string
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--" {
			operands = append(operands, os.Args[i+1:]...)
			return cfg, operands
		}
		if parseFlag(arg, &cfg) {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
			os.Exit(1)
		}
		operands = append(operands, arg)
	}
	return cfg, operands
}

// parseFlag handles --from, --to, --format, --padding, --round, --suffix.
func parseFlag(arg string, cfg *config) bool {
	switch {
	case strings.HasPrefix(arg, "--from="):
		cfg.fromUnit = arg[len("--from="):]
	case strings.HasPrefix(arg, "--to="):
		cfg.toUnit = arg[len("--to="):]
	case strings.HasPrefix(arg, "--format="):
		cfg.format = arg[len("--format="):]
	case strings.HasPrefix(arg, "--padding="):
		v, err := strconv.Atoi(arg[len("--padding="):])
		if err == nil {
			cfg.padding = v
		}
	case strings.HasPrefix(arg, "--round="):
		cfg.round = arg[len("--round="):]
	case strings.HasPrefix(arg, "--suffix="):
		cfg.suffix = arg[len("--suffix="):]
	default:
		return false
	}
	return true
}

func isValidUnit(u string) bool {
	return u == unitNone || u == unitSI || u == unitIEC || u == unitIECI
}

func isValidRound(r string) bool {
	switch r {
	case roundFromZero, roundTowardsZero, roundUp, roundDown, roundNearest:
		return true
	default:
		return false
	}
}

func run(cfg config, operands []string) int {
	if !isValidUnit(cfg.fromUnit) {
		fmt.Fprintf(os.Stderr, "%s: invalid --from argument: '%s'\n", progName, cfg.fromUnit)
		return 1
	}
	if !isValidUnit(cfg.toUnit) {
		fmt.Fprintf(os.Stderr, "%s: invalid --to argument: '%s'\n", progName, cfg.toUnit)
		return 1
	}
	if !isValidRound(cfg.round) {
		fmt.Fprintf(os.Stderr, "%s: invalid --round argument: '%s'\n", progName, cfg.round)
		return 1
	}
	if len(operands) > 0 {
		return processOperands(operands, cfg)
	}
	return processStdin(cfg)
}

func processOperands(operands []string, cfg config) int {
	exitCode := 0
	for _, op := range operands {
		if err := convertAndPrint(op, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 2
		}
	}
	return exitCode
}

// R1.4: read numbers from stdin when no operands given.
func processStdin(cfg config) int {
	exitCode := 0
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if err := convertAndPrint(scanner.Text(), cfg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			exitCode = 2
		}
	}
	return exitCode
}

func convertAndPrint(input string, cfg config) error {
	input = strings.TrimSpace(input)
	// R1.3: pure passthrough when no options are active.
	if isPassthrough(cfg) {
		return passthroughAndPrint(input)
	}
	// R2.4: strip user suffix from input before parsing.
	stripped := stripInputSuffix(input, cfg.suffix)
	value, err := parseNumber(stripped, cfg.fromUnit)
	if err != nil {
		return fmt.Errorf("invalid number: '%s'", input)
	}
	fmt.Println(formatOutput(value, cfg))
	return nil
}

func isPassthrough(cfg config) bool {
	return cfg.fromUnit == unitNone && cfg.toUnit == unitNone &&
		cfg.format == "" && cfg.padding == 0 && cfg.suffix == ""
}

func passthroughAndPrint(input string) error {
	if _, err := strconv.ParseFloat(input, 64); err != nil {
		return fmt.Errorf("invalid number: '%s'", input)
	}
	fmt.Println(input)
	return nil
}

// R2.4: strip --suffix from input before parsing.
func stripInputSuffix(s, suffix string) string {
	if suffix != "" && strings.HasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
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

// --- Output formatting ---

func formatOutput(value float64, cfg config) string {
	spec := parseFmtSpec(cfg.format)
	if cfg.toUnit == unitNone {
		return formatRawOutput(value, cfg, spec)
	}
	return formatScaledOutput(value, cfg, spec)
}

// R2.1: parse --format printf-style spec (%[-][width][.precision]f).
func parseFmtSpec(format string) fmtSpec {
	spec := fmtSpec{precision: -1}
	if format == "" {
		return spec
	}
	pctIdx := strings.IndexByte(format, '%')
	if pctIdx < 0 {
		spec.prefix = format
		return spec
	}
	spec.prefix = format[:pctIdx]
	parseFormatVerb(format[pctIdx+1:], &spec)
	return spec
}

func parseFormatVerb(s string, spec *fmtSpec) {
	if len(s) > 0 && s[0] == '-' {
		spec.leftAlign = true
		s = s[1:]
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 {
		spec.width, _ = strconv.Atoi(s[:i])
		s = s[i:]
	}
	if len(s) > 0 && s[0] == '.' {
		s = s[1:]
		j := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		spec.precision = 0
		if j > 0 {
			spec.precision, _ = strconv.Atoi(s[:j])
		}
		s = s[j:]
	}
	if len(s) > 0 && (s[0] == 'f' || s[0] == 'g') {
		s = s[1:]
	}
	spec.tail = s
}

func formatRawOutput(value float64, cfg config, spec fmtSpec) string {
	var numStr string
	if spec.precision >= 0 {
		negative := value < 0
		abs := math.Abs(value)
		abs = roundToPrecision(abs, spec.precision, cfg.round, negative)
		signed := abs
		if negative {
			signed = -abs
		}
		numStr = strconv.FormatFloat(signed, 'f', spec.precision, 64)
	} else {
		numStr = formatRaw(value)
	}
	return assembleOutput(numStr, cfg, spec)
}

func formatScaledOutput(value float64, cfg config, spec fmtSpec) string {
	base, sfx := scaleInfo(cfg.toUnit)
	negative := value < 0
	abs := math.Abs(value)
	level, scaled := findScaleLevel(abs, base, len(sfx)-1)
	prec := determinePrecision(scaled, level, spec)
	if prec >= 0 {
		scaled = roundToPrecision(scaled, prec, cfg.round, negative)
	}
	// Re-evaluate precision when rounding changes magnitude (default only).
	if spec.precision < 0 {
		newPrec := determinePrecision(scaled, level, spec)
		if newPrec != prec && newPrec >= 0 {
			scaled = roundToPrecision(scaled, newPrec, cfg.round, negative)
		}
		prec = newPrec
	}
	return buildScaledString(scaled, prec, negative, sfx[level], cfg, spec)
}

func determinePrecision(scaled float64, level int, spec fmtSpec) int {
	if spec.precision >= 0 {
		return spec.precision
	}
	if level == 0 {
		return -1
	}
	if scaled < 10 {
		return 1
	}
	return 0
}

func buildScaledString(scaled float64, prec int, negative bool, unitSfx string, cfg config, spec fmtSpec) string {
	signed := scaled
	if negative {
		signed = -scaled
	}
	var numStr string
	if prec >= 0 {
		numStr = strconv.FormatFloat(signed, 'f', prec, 64)
	} else {
		numStr = formatRaw(signed)
	}
	return assembleOutput(numStr+unitSfx, cfg, spec)
}

func assembleOutput(core string, cfg config, spec fmtSpec) string {
	result := core + cfg.suffix
	pad := effectivePadding(cfg.padding, spec)
	if pad != 0 {
		result = padOutput(result, pad)
	}
	return spec.prefix + result + spec.tail
}

func scaleInfo(unit string) (float64, []string) {
	switch unit {
	case unitSI:
		return baseSI, scaleSISuffixes
	case unitIECI:
		return baseIEC, scaleIECISuffixes
	default:
		return baseIEC, scaleSuffixes
	}
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

// formatRaw outputs an integer if the value has no fractional part.
func formatRaw(value float64) string {
	if value == math.Trunc(value) && !math.IsInf(value, 0) {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// R2.2: compute effective padding from --padding or --format width.
func effectivePadding(cfgPadding int, spec fmtSpec) int {
	if cfgPadding != 0 {
		return cfgPadding
	}
	if spec.width > 0 {
		if spec.leftAlign {
			return -spec.width
		}
		return spec.width
	}
	return 0
}

// R2.2: pad output string to width. Positive = right-align, negative = left-align.
func padOutput(s string, padding int) string {
	width := padding
	if width < 0 {
		width = -width
	}
	if len(s) >= width {
		return s
	}
	spaces := strings.Repeat(" ", width-len(s))
	if padding < 0 {
		return s + spaces
	}
	return spaces + s
}

// R2.3: round value to precision using the specified method.
func roundToPrecision(value float64, precision int, method string, negative bool) float64 {
	factor := math.Pow(10, float64(precision))
	v := value * factor
	v = applyRound(v, method, negative)
	return v / factor
}

// applyRound applies rounding to a positive scaled value.
// negative indicates the original number's sign for directional rounding.
func applyRound(v float64, method string, negative bool) float64 {
	switch method {
	case roundUp:
		if negative {
			return math.Floor(v)
		}
		return math.Ceil(v)
	case roundDown:
		if negative {
			return math.Ceil(v)
		}
		return math.Floor(v)
	case roundFromZero:
		return math.Ceil(v)
	case roundTowardsZero:
		return math.Floor(v)
	case roundNearest:
		return math.Round(v)
	default:
		return math.Round(v)
	}
}
