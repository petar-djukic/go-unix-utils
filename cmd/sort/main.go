// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R1.1–R1.7: basic sorting and output control.
// Implements prd053-sort R2.1: numeric sort mode (-n).
// Implements prd053-sort R3.1–R3.3: key fields and delimiters.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "sort"

// sortKey represents a single -k key specification (R3.2).
type sortKey struct {
	startField int  // 1-based start field number
	startChar  int  // 1-based char offset within start field (0 = start)
	endField   int  // 1-based end field number (0 = end of line)
	endChar    int  // 1-based char offset within end field (0 = end)
	numeric    bool // n modifier: numeric comparison
	reverse    bool // r modifier: reverse this key
	hasOpts    bool // true if any modifiers were specified on this key
}

// options holds parsed sort flags.
type options struct {
	reverse    bool      // -r, --reverse (R1.4)
	unique     bool      // -u, --unique (R1.5)
	outputFile string    // -o, --output (R1.6)
	stable     bool      // -s, --stable (R1.7)
	numeric    bool      // -n, --numeric-sort (R2.1)
	fieldSep   string    // -t, --field-separator (R3.1)
	keys       []sortKey // -k, --key (R3.2, R3.3)
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and processes files, returning the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	lines, readErr := readAllFiles(files, stdin, stderr)
	sortLines(lines, opts)
	if opts.unique {
		lines = dedupLines(lines, opts)
	}
	return writeOutput(lines, opts, stdout, stderr, readErr)
}

// parseArgs separates flags from file arguments.
// Returns parsed options, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (options, []string, int) {
	var opts options
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "-" {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			newI, code := applyLongFlag(&opts, arg, args, i, stdout, stderr)
			if code >= 0 {
				return opts, nil, code
			}
			i = newI
			continue
		}
		newI, code := applyShortFlags(&opts, arg, args, i, stderr)
		if code >= 0 {
			return opts, nil, code
		}
		i = newI
	}
	return opts, files, -1
}

// applyShortFlags processes a short flag group (e.g., "-rnk2,2").
// Returns the updated args index and exit code (-1 = continue).
func applyShortFlags(o *options, arg string, args []string, i int, stderr io.Writer) (int, int) {
	for j := 1; j < len(arg); j++ {
		ch := arg[j]
		switch ch {
		case 'o':
			return handleShortArgFlag(arg, j, args, i, ch, stderr, func(val string) int {
				o.outputFile = val
				return -1
			})
		case 'k':
			return handleShortArgFlag(arg, j, args, i, ch, stderr, func(val string) int {
				return addKeyFromSpec(o, val, stderr)
			})
		case 't':
			return handleShortArgFlag(arg, j, args, i, ch, stderr, func(val string) int {
				o.fieldSep = val
				return -1
			})
		default:
			if !applyShortFlag(o, ch) {
				fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, ch)
				printTryHelp(stderr)
				return i, 2
			}
		}
	}
	return i, -1
}

// handleShortArgFlag consumes an argument for a short flag and calls apply.
func handleShortArgFlag(arg string, j int, args []string, i int, ch byte, stderr io.Writer, apply func(string) int) (int, int) {
	val, newI, code := consumeShortArg(arg, j, args, i, ch, stderr)
	if code >= 0 {
		return newI, code
	}
	return newI, apply(val)
}

// consumeShortArg extracts the argument for a short flag that requires one.
// The value is either the rest of the current arg or the next arg.
func consumeShortArg(arg string, j int, args []string, i int, ch byte, stderr io.Writer) (string, int, int) {
	if j+1 < len(arg) {
		return arg[j+1:], i, -1
	}
	if i+1 < len(args) {
		return args[i+1], i + 1, -1
	}
	fmt.Fprintf(stderr, "%s: option requires an argument -- '%c'\n", progName, ch)
	printTryHelp(stderr)
	return "", i, 2
}

// applyShortFlag applies a single-character flag to options.
// Returns false for unrecognized flags.
func applyShortFlag(o *options, ch byte) bool {
	switch ch {
	case 'r':
		o.reverse = true
	case 'u':
		o.unique = true
	case 's':
		o.stable = true
	case 'n':
		o.numeric = true
	default:
		return false
	}
	return true
}

// applyLongFlag handles --long-name flags.
// Returns the updated args index and exit code (-1 = continue).
func applyLongFlag(o *options, arg string, args []string, i int, stdout, stderr io.Writer) (int, int) {
	switch {
	case arg == "--reverse":
		o.reverse = true
	case arg == "--unique":
		o.unique = true
	case arg == "--stable":
		o.stable = true
	case arg == "--numeric-sort":
		o.numeric = true
	case arg == "--help":
		printHelp(stdout)
		return i, 0
	case arg == "--version":
		printVersion(stdout)
		return i, 0
	case arg == "--output" || strings.HasPrefix(arg, "--output="):
		return applyLongOutput(o, arg, args, i, stderr)
	case arg == "--key" || strings.HasPrefix(arg, "--key="):
		return applyLongKey(o, arg, args, i, stderr)
	case arg == "--field-separator" || strings.HasPrefix(arg, "--field-separator="):
		return applyLongSep(o, arg, args, i, stderr)
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return i, 2
	}
	return i, -1
}

// applyLongOutput handles --output and --output=FILE.
func applyLongOutput(o *options, arg string, args []string, i int, stderr io.Writer) (int, int) {
	val, newI, code := consumeLongArg(arg, "--output", args, i, stderr)
	if code >= 0 {
		return newI, code
	}
	o.outputFile = val
	return newI, -1
}

// applyLongKey handles --key and --key=KEYDEF.
func applyLongKey(o *options, arg string, args []string, i int, stderr io.Writer) (int, int) {
	val, newI, code := consumeLongArg(arg, "--key", args, i, stderr)
	if code >= 0 {
		return newI, code
	}
	return newI, addKeyFromSpec(o, val, stderr)
}

// applyLongSep handles --field-separator and --field-separator=CHAR.
func applyLongSep(o *options, arg string, args []string, i int, stderr io.Writer) (int, int) {
	val, newI, code := consumeLongArg(arg, "--field-separator", args, i, stderr)
	if code >= 0 {
		return newI, code
	}
	o.fieldSep = val
	return newI, -1
}

// consumeLongArg extracts the argument for a long flag (--flag=VAL or --flag VAL).
func consumeLongArg(arg, prefix string, args []string, i int, stderr io.Writer) (string, int, int) {
	eqForm := prefix + "="
	if strings.HasPrefix(arg, eqForm) {
		return arg[len(eqForm):], i, -1
	}
	if i+1 < len(args) {
		return args[i+1], i + 1, -1
	}
	fmt.Fprintf(stderr, "%s: option '%s' requires an argument\n", progName, prefix)
	printTryHelp(stderr)
	return "", i, 2
}

// addKeyFromSpec parses a key specification and appends it to options.
// Returns exit code (-1 = success).
func addKeyFromSpec(o *options, spec string, stderr io.Writer) int {
	k, err := parseSortKey(spec)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		printTryHelp(stderr)
		return 2
	}
	o.keys = append(o.keys, k)
	return -1
}

// parseSortKey parses a -k KEYDEF specification (R3.2).
// Format: F[.C][OPTS][,F[.C][OPTS]].
func parseSortKey(spec string) (sortKey, error) {
	parts := strings.SplitN(spec, ",", 2)
	startField, startChar, startMods, err := parseKeyPos(parts[0])
	if err != nil {
		return sortKey{}, fmt.Errorf("invalid key specification: %s", spec)
	}
	k := sortKey{startField: startField, startChar: startChar}
	applyKeyMods(&k, startMods)
	if len(parts) == 2 {
		endField, endChar, endMods, err := parseKeyPos(parts[1])
		if err != nil {
			return sortKey{}, fmt.Errorf("invalid key specification: %s", spec)
		}
		k.endField = endField
		k.endChar = endChar
		applyKeyMods(&k, endMods)
	}
	return k, nil
}

// parseKeyPos parses one side of a key specification (e.g., "2.3n").
// Returns field number, char offset, modifier string, and error.
func parseKeyPos(s string) (int, int, string, error) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, "", fmt.Errorf("missing field number")
	}
	field, _ := strconv.Atoi(s[:i])
	if field == 0 {
		return 0, 0, "", fmt.Errorf("field number must be positive")
	}
	charPos := 0
	if i < len(s) && s[i] == '.' {
		i++
		j := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if j < i {
			charPos, _ = strconv.Atoi(s[j:i])
		}
	}
	return field, charPos, s[i:], nil
}

// applyKeyMods applies modifier letters to a sort key.
func applyKeyMods(k *sortKey, mods string) {
	for _, ch := range mods {
		switch ch {
		case 'n':
			k.numeric = true
			k.hasOpts = true
		case 'r':
			k.reverse = true
			k.hasOpts = true
		}
	}
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Write sorted concatenation of all FILE(s) to standard output.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -k, --key=KEYDEF         sort via a key; KEYDEF gives location and type")
	fmt.Fprintln(w, "  -n, --numeric-sort        compare according to string numerical value")
	fmt.Fprintln(w, "  -o, --output=FILE         write result to FILE instead of standard output")
	fmt.Fprintln(w, "  -r, --reverse             reverse the result of comparisons")
	fmt.Fprintln(w, "  -s, --stable              stabilize sort by disabling last-resort comparison")
	fmt.Fprintln(w, "  -t, --field-separator=SEP use SEP instead of non-blank to blank transition")
	fmt.Fprintln(w, "  -u, --unique              with default sort, output only the first of an equal run")
	fmt.Fprintln(w, "      --help                display this help and exit")
	fmt.Fprintln(w, "      --version             output version information and exit")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// readAllFiles reads lines from all named files into a single slice.
// R1.2: "-" reads from stdin. R1.3: multiple files combined.
func readAllFiles(files []string, stdin io.Reader, stderr io.Writer) ([]string, int) {
	var lines []string
	exitCode := 0
	for _, name := range files {
		fileLines, err := readOneFile(name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 2
			continue
		}
		lines = append(lines, fileLines...)
	}
	return lines, exitCode
}

// readOneFile reads all lines from a single file or stdin.
func readOneFile(name string, stdin io.Reader) ([]string, error) {
	if name == "-" {
		return readLines(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // best-effort close on read-only file
	return readLines(f)
}

// readLines reads all lines from a reader.
func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// sortLines sorts the lines according to the active options.
// R1.7: uses stable sort when -s is given, unstable otherwise.
func sortLines(lines []string, opts options) {
	less := buildLessFunc(lines, opts)
	if opts.stable || opts.unique {
		sort.SliceStable(lines, less)
	} else {
		sort.Slice(lines, less)
	}
}

// buildLessFunc returns the comparison function for sorting.
func buildLessFunc(lines []string, opts options) func(i, j int) bool {
	if len(opts.keys) == 0 {
		return buildWholeLineLess(lines, opts)
	}
	return buildKeyLess(lines, opts)
}

// buildWholeLineLess returns a comparison for whole-line sorting.
// R1.1: lexicographic. R1.4: reverse. R2.1: numeric.
func buildWholeLineLess(lines []string, opts options) func(i, j int) bool {
	if opts.numeric {
		return func(i, j int) bool {
			cmp := compareNumeric(lines[i], lines[j])
			if opts.reverse {
				return cmp > 0
			}
			return cmp < 0
		}
	}
	if opts.reverse {
		return func(i, j int) bool { return lines[i] > lines[j] }
	}
	return func(i, j int) bool { return lines[i] < lines[j] }
}

// buildKeyLess returns a comparison using key specifications (R3.2, R3.3).
func buildKeyLess(lines []string, opts options) func(i, j int) bool {
	return func(i, j int) bool {
		for ki := range opts.keys {
			cmp := compareOneKey(lines[i], lines[j], opts.keys[ki], opts)
			if cmp != 0 {
				return cmp < 0
			}
		}
		if opts.stable || opts.unique {
			return false
		}
		return lines[i] < lines[j]
	}
}

// splitFields splits a line into fields using the configured separator.
// R3.1: -t uses the specified character; default splits on whitespace runs.
func splitFields(line, sep string) []string {
	if sep != "" {
		return strings.Split(line, sep)
	}
	return strings.Fields(line)
}

// extractKey extracts the sort key substring from a line (R3.2).
func extractKey(line string, k sortKey, sep string) string {
	fields := splitFields(line, sep)
	si := k.startField - 1
	if si >= len(fields) {
		return ""
	}
	ei := len(fields) - 1
	if k.endField > 0 {
		ei = k.endField - 1
		if ei >= len(fields) {
			ei = len(fields) - 1
		}
	}
	if si > ei {
		return ""
	}
	if si == ei {
		return charSlice(fields[si], k.startChar, k.endChar)
	}
	return multiFieldKey(fields, si, ei, k)
}

// charSlice extracts a character range from a string.
// startChar and endChar are 1-based; 0 means no limit on that side.
func charSlice(s string, startChar, endChar int) string {
	sc := 0
	if startChar > 1 {
		sc = startChar - 1
	}
	ec := len(s)
	if endChar > 0 && endChar <= len(s) {
		ec = endChar
	}
	if sc >= len(s) || sc >= ec {
		return ""
	}
	return s[sc:ec]
}

// multiFieldKey joins fields from si to ei with character trimming.
func multiFieldKey(fields []string, si, ei int, k sortKey) string {
	var parts []string
	for i := si; i <= ei; i++ {
		f := fields[i]
		if i == si && k.startChar > 1 {
			off := k.startChar - 1
			if off >= len(f) {
				continue
			}
			f = f[off:]
		}
		if i == ei && k.endChar > 0 && k.endChar < len(f) {
			f = f[:k.endChar]
		}
		parts = append(parts, f)
	}
	return strings.Join(parts, " ")
}

// compareOneKey compares two lines by a single sort key (R3.2).
// Keys with explicit modifiers use their own flags; keys without
// modifiers inherit the global flags (R3.3).
func compareOneKey(a, b string, k sortKey, opts options) int {
	ka := extractKey(a, k, opts.fieldSep)
	kb := extractKey(b, k, opts.fieldSep)
	isNumeric := k.numeric
	isReverse := k.reverse
	if !k.hasOpts {
		isNumeric = opts.numeric
		isReverse = opts.reverse
	}
	var cmp int
	if isNumeric {
		cmp = compareNumeric(ka, kb)
	} else {
		cmp = strings.Compare(ka, kb)
	}
	if isReverse {
		return -cmp
	}
	return cmp
}

// compareNumeric compares two strings as numeric values (R2.1).
func compareNumeric(a, b string) int {
	fa := parseNumeric(a)
	fb := parseNumeric(b)
	switch {
	case fa < fb:
		return -1
	case fa > fb:
		return 1
	default:
		return 0
	}
}

// parseNumeric extracts the leading numeric value from a string.
// Handles leading whitespace, optional sign, integer and decimal parts.
// Returns 0 for non-numeric strings, matching GNU sort -n behavior.
func parseNumeric(s string) float64 {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return 0
	}
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	hasDigit := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		hasDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			hasDigit = true
			i++
		}
	}
	if !hasDigit {
		return 0
	}
	f, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	return f
}

// dedupLines removes adjacent lines that compare equal by key (R1.5).
func dedupLines(lines []string, opts options) []string {
	if len(lines) == 0 {
		return lines
	}
	result := []string{lines[0]}
	for i := 1; i < len(lines); i++ {
		if !linesEqualByKey(lines[i], lines[i-1], opts) {
			result = append(result, lines[i])
		}
	}
	return result
}

// linesEqualByKey checks whether two lines are equal under the active keys.
func linesEqualByKey(a, b string, opts options) bool {
	if len(opts.keys) == 0 {
		if opts.numeric {
			return compareNumeric(a, b) == 0
		}
		return a == b
	}
	for i := range opts.keys {
		if compareOneKey(a, b, opts.keys[i], opts) != 0 {
			return false
		}
	}
	return true
}

// writeOutput writes sorted lines to the appropriate destination.
// R1.6: writes to -o file if specified, otherwise stdout.
func writeOutput(lines []string, opts options, stdout, stderr io.Writer, readErr int) int {
	if opts.outputFile != "" {
		return writeToFile(lines, opts.outputFile, stderr, readErr)
	}
	if writeErr := writeLines(lines, stdout); writeErr != 0 {
		return writeErr
	}
	return readErr
}

// writeToFile writes lines to the named file. R1.6: -o FILE.
func writeToFile(lines []string, path string, stderr io.Writer, readErr int) int {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: open failed: %s: %s\n", progName, path, unwrapPathError(err))
		return 2
	}
	writeErr := writeLines(lines, f)
	if cerr := f.Close(); cerr != nil && writeErr == 0 {
		fmt.Fprintf(stderr, "%s: write failed: %s: %s\n", progName, path, cerr)
		return 2
	}
	if writeErr != 0 {
		return writeErr
	}
	return readErr
}

// writeLines writes all lines to w, each followed by a newline.
func writeLines(lines []string, w io.Writer) int {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return 2
		}
		if err := bw.WriteByte('\n'); err != nil {
			return 2
		}
	}
	if err := bw.Flush(); err != nil {
		return 2
	}
	return 0
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
