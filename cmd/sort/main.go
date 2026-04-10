// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sort: sort lines of text files.
// Implements srd053-sort R1.1-R1.7, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "sort"

// restConsumed signals that a value-consuming short flag used the rest
// of the argument cluster (e.g. -t: consumed ":" from the cluster).
const restConsumed = -2

// config holds parsed command-line options from srd053-sort.
type config struct {
	reverse      bool     // R1.4: -r
	unique       bool     // R1.5: -u
	stable       bool     // R1.7: -s
	numericSort  bool     // R2.1: -n
	humanNumeric bool     // R2.2: -h
	monthSort    bool     // R2.3: -M
	versionSort  bool     // R2.4: -V
	generalNum   bool     // -g
	ignoreCase   bool     // -f
	ignoreBlanks bool     // R3.4: -b
	check        bool     // R4.2: -c
	checkQuiet   bool     // R4.2: -C
	zeroTerm     bool     // -z
	outputFile   string   // R1.6: -o
	fieldSep     string   // R3.1: -t
	keys         []string // R3.2, R3.3: -k
	files        []string
	parseErr     bool
	parsedKeys   []keySpec // R3.2: parsed -k specs
	sepByte      byte      // R3.1: field separator byte (0 = default)
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the sort logic and returns the exit code.
// R4.1: returns 0 on success. R4.3: returns 2 on usage errors.
func run(args []string) int {
	cfg := parseArgs(args)
	if cfg.parseErr {
		return 2
	}
	if err := parseKeySpecs(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 2
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	lines, err := readAllLines(&cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 2
	}
	// R4.2: check mode — verify sorted order without producing output.
	if cfg.check || cfg.checkQuiet {
		return checkSorted(&cfg, lines)
	}
	sortLines(lines, &cfg)
	if cfg.unique {
		lines = dedup(lines, dedupFunc(&cfg))
	}
	return writeOutput(&cfg, lines)
}

// parseKeySpecs parses -t and -k flags into sepByte and parsedKeys.
// R3.1: field separator. R3.2, R3.3: key definitions.
func parseKeySpecs(cfg *config) error {
	if cfg.fieldSep != "" {
		cfg.sepByte = cfg.fieldSep[0]
	}
	for _, k := range cfg.keys {
		ks, err := parseKeyDef(k)
		if err != nil {
			return err
		}
		cfg.parsedKeys = append(cfg.parsedKeys, ks)
	}
	return nil
}

// dedupFunc returns the comparison function for dedup based on config.
func dedupFunc(cfg *config) func(string, string) int {
	if len(cfg.parsedKeys) > 0 {
		return func(a, b string) int {
			return compareKeys(a, b, cfg)
		}
	}
	return compareFuncWithBlanks(cfg)
}

// readAllLines reads all input lines from the configured files.
// R1.2: reads stdin when file is "-". R1.3: combines multiple files.
func readAllLines(cfg *config) ([]string, error) {
	var lines []string
	for _, name := range cfg.files {
		fileLines, err := readLinesFromFile(name, cfg.zeroTerm)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

// readLinesFromFile reads lines from a single file or stdin.
func readLinesFromFile(name string, zeroTerm bool) ([]string, error) {
	r, closer, err := openInput(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	defer closer()
	return scanLines(r, zeroTerm)
}

// scanLines reads all lines from r using the appropriate delimiter.
func scanLines(r io.Reader, zeroTerm bool) ([]string, error) {
	scanner := bufio.NewScanner(r)
	if zeroTerm {
		scanner.Split(scanZeroTerminated)
	}
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// scanZeroTerminated is a bufio.SplitFunc that splits on NUL bytes.
func scanZeroTerminated(data []byte, atEOF bool) (int, []byte, error) {
	for i, b := range data {
		if b == 0 {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// openInput opens a file for reading, or returns stdin for "-".
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// writeOutput writes lines to stdout or the configured output file.
// R1.6: -o FILE writes to FILE; FILE may be the same as an input file.
func writeOutput(cfg *config, lines []string) int {
	w, closer, err := openOutput(cfg.outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 2
	}
	defer closer()
	terminator := "\n"
	if cfg.zeroTerm {
		terminator = "\x00"
	}
	if err := writeLines(w, lines, terminator); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		return 2
	}
	return 0
}

// openOutput opens the output destination. Empty string means stdout.
func openOutput(path string) (io.Writer, func(), error) {
	if path == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// writeLines writes each line followed by terminator to w.
func writeLines(w io.Writer, lines []string, terminator string) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		if _, err := bw.WriteString(terminator); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// sortLines sorts lines in-place using stable sort with the configured mode.
// R1.1: default lexicographic sort. R1.4: -r reverses the result.
// R2.1-R2.4: numeric, human-numeric, month, version sort modes.
// R3.2-R3.3: key-based sorting when -k is specified.
func sortLines(lines []string, cfg *config) {
	if len(cfg.parsedKeys) > 0 {
		sortWithKeys(lines, cfg)
		return
	}
	cmp := compareFuncWithBlanks(cfg)
	sort.SliceStable(lines, func(i, j int) bool {
		result := cmp(lines[i], lines[j])
		if result == 0 && !cfg.stable {
			result = strings.Compare(lines[i], lines[j])
		}
		if cfg.reverse {
			return result > 0
		}
		return result < 0
	})
}

// sortWithKeys sorts lines using the key specifications.
// R3.2, R3.3: keys are compared in order; earlier keys take precedence.
// Per-key reverse is applied within compareKeys; last-resort is always ascending.
func sortWithKeys(lines []string, cfg *config) {
	sort.SliceStable(lines, func(i, j int) bool {
		result := compareKeys(lines[i], lines[j], cfg)
		if result == 0 && !cfg.stable {
			result = strings.Compare(lines[i], lines[j])
		}
		return result < 0
	})
}

// compareFuncWithBlanks wraps compareFunc with R3.4 -b support for the
// no-key sort path.
func compareFuncWithBlanks(cfg *config) func(string, string) int {
	base := compareFunc(cfg)
	if !cfg.ignoreBlanks {
		return base
	}
	return func(a, b string) int {
		return base(strings.TrimLeft(a, " \t"), strings.TrimLeft(b, " \t"))
	}
}

// dedup removes consecutive lines that compare equal under the active sort key.
// R1.5: -u outputs only the first of an equal run.
func dedup(lines []string, cmp func(string, string) int) []string {
	if len(lines) == 0 {
		return lines
	}
	out := lines[:1]
	for _, line := range lines[1:] {
		if cmp(line, out[len(out)-1]) != 0 {
			out = append(out, line)
		}
	}
	return out
}

// parseArgs extracts flags and file arguments from the command line.
// Uses manual parsing consistent with other cmd/ packages in this repo.
func parseArgs(args []string) config {
	var cfg config
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			return cfg
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.files = append(cfg.files, arg)
			continue
		}
		consumed := parseLongFlag(&cfg, args, i)
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		consumed = parseShortFlags(&cfg, args, i)
		if consumed > 0 {
			i += consumed - 1
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
		cfg.parseErr = true
		return cfg
	}
	return cfg
}

// parseLongFlag handles --long-form flags. Returns args consumed (0 if not matched).
func parseLongFlag(cfg *config, args []string, i int) int {
	arg := args[i]
	switch {
	case arg == "--reverse":
		cfg.reverse = true
		return 1
	case arg == "--unique":
		cfg.unique = true
		return 1
	case arg == "--stable":
		cfg.stable = true
		return 1
	case arg == "--numeric-sort":
		cfg.numericSort = true
		return 1
	case arg == "--human-numeric-sort":
		cfg.humanNumeric = true
		return 1
	case arg == "--month-sort":
		cfg.monthSort = true
		return 1
	case arg == "--version-sort":
		cfg.versionSort = true
		return 1
	case arg == "--general-numeric-sort":
		cfg.generalNum = true
		return 1
	case arg == "--ignore-case":
		cfg.ignoreCase = true
		return 1
	case arg == "--ignore-leading-blanks":
		cfg.ignoreBlanks = true
		return 1
	case arg == "--check":
		cfg.check = true
		return 1
	case arg == "--check=quiet" || arg == "--check=silent":
		cfg.checkQuiet = true
		return 1
	case arg == "--zero-terminated":
		cfg.zeroTerm = true
		return 1
	}
	return parseLongValueFlag(cfg, args, i)
}

// parseLongValueFlag handles --flag=VALUE and --flag VALUE long forms.
func parseLongValueFlag(cfg *config, args []string, i int) int {
	arg := args[i]
	switch {
	case arg == "--output":
		return setValueFlag(&cfg.outputFile, args, i)
	case strings.HasPrefix(arg, "--output="):
		cfg.outputFile = arg[len("--output="):]
		return 1
	case arg == "--field-separator":
		return setValueFlag(&cfg.fieldSep, args, i)
	case strings.HasPrefix(arg, "--field-separator="):
		cfg.fieldSep = arg[len("--field-separator="):]
		return 1
	case arg == "--key":
		return appendValueFlag(&cfg.keys, args, i)
	case strings.HasPrefix(arg, "--key="):
		cfg.keys = append(cfg.keys, arg[len("--key="):])
		return 1
	}
	return 0
}

// setValueFlag sets dst to the next arg. Returns args consumed or flags error.
func setValueFlag(dst *string, args []string, i int) int {
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "%s: option '%s' requires an argument\n",
			progName, args[i])
		return 0
	}
	*dst = args[i+1]
	return 2
}

// appendValueFlag appends the next arg to dst slice. Returns args consumed.
func appendValueFlag(dst *[]string, args []string, i int) int {
	if i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "%s: option '%s' requires an argument\n",
			progName, args[i])
		return 0
	}
	*dst = append(*dst, args[i+1])
	return 2
}

// parseShortFlags handles -x style flags, including combined short flags
// like -rn and flags with attached values like -oFILE or -kKEYDEF.
func parseShortFlags(cfg *config, args []string, i int) int {
	arg := args[i]
	extra := 0
	for j := 1; j < len(arg); j++ {
		consumed := parseOneShort(cfg, arg[j], arg[j+1:], args, i+1+extra)
		if consumed == -1 {
			return 0 // unknown flag
		}
		if consumed == restConsumed {
			break // value consumed from rest of this arg
		}
		if consumed > 0 {
			extra += consumed
			break // value-consuming flag ends the cluster
		}
	}
	return 1 + extra
}

// parseOneShort handles a single short flag character. Returns:
//
//	0: boolean flag consumed, continue cluster
//	>0: value-consuming flag, skip that many additional args
//	-1: unknown flag
func parseOneShort(cfg *config, ch byte, rest string, args []string, nextIdx int) int {
	switch ch {
	case 'r':
		cfg.reverse = true
	case 'u':
		cfg.unique = true
	case 's':
		cfg.stable = true
	case 'n':
		cfg.numericSort = true
	case 'h':
		cfg.humanNumeric = true
	case 'M':
		cfg.monthSort = true
	case 'V':
		cfg.versionSort = true
	case 'g':
		cfg.generalNum = true
	case 'f':
		cfg.ignoreCase = true
	case 'b':
		cfg.ignoreBlanks = true
	case 'c':
		cfg.check = true
	case 'C':
		cfg.checkQuiet = true
	case 'z':
		cfg.zeroTerm = true
	case 'o':
		return shortValueFlag(&cfg.outputFile, rest, args, nextIdx)
	case 't':
		return shortValueFlag(&cfg.fieldSep, rest, args, nextIdx)
	case 'k':
		return shortKeyFlag(&cfg.keys, rest, args, nextIdx)
	default:
		return -1
	}
	return 0
}

// shortValueFlag extracts the value for a short flag that takes an argument.
// If rest is non-empty, uses rest as the value (e.g. -oFILE). Otherwise
// consumes the next arg.
func shortValueFlag(dst *string, rest string, args []string, nextIdx int) int {
	if rest != "" {
		*dst = rest
		return restConsumed
	}
	if nextIdx < len(args) {
		*dst = args[nextIdx]
		return 1
	}
	return -1
}

// shortKeyFlag extracts the value for -k and appends to the keys slice.
func shortKeyFlag(dst *[]string, rest string, args []string, nextIdx int) int {
	if rest != "" {
		*dst = append(*dst, rest)
		return restConsumed
	}
	if nextIdx < len(args) {
		*dst = append(*dst, args[nextIdx])
		return 1
	}
	return -1
}

// checkSorted verifies that lines are already in sorted order.
// R4.2: -c exits 1 with diagnostic on disorder. -C exits 1 silently.
func checkSorted(cfg *config, lines []string) int {
	for i := 1; i < len(lines); i++ {
		result := comparePair(cfg, lines[i-1], lines[i])
		if result > 0 || (cfg.unique && result == 0) {
			if cfg.check && !cfg.checkQuiet {
				reportDisorder(cfg, i+1, lines[i])
			}
			return 1
		}
	}
	return 0
}

// comparePair compares two adjacent lines using the same ordering as sort.
// Returns negative if a < b, 0 if equal, positive if a > b in sort order.
func comparePair(cfg *config, a, b string) int {
	var result int
	if len(cfg.parsedKeys) > 0 {
		result = compareKeys(a, b, cfg)
	} else {
		result = compareFuncWithBlanks(cfg)(a, b)
	}
	if result == 0 && !cfg.stable {
		result = strings.Compare(a, b)
	}
	// For no-key path, apply global reverse.
	// For key path, reverse is already applied per-key inside compareKeys.
	if len(cfg.parsedKeys) == 0 && cfg.reverse {
		result = -result
	}
	return result
}

// reportDisorder prints the GNU sort-style disorder diagnostic to stderr.
// R4.2: format matches GNU sort: "sort: FILE:LINE: disorder: CONTENT".
func reportDisorder(cfg *config, lineNum int, line string) {
	name := cfg.files[0]
	fmt.Fprintf(os.Stderr, "%s: %s:%d: disorder: %s\n",
		progName, name, lineNum, line)
}
