// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sort implements the sort (sort lines of text files) command.
// Implements: prd053-sort R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R1.7, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sortMode represents the active comparison mode for sorting.
type sortMode int

const (
	sortLexicographic sortMode = iota // default: byte-value comparison
	sortNumeric                       // R2.1: -n numeric value
	sortHumanNumeric                  // R2.2: -h numeric with SI suffixes
	sortMonth                         // R2.3: -M month abbreviation
	sortVersion                       // R2.4: -V version number segments
)

// keyDef represents a single -k KEYDEF specification.
// R3.2: KEYDEF format is F[.C][OPTS][,F[.C][OPTS]].
type keyDef struct {
	startField  int      // 1-based start field number
	startChar   int      // 1-based start character position (0 = field start)
	endField    int      // 1-based end field number (0 = end of line)
	endChar     int      // 1-based end character position (0 = end of field)
	mode        sortMode // per-key sort mode
	hasMode     bool     // true if mode was explicitly set on this key
	reverse     bool     // per-key reverse
	hasReverse  bool     // true if reverse was explicitly set on this key
	ignoreBlank bool     // per-key ignore leading blanks
	hasBlank    bool     // true if ignoreBlank was explicitly set on this key
}

// siMultipliers maps SI suffix characters to their multiplier values.
// R2.2: K, M, G, T, P, E, Z, Y suffixes for human-numeric sort.
var siMultipliers = map[byte]float64{
	'K': 1e3, 'M': 1e6, 'G': 1e9, 'T': 1e12,
	'P': 1e15, 'E': 1e18, 'Z': 1e21, 'Y': 1e24,
}

// monthRank maps uppercase three-letter month abbreviations to their rank.
// R2.3: JAN < FEB < ... < DEC, unknown strings get rank 0 (sort before JAN).
var monthRank = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

// config holds all parsed command-line options.
type config struct {
	reverse      bool     // -r: reverse sort order
	unique       bool     // -u: output only the first of equal consecutive lines
	stable       bool     // -s: preserve input order of equal lines
	ignoreBlanks bool     // -b: ignore leading blanks in sort keys
	outputFile   string   // -o FILE: write output to FILE
	mode         sortMode // active sort comparison mode
	keys         []keyDef // R3.2: -k KEYDEF key specifications
	separator    byte     // R3.1: -t CHAR field separator
	hasSeparator bool     // true when -t was specified
	files        []string
}

func main() {
	// D1: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		os.Exit(2)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		os.Exit(2)
	}
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags {
			cfg.files = append(cfg.files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		if arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--reverse":
				cfg.reverse = true
			case arg == "--unique":
				cfg.unique = true
			case arg == "--stable":
				cfg.stable = true
			case arg == "--output" || strings.HasPrefix(arg, "--output="):
				val, err := parseLongOptValue(arg, "--output", args, &i)
				if err != nil {
					return nil, err
				}
				cfg.outputFile = val
			case arg == "--numeric-sort":
				cfg.mode = sortNumeric
			case arg == "--human-numeric-sort":
				cfg.mode = sortHumanNumeric
			case arg == "--month-sort":
				cfg.mode = sortMonth
			case arg == "--version-sort":
				cfg.mode = sortVersion
			case arg == "--ignore-leading-blanks":
				cfg.ignoreBlanks = true
			case arg == "--key" || strings.HasPrefix(arg, "--key="):
				val, err := parseLongOptValue(arg, "--key", args, &i)
				if err != nil {
					return nil, err
				}
				kd, err := parseKeyDef(val)
				if err != nil {
					return nil, err
				}
				cfg.keys = append(cfg.keys, kd)
			case arg == "--field-separator" || strings.HasPrefix(arg, "--field-separator="):
				val, err := parseLongOptValue(arg, "--field-separator", args, &i)
				if err != nil {
					return nil, err
				}
				if len(val) != 1 {
					return nil, fmt.Errorf("multi-character tab %q", val)
				}
				cfg.separator = val[0]
				cfg.hasSeparator = true
			default:
				return nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
		}

		// Short flags.
		if strings.HasPrefix(arg, "-") {
			rest := arg[1:]
			for j := 0; j < len(rest); j++ {
				ch := rest[j]
				switch ch {
				case 'r':
					cfg.reverse = true
				case 'u':
					cfg.unique = true
				case 's':
					cfg.stable = true
				case 'n':
					cfg.mode = sortNumeric
				case 'h':
					cfg.mode = sortHumanNumeric
				case 'M':
					cfg.mode = sortMonth
				case 'V':
					cfg.mode = sortVersion
				case 'b':
					cfg.ignoreBlanks = true
				case 'k':
					val, err := parseShortOptValue(rest, j, args, &i)
					if err != nil {
						return nil, err
					}
					kd, err := parseKeyDef(val)
					if err != nil {
						return nil, err
					}
					cfg.keys = append(cfg.keys, kd)
					j = len(rest) // consumed rest
				case 't':
					val, err := parseShortOptValue(rest, j, args, &i)
					if err != nil {
						return nil, err
					}
					if len(val) != 1 {
						return nil, fmt.Errorf("multi-character tab %q", val)
					}
					cfg.separator = val[0]
					cfg.hasSeparator = true
					j = len(rest) // consumed rest
				case 'o':
					val, err := parseShortOptValue(rest, j, args, &i)
					if err != nil {
						return nil, err
					}
					cfg.outputFile = val
					j = len(rest) // consumed rest
				default:
					return nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			continue
		}

		cfg.files = append(cfg.files, arg)
	}

	return cfg, nil
}

// parseLongOptValue extracts the value for a long option, either from
// --opt=value form or from the next argument.
func parseLongOptValue(arg, name string, args []string, i *int) (string, error) {
	if strings.HasPrefix(arg, name+"=") {
		return arg[len(name)+1:], nil
	}
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("option '%s' requires an argument", name)
	}
	return args[*i], nil
}

// parseShortOptValue extracts the value for a short option that takes an
// argument. If characters remain after the flag letter in the same token,
// they form the value; otherwise the next argument is consumed.
func parseShortOptValue(rest string, j int, args []string, i *int) (string, error) {
	if j+1 < len(rest) {
		return rest[j+1:], nil
	}
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("option requires an argument -- '%c'", rest[j])
	}
	return args[*i], nil
}

// run executes the sort logic with the given configuration.
func run(cfg *config) error {
	// R1.2, R1.3: Read all input lines from stdin or named files.
	lines, err := readAllLines(cfg.files)
	if err != nil {
		return err
	}

	// Build comparison function based on sort mode and key specifications.
	// R3.2: When keys are specified, use key-based comparison.
	var cmp func(a, b []byte) int
	var applyGlobalReverse bool

	if len(cfg.keys) > 0 {
		cmp = buildKeyCompareFunc(cfg)
		// Per-key reverse is handled inside buildKeyCompareFunc.
		applyGlobalReverse = false
	} else {
		cmp = buildCompareFunc(cfg.mode)
		applyGlobalReverse = cfg.reverse
	}

	// GNU sort applies a last-resort full-line lexicographic comparison
	// when the primary key comparison is equal, unless -s or -u is active.
	useLastResort := !cfg.stable && !cfg.unique &&
		(len(cfg.keys) > 0 || cfg.mode != sortLexicographic)

	lessFunc := func(i, j int) bool {
		c := cmp(lines[i], lines[j])
		if c == 0 && useLastResort {
			c = bytes.Compare(lines[i], lines[j])
		}
		if applyGlobalReverse {
			// R1.4: -r reverses the sort order.
			return c > 0
		}
		return c < 0
	}
	// R1.7: -s preserves input order. -u also implies stable sort in GNU sort
	// to ensure the first of equal elements (in input order) is kept.
	if cfg.stable || cfg.unique {
		sort.SliceStable(lines, lessFunc)
	} else {
		sort.Slice(lines, lessFunc)
	}

	// R1.5: -u outputs only the first of consecutive equal lines after sorting.
	if cfg.unique {
		lines = dedupWith(lines, func(a, b []byte) bool {
			return cmp(a, b) == 0
		})
	}

	// R1.6: -o FILE writes output to FILE instead of stdout.
	return writeOutput(cfg, lines)
}

// buildCompareFunc returns a comparison function for the given sort mode.
func buildCompareFunc(mode sortMode) func(a, b []byte) int {
	switch mode {
	case sortNumeric:
		return compareNumeric
	case sortHumanNumeric:
		return compareHumanNumeric
	case sortMonth:
		return compareMonth
	case sortVersion:
		return compareVersion
	default:
		return func(a, b []byte) int {
			return bytes.Compare(a, b)
		}
	}
}

// readAllLines reads all lines from the given files (or stdin if none).
// Each line is stored as a byte slice without the trailing newline.
func readAllLines(files []string) ([][]byte, error) {
	if len(files) == 0 {
		return readLines(os.Stdin)
	}

	var allLines [][]byte
	for _, f := range files {
		var lines [][]byte
		var err error
		if f == "-" {
			lines, err = readLines(os.Stdin)
		} else {
			lines, err = readLinesFromFile(f)
		}
		if err != nil {
			return nil, err
		}
		allLines = append(allLines, lines...)
	}
	return allLines, nil
}

// readLinesFromFile opens a file and reads all lines from it.
func readLinesFromFile(path string) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open failed: %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
	return readLines(f)
}

// readLines reads all lines from a reader, returning each line as a byte slice.
func readLines(r io.Reader) ([][]byte, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines [][]byte
	for scanner.Scan() {
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}
	return lines, nil
}

// dedupWith removes consecutive elements where equal returns true.
// R1.5: -u suppresses consecutive equal lines using the active comparison.
func dedupWith(lines [][]byte, equal func(a, b []byte) bool) [][]byte {
	if len(lines) == 0 {
		return lines
	}
	result := [][]byte{lines[0]}
	for i := 1; i < len(lines); i++ {
		if !equal(lines[i], lines[i-1]) {
			result = append(result, lines[i])
		}
	}
	return result
}

// --- R2.1: Numeric sort ---

// compareNumeric compares two lines by their numeric value.
// R2.1: Parsing leading whitespace and optional sign.
func compareNumeric(a, b []byte) int {
	va := parseNumeric(a)
	vb := parseNumeric(b)
	switch {
	case va < vb:
		return -1
	case va > vb:
		return 1
	default:
		return 0
	}
}

// parseNumeric extracts a numeric value from the beginning of a byte slice.
// Skips leading whitespace, reads optional sign, digits, and decimal point.
func parseNumeric(b []byte) float64 {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	if i >= len(b) {
		return 0
	}
	start := i
	if b[i] == '+' || b[i] == '-' {
		i++
	}
	hasDigits := false
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		hasDigits = true
		i++
	}
	if i < len(b) && b[i] == '.' {
		i++
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			hasDigits = true
			i++
		}
	}
	if !hasDigits {
		return 0
	}
	val, err := strconv.ParseFloat(string(b[start:i]), 64)
	if err != nil {
		return 0
	}
	return val
}

// --- R2.2: Human-numeric sort ---

// compareHumanNumeric compares two lines by numeric value with SI suffixes.
// R2.2: Recognizes K, M, G, T, P, E, Z, Y suffixes.
func compareHumanNumeric(a, b []byte) int {
	va := parseHumanNumeric(a)
	vb := parseHumanNumeric(b)
	switch {
	case va < vb:
		return -1
	case va > vb:
		return 1
	default:
		return 0
	}
}

// parseHumanNumeric extracts a numeric value with an optional SI suffix.
func parseHumanNumeric(b []byte) float64 {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	if i >= len(b) {
		return 0
	}
	start := i
	if b[i] == '+' || b[i] == '-' {
		i++
	}
	hasDigits := false
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		hasDigits = true
		i++
	}
	if i < len(b) && b[i] == '.' {
		i++
		for i < len(b) && b[i] >= '0' && b[i] <= '9' {
			hasDigits = true
			i++
		}
	}
	if !hasDigits {
		return 0
	}
	val, err := strconv.ParseFloat(string(b[start:i]), 64)
	if err != nil {
		return 0
	}
	// Check for SI suffix.
	if i < len(b) {
		if mult, ok := siMultipliers[b[i]]; ok {
			val *= mult
		}
	}
	return val
}

// --- R2.3: Month sort ---

// compareMonth compares two lines by month abbreviation.
// R2.3: Unknown strings sort before JAN.
func compareMonth(a, b []byte) int {
	ma := parseMonthRank(a)
	mb := parseMonthRank(b)
	switch {
	case ma < mb:
		return -1
	case ma > mb:
		return 1
	default:
		return 0
	}
}

// parseMonthRank extracts a month rank from the beginning of a byte slice.
// Skips leading whitespace, reads three characters, and maps to month rank.
func parseMonthRank(b []byte) int {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	if i+3 > len(b) {
		return 0
	}
	abbr := strings.ToUpper(string(b[i : i+3]))
	if rank, ok := monthRank[abbr]; ok {
		return rank
	}
	return 0
}

// --- R2.4: Version sort ---

// compareVersion compares two lines using version number sorting.
// R2.4: Natural sort of version number segments using gnulib filevercmp algorithm.
func compareVersion(a, b []byte) int {
	if len(a) == 0 {
		if len(b) == 0 {
			return 0
		}
		return -1
	}
	if len(b) == 0 {
		return 1
	}
	// Split at file extension boundary and compare prefix then suffix.
	aPrefix := filePrefixLen(a)
	bPrefix := filePrefixLen(b)
	result := verrevcmp(a[:aPrefix], b[:bPrefix])
	if result != 0 {
		return result
	}
	return verrevcmp(a[aPrefix:], b[bPrefix:])
}

// filePrefixLen returns the length of the file name prefix before the
// last extension-like segment (a dot followed by an alpha character or ~).
func filePrefixLen(s []byte) int {
	prefixLen := len(s)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' && i+1 < len(s) &&
			(isAlpha(s[i+1]) || s[i+1] == '~') {
			prefixLen = i
		}
	}
	return prefixLen
}

// verrevcmp implements the gnulib version comparison algorithm.
func verrevcmp(a, b []byte) int {
	ai, bi := 0, 0
	for ai < len(a) || bi < len(b) {
		// Compare non-digit characters using version ordering.
		for (ai < len(a) && !isDigit(a[ai])) || (bi < len(b) && !isDigit(b[bi])) {
			ac := charOrder(a, ai)
			bc := charOrder(b, bi)
			if ac != bc {
				if ac < bc {
					return -1
				}
				return 1
			}
			ai++
			bi++
		}
		// Skip leading zeros in digit runs.
		for ai < len(a) && a[ai] == '0' {
			ai++
		}
		for bi < len(b) && b[bi] == '0' {
			bi++
		}
		// Compare digit runs by length, then by value.
		firstDiff := 0
		for ai < len(a) && bi < len(b) && isDigit(a[ai]) && isDigit(b[bi]) {
			if firstDiff == 0 {
				firstDiff = int(a[ai]) - int(b[bi])
			}
			ai++
			bi++
		}
		if ai < len(a) && isDigit(a[ai]) {
			return 1
		}
		if bi < len(b) && isDigit(b[bi]) {
			return -1
		}
		if firstDiff != 0 {
			if firstDiff < 0 {
				return -1
			}
			return 1
		}
	}
	return 0
}

// charOrder returns the sort order for a character in version comparison,
// following gnulib's filevercmp ordering.
func charOrder(s []byte, pos int) int {
	if pos >= len(s) {
		return 0
	}
	c := s[pos]
	if isDigit(c) {
		return 0
	}
	if isAlpha(c) {
		return int(c)
	}
	if c == '~' {
		return -1
	}
	return int(c) + 256
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// --- R3.2: Key definition parsing ---

// parseKeyDef parses a KEYDEF string from -k into a keyDef.
// R3.2: Format is F[.C][OPTS][,F[.C][OPTS]].
func parseKeyDef(s string) (keyDef, error) {
	kd := keyDef{}
	parts := strings.SplitN(s, ",", 2)

	// Parse start position.
	field, char, rest := parseFieldPos(parts[0])
	kd.startField = field
	kd.startChar = char
	parseKeyOpts(&kd, rest)

	// Parse end position if present.
	if len(parts) > 1 {
		field, char, rest = parseFieldPos(parts[1])
		kd.endField = field
		kd.endChar = char
		parseKeyOpts(&kd, rest)
	}

	if kd.startField < 1 {
		return kd, fmt.Errorf("invalid key field number")
	}
	return kd, nil
}

// parseFieldPos parses F[.C] from the beginning of s, returning the field
// number, character position, and remaining string (modifier letters).
func parseFieldPos(s string) (field, char int, rest string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 {
		field, _ = strconv.Atoi(s[:i])
	}
	rest = s[i:]

	if len(rest) > 0 && rest[0] == '.' {
		rest = rest[1:]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j > 0 {
			char, _ = strconv.Atoi(rest[:j])
		}
		rest = rest[j:]
	}
	return
}

// parseKeyOpts parses modifier letters (n, h, M, V, r, b) from a string
// and applies them to the key definition.
func parseKeyOpts(kd *keyDef, opts string) {
	for _, c := range opts {
		switch c {
		case 'n':
			kd.mode = sortNumeric
			kd.hasMode = true
		case 'h':
			kd.mode = sortHumanNumeric
			kd.hasMode = true
		case 'M':
			kd.mode = sortMonth
			kd.hasMode = true
		case 'V':
			kd.mode = sortVersion
			kd.hasMode = true
		case 'r':
			kd.reverse = true
			kd.hasReverse = true
		case 'b':
			kd.ignoreBlank = true
			kd.hasBlank = true
		}
	}
}

// --- R3.1, R3.2: Key extraction ---

// extractKey extracts the sort key bytes from a line based on the key definition.
// R3.1: Uses the configured field separator or default blank-to-non-blank.
// R3.2: Respects field and character positions from the KEYDEF.
func extractKey(line []byte, kd keyDef, sep byte, hasSep bool, globalIgnoreBlank bool) []byte {
	var fields [][]byte
	if hasSep {
		fields = bytes.Split(line, []byte{sep})
	} else {
		fields = bytes.Fields(line)
	}

	sf := kd.startField - 1
	if sf >= len(fields) {
		return nil
	}

	ef := len(fields) - 1
	if kd.endField > 0 {
		ef = kd.endField - 1
		if ef >= len(fields) {
			ef = len(fields) - 1
		}
	}
	if sf > ef {
		return nil
	}

	ignoreBlank := globalIgnoreBlank
	if kd.hasBlank {
		ignoreBlank = kd.ignoreBlank
	}

	// Single field, no character offsets: return the whole field.
	if sf == ef && kd.startChar <= 1 && kd.endChar == 0 {
		f := fields[sf]
		if ignoreBlank {
			f = bytes.TrimLeft(f, " \t")
		}
		return f
	}

	// Single field with character positions.
	if sf == ef {
		f := fields[sf]
		if ignoreBlank {
			f = bytes.TrimLeft(f, " \t")
		}
		startOff := 0
		if kd.startChar > 1 {
			startOff = kd.startChar - 1
		}
		endOff := len(f)
		if kd.endChar > 0 && kd.endChar < len(f) {
			endOff = kd.endChar
		}
		if startOff >= len(f) || startOff >= endOff {
			return nil
		}
		return f[startOff:endOff]
	}

	// Multi-field range.
	var result []byte
	for i := sf; i <= ef; i++ {
		f := fields[i]
		if i == sf {
			if ignoreBlank {
				f = bytes.TrimLeft(f, " \t")
			}
			if kd.startChar > 1 {
				off := kd.startChar - 1
				if off >= len(f) {
					f = nil
				} else {
					f = f[off:]
				}
			}
		}
		if i == ef && kd.endChar > 0 {
			if kd.endChar < len(f) {
				f = f[:kd.endChar]
			}
		}
		if len(result) > 0 {
			if hasSep {
				result = append(result, sep)
			} else {
				result = append(result, ' ')
			}
		}
		result = append(result, f...)
	}
	return result
}

// --- R3.2, R3.3: Key-based comparison ---

// buildKeyCompareFunc returns a comparison function that compares lines
// using the configured key specifications. Per-key modes and reverse
// settings are applied within the function.
// R3.3: Earlier keys take precedence; later keys break ties.
func buildKeyCompareFunc(cfg *config) func(a, b []byte) int {
	return func(a, b []byte) int {
		for _, kd := range cfg.keys {
			ka := extractKey(a, kd, cfg.separator, cfg.hasSeparator, cfg.ignoreBlanks)
			kb := extractKey(b, kd, cfg.separator, cfg.hasSeparator, cfg.ignoreBlanks)

			mode := cfg.mode
			if kd.hasMode {
				mode = kd.mode
			}

			cmp := buildCompareFunc(mode)
			c := cmp(ka, kb)

			if c != 0 {
				rev := cfg.reverse
				if kd.hasReverse {
					rev = kd.reverse
				}
				if rev {
					c = -c
				}
				return c
			}
		}
		return 0
	}
}

// writeOutput writes sorted lines to the configured output destination.
// D3: When -o specifies the same file as an input, input is already in memory.
func writeOutput(cfg *config, lines [][]byte) error {
	var w io.Writer
	if cfg.outputFile == "" {
		w = os.Stdout
	} else {
		f, err := os.Create(cfg.outputFile)
		if err != nil {
			return fmt.Errorf("open failed: %s: %w", cfg.outputFile, err)
		}
		defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
		w = f
	}

	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.Write(line); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
		if err := bw.WriteByte('\n'); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}
