// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/numfmt: convert numbers from/to human-readable strings.
// Implements srd071-numfmt R1.1-R1.4 (core conversion), R2.1 (--format),
// R2.2 (--padding), R2.3 (--round), R2.4 (--suffix), R3.1 (--field),
// R3.2 (--delimiter), R3.3 (--header), R3.4 (--from-unit/--to-unit).
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

type unitMode int

const (
	unitNone unitMode = iota
	unitAuto
	unitSI
	unitIEC
	unitIECI
)

// roundMethod controls rounding behavior. R2.3.
type roundMethod int

const (
	roundFromZero    roundMethod = iota // default: away from zero
	roundUp                             // ceiling (towards +infinity)
	roundDown                           // floor (towards -infinity)
	roundTowardsZero                    // truncation
	roundNearest                        // round half away from zero
)

const (
	siBase  = 1000.0
	iecBase = 1024.0
)

var (
	siSuffixes   = []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
	iecSuffixes  = []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
	ieciSuffixes = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
	suffixPower  = map[byte]int{
		'K': 1, 'k': 1, 'M': 2, 'G': 3, 'T': 4,
		'P': 5, 'E': 6, 'Z': 7, 'Y': 8,
	}
)

// formatSpec holds parsed --format directive components. R2.1.
type formatSpec struct {
	prefix    string
	suffix    string
	leftAlign bool
	width     int
	precision int // -1 = use default
}

// config holds parsed command-line options.
type config struct {
	from      unitMode
	to        unitMode
	fromUnit  float64     // R3.4: --from-unit multiplier (default 1)
	toUnit    float64     // R3.4: --to-unit divisor (default 1)
	fmtSpec   formatSpec  // R2.1: --format
	hasFormat bool
	field     int         // R3.1: 1-indexed field to convert (default 1)
	delimiter string      // R3.2: field delimiter (empty = whitespace)
	padding   int         // R2.2: output padding width
	header    int         // R3.3: lines to pass through unchanged (0 = disabled)
	suffix    string      // R2.4: suffix to strip/append
	round     roundMethod // R2.3: rounding method (default from-zero)
	// TODO: --grouping conflicts with srd071 non_goals; skipped per E6.
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

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

func parseArgs(args []string, stdout, stderr io.Writer) (config, []string, int) {
	cfg := config{fromUnit: 1, toUnit: 1, field: 1}
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
		var err error
		if strings.HasPrefix(arg, "--") {
			err = handleLongFlag(arg, args, &i, &cfg)
		} else if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			err = handleShortFlag(arg, args, &i, &cfg)
		} else {
			operands = append(operands, arg)
			continue
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
			return cfg, nil, 1
		}
	}
	return cfg, operands, -1
}

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

// handleLongFlag dispatches long flags using a table-driven approach.
func handleLongFlag(arg string, args []string, i *int, cfg *config) error {
	// R3.3: --header has optional value (--header or --header=N).
	if handled, err := handleHeaderFlag(arg, cfg); handled {
		return err
	}
	type entry struct {
		name    string
		handler func(string, *config) error
	}
	table := []entry{
		{"--from-unit", parseFromUnit}, {"--to-unit", parseToUnit},
		{"--from", parseFromMode}, {"--to", parseToMode},
		{"--format", parseFormat}, {"--field", parseField},
		{"--delimiter", parseDelimiter}, {"--padding", parsePadding},
		{"--suffix", parseSuffix}, {"--round", parseRound},
	}
	for _, e := range table {
		if v, ok := extractFlagValue(arg, args, i, e.name); ok {
			return e.handler(v, cfg)
		}
	}
	return fmt.Errorf("unrecognized option '%s'", arg)
}

// handleHeaderFlag handles --header with optional =N value. R3.3.
func handleHeaderFlag(arg string, cfg *config) (bool, error) {
	if arg == "--header" {
		cfg.header = 1
		return true, nil
	}
	if strings.HasPrefix(arg, "--header=") {
		val := arg[len("--header="):]
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return true, fmt.Errorf("invalid --header argument '%s'", val)
		}
		cfg.header = n
		return true, nil
	}
	return false, nil
}

// handleShortFlag handles -d (delimiter). R3.2.
func handleShortFlag(arg string, args []string, i *int, cfg *config) error {
	if len(arg) >= 2 && arg[1] == 'd' {
		var val string
		if len(arg) > 2 {
			val = arg[2:]
		} else if *i+1 < len(args) {
			*i++
			val = args[*i]
		} else {
			return fmt.Errorf("option '-d' requires an argument")
		}
		return parseDelimiter(val, cfg)
	}
	return fmt.Errorf("unrecognized option '%s'", arg)
}

func extractFlagValue(arg string, args []string, i *int, flag string) (string, bool) {
	if p := flag + "="; strings.HasPrefix(arg, p) {
		return arg[len(p):], true
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

// --- Flag value parsers ---

func parseFromMode(v string, cfg *config) error {
	m, err := parseUnitMode(v)
	if err != nil {
		return fmt.Errorf("invalid --from argument '%s'", v)
	}
	cfg.from = m
	return nil
}

func parseToMode(v string, cfg *config) error {
	m, err := parseUnitMode(v)
	if err != nil {
		return fmt.Errorf("invalid --to argument '%s'", v)
	}
	cfg.to = m
	return nil
}

func parseFromUnit(v string, cfg *config) error {
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid --from-unit argument '%s'", v)
	}
	cfg.fromUnit = n
	return nil
}

func parseToUnit(v string, cfg *config) error {
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid --to-unit argument '%s'", v)
	}
	cfg.toUnit = n
	return nil
}

func parseFormat(v string, cfg *config) error {
	spec, err := parseFormatSpec(v)
	if err != nil {
		return err
	}
	cfg.fmtSpec = spec
	cfg.hasFormat = true
	return nil
}

func parseField(v string, cfg *config) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fmt.Errorf("invalid --field argument '%s'", v)
	}
	cfg.field = n
	return nil
}

func parseDelimiter(v string, cfg *config) error {
	if len(v) != 1 {
		return fmt.Errorf("the delimiter must be a single character")
	}
	cfg.delimiter = v
	return nil
}

func parsePadding(v string, cfg *config) error {
	n, err := strconv.Atoi(v)
	if err != nil || n == 0 {
		return fmt.Errorf("invalid --padding argument '%s'", v)
	}
	cfg.padding = n
	return nil
}

// parseSuffix sets the suffix to strip from input and append to output. R2.4.
func parseSuffix(v string, cfg *config) error {
	cfg.suffix = v
	return nil
}

// parseRound sets the rounding method. R2.3.
func parseRound(v string, cfg *config) error {
	switch v {
	case "up":
		cfg.round = roundUp
	case "down":
		cfg.round = roundDown
	case "from-zero":
		cfg.round = roundFromZero
	case "towards-zero":
		cfg.round = roundTowardsZero
	case "nearest":
		cfg.round = roundNearest
	default:
		return fmt.Errorf("invalid --round argument '%s'", v)
	}
	return nil
}

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

// --- Format spec parsing (R2.1) ---

// parseFormatSpec parses a printf-style format string: prefix%[-][width][.prec]fsuffix.
func parseFormatSpec(f string) (formatSpec, error) {
	pctIdx := strings.Index(f, "%")
	if pctIdx < 0 {
		return formatSpec{}, fmt.Errorf("format '%s' has no %% directive", f)
	}
	spec := formatSpec{prefix: f[:pctIdx], precision: -1}
	rest := f[pctIdx+1:]
	pos := 0
	for pos < len(rest) && (rest[pos] == '-' || rest[pos] == ' ') {
		if rest[pos] == '-' {
			spec.leftAlign = true
		}
		pos++
	}
	pos = parseDigits(rest, pos, &spec.width)
	if pos < len(rest) && rest[pos] == '.' {
		pos++
		spec.precision = 0
		pos = parseDigits(rest, pos, &spec.precision)
	}
	if pos >= len(rest) || rest[pos] != 'f' {
		return formatSpec{}, fmt.Errorf("format '%s' missing 'f' conversion", f)
	}
	spec.suffix = rest[pos+1:]
	return spec, nil
}

// parseDigits reads decimal digits into *out and returns the new position.
func parseDigits(s string, pos int, out *int) int {
	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		*out = *out*10 + int(s[pos]-'0')
		pos++
	}
	return pos
}

// --- Processing ---

func processOperands(operands []string, cfg config, stdout, stderr io.Writer) int {
	exitCode := 0
	for _, op := range operands {
		result, err := convertNumber(op, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			exitCode = 2
			continue
		}
		fmt.Fprintln(stdout, result)
	}
	return exitCode
}

// processStdin reads lines from stdin and processes them. R3.3: header passthrough.
func processStdin(stdin io.Reader, cfg config, stdout, stderr io.Writer) int {
	scanner := bufio.NewScanner(stdin)
	exitCode := 0
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= cfg.header {
			fmt.Fprintln(stdout, scanner.Text())
			continue
		}
		result, err := processLine(scanner.Text(), cfg)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			exitCode = 2
			continue
		}
		fmt.Fprintln(stdout, result)
	}
	return exitCode
}

// processLine handles field selection within a line. R3.1, R3.2.
func processLine(line string, cfg config) (string, error) {
	if cfg.delimiter != "" {
		return processDelimitedLine(line, cfg)
	}
	return processWhitespaceLine(line, cfg)
}

func processWhitespaceLine(line string, cfg config) (string, error) {
	start, end, found := findFieldBounds(line, cfg.field)
	if !found {
		return line, nil
	}
	converted, err := convertNumber(line[start:end], cfg)
	if err != nil {
		return "", err
	}
	return line[:start] + converted + line[end:], nil
}

func processDelimitedLine(line string, cfg config) (string, error) {
	parts := strings.Split(line, cfg.delimiter)
	idx := cfg.field - 1
	if idx >= len(parts) {
		return line, nil
	}
	converted, err := convertNumber(parts[idx], cfg)
	if err != nil {
		return "", err
	}
	parts[idx] = converted
	return strings.Join(parts, cfg.delimiter), nil
}

// findFieldBounds returns byte positions of the n-th whitespace-delimited field.
func findFieldBounds(line string, n int) (int, int, bool) {
	pos, num := 0, 0
	for pos < len(line) {
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}
		if pos >= len(line) {
			break
		}
		num++
		start := pos
		for pos < len(line) && line[pos] != ' ' && line[pos] != '\t' {
			pos++
		}
		if num == n {
			return start, pos, true
		}
	}
	return 0, 0, false
}

// convertNumber converts a single number string.
// R1.1-R1.3, R2.1-R2.4, R3.4.
func convertNumber(input string, cfg config) (string, error) {
	padding := cfg.padding
	trimmed := strings.TrimSpace(input)
	if padding == 0 && len(trimmed) < len(input) {
		padding = len(input)
	}
	// R2.4: strip suffix from input before conversion.
	trimmed = stripSuffix(trimmed, cfg.suffix)
	val, err := parseInputNumber(trimmed, cfg.from)
	if err != nil {
		return "", err
	}
	val *= cfg.fromUnit
	val /= cfg.toUnit
	precision := -1
	if cfg.hasFormat {
		precision = cfg.fmtSpec.precision
	}
	numStr := formatOutputNumber(val, cfg.to, precision, cfg.round)
	if cfg.hasFormat {
		numStr = applyFormat(numStr, cfg.fmtSpec)
	}
	// R2.4: append suffix to output.
	numStr += cfg.suffix
	if padding != 0 {
		numStr = applyPadding(numStr, padding)
	}
	return numStr, nil
}

// stripSuffix removes a user suffix from the input before parsing. R2.4.
func stripSuffix(s, suffix string) string {
	if suffix != "" && strings.HasSuffix(s, suffix) {
		return s[:len(s)-len(suffix)]
	}
	return s
}

// applyFormat applies the format spec to a numeric string. R2.1.
func applyFormat(numStr string, spec formatSpec) string {
	if spec.width > 0 && len(numStr) < spec.width {
		pad := strings.Repeat(" ", spec.width-len(numStr))
		if spec.leftAlign {
			numStr += pad
		} else {
			numStr = pad + numStr
		}
	}
	return spec.prefix + numStr + spec.suffix
}

// applyPadding pads output to width. R2.2: positive=right-align, negative=left-align.
func applyPadding(s string, padding int) string {
	w := padding
	if w < 0 {
		w = -w
	}
	if len(s) >= w {
		return s
	}
	pad := strings.Repeat(" ", w-len(s))
	if padding < 0 {
		return s + pad
	}
	return pad + s
}

// --- Number parsing ---

// parseInputNumber parses a number string with optional suffix.
// R1.2: interprets suffixes based on the from unit mode.
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

func splitNumSuffix(s string) (string, string) {
	i := len(s)
	for i > 0 && isASCIILetter(s[i-1]) {
		i--
	}
	return s[:i], s[i:]
}

func isASCIILetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

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

func scaledPower(suffix string, expectI bool, base float64) (float64, error) {
	idx := lookupSuffixPower(suffix, expectI)
	if idx < 0 {
		return 0, fmt.Errorf("invalid suffix '%s'", suffix)
	}
	return math.Pow(base, float64(idx)), nil
}

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

func autoSuffixMultiplier(suffix string) (float64, error) {
	if len(suffix) == 2 && suffix[1] == 'i' {
		if idx := lookupSuffixPower(suffix, true); idx >= 0 {
			return math.Pow(iecBase, float64(idx)), nil
		}
	}
	if len(suffix) == 1 {
		if idx := lookupSuffixPower(suffix, false); idx >= 0 {
			return math.Pow(siBase, float64(idx)), nil
		}
	}
	return 0, fmt.Errorf("invalid suffix '%s'", suffix)
}

// --- Output formatting ---

// formatOutputNumber formats a value with optional suffix, precision, and rounding. R2.3.
func formatOutputNumber(val float64, mode unitMode, prec int, rnd roundMethod) string {
	if mode == unitNone {
		if prec >= 0 {
			val = roundValue(val, prec, rnd)
			return fmt.Sprintf("%.*f", prec, val)
		}
		return formatPlainNumber(val)
	}
	base, suffixes := outputParams(mode)
	return formatWithSuffix(val, base, suffixes, prec, rnd)
}

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

func formatPlainNumber(val float64) string {
	if val == math.Trunc(val) && !math.IsInf(val, 0) && !math.IsNaN(val) {
		return strconv.FormatInt(int64(val), 10)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// formatWithSuffix scales val by base and formats with the appropriate suffix. R2.3.
func formatWithSuffix(val, base float64, suffixes []string, prec int, rnd roundMethod) string {
	neg := val < 0
	absVal := math.Abs(val)
	idx, scaled := 0, absVal
	for idx+1 < len(suffixes) && scaled >= base {
		scaled /= base
		idx++
	}
	if neg {
		scaled = -scaled
	}
	if idx == 0 {
		if prec >= 0 {
			val = roundValue(val, prec, rnd)
			return fmt.Sprintf("%.*f", prec, val)
		}
		return formatPlainNumber(val)
	}
	return formatScaledValue(scaled, suffixes[idx], prec, rnd)
}

// formatScaledValue formats a scaled value with its unit suffix. R2.3.
func formatScaledValue(val float64, suffix string, prec int, rnd roundMethod) string {
	if prec >= 0 {
		val = roundValue(val, prec, rnd)
		return fmt.Sprintf("%.*f%s", prec, val, suffix)
	}
	defPrec := 1
	if math.Abs(val) >= 10 {
		defPrec = 0
	}
	val = roundValue(val, defPrec, rnd)
	return fmt.Sprintf("%.*f%s", defPrec, val, suffix)
}

// roundValue rounds val to the given precision using the specified method. R2.3.
func roundValue(val float64, precision int, method roundMethod) float64 {
	factor := math.Pow(10, float64(precision))
	scaled := val * factor
	var rounded float64
	switch method {
	case roundUp:
		rounded = math.Ceil(scaled)
	case roundDown:
		rounded = math.Floor(scaled)
	case roundFromZero:
		rounded = roundAwayFromZero(scaled)
	case roundTowardsZero:
		rounded = math.Trunc(scaled)
	case roundNearest:
		rounded = math.Round(scaled)
	default:
		rounded = roundAwayFromZero(scaled)
	}
	return rounded / factor
}

// roundAwayFromZero rounds away from zero: positive values ceil, negative values floor.
func roundAwayFromZero(v float64) float64 {
	if v >= 0 {
		return math.Ceil(v)
	}
	return math.Floor(v)
}

// --- Help ---

func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [NUMBER]...\n", progName)
	fmt.Fprintln(w, "Reformat NUMBER(s), or the numbers from standard input if none are specified.")
	fmt.Fprintln(w)
	printHelpFlags(w)
	fmt.Fprintln(w)
	printHelpUnits(w)
}

func printHelpFlags(w io.Writer) {
	flags := []string{
		"  -d, --delimiter=X  use X instead of whitespace for field delimiter",
		"      --field=N      replace the number in input field N (default N=1)",
		"      --format=FORMAT  use printf style floating-point FORMAT",
		"      --from=UNIT    auto-scale input numbers to UNITs; default is 'none'",
		"      --from-unit=N  specify the input unit size (instead of the default 1)",
		"      --header[=N]   print (without converting) the first N header lines",
		"      --padding=N    pad the output to N characters; positive for right-aligned",
		"      --round=METHOD use METHOD for rounding: up, down, from-zero, towards-zero, nearest",
		"      --suffix=SUFFIX  add SUFFIX to output, and accept optional SUFFIX in input",
		"      --to=UNIT      auto-scale output numbers to UNITs; default is 'none'",
		"      --to-unit=N    the output unit size (instead of the default 1)",
		"      --help         display this help and exit",
		"      --version      output version information and exit",
	}
	for _, f := range flags {
		fmt.Fprintln(w, f)
	}
}

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

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}
