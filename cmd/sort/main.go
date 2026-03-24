// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R1.1–R1.6:
// core entry point with basic line sorting, reverse (-r), numeric (-n),
// unique (-u), and output file (-o) support.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds parsed command-line state.
type config struct {
	reverse    bool
	numeric    bool
	unique     bool
	stable     bool
	outputFile string
	files      []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg))
}

// run reads input, sorts lines, and writes output.
func run(cfg config) int {
	lines, err := readAllLines(cfg.files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		return 2
	}
	sortLines(lines, cfg)
	if cfg.unique {
		lines = dedup(lines, cfg)
	}
	return writeOutput(lines, cfg.outputFile)
}

// --- Input reading ---

// readAllLines reads lines from files or stdin.
// R1.1: read from stdin when no files or file is "-".
// R1.2: concatenate multiple files in order.
func readAllLines(files []string) ([][]byte, error) {
	if len(files) == 0 {
		return readLines(os.Stdin)
	}
	var all [][]byte
	for _, f := range files {
		lines, err := readFile(f)
		if err != nil {
			return nil, err
		}
		all = append(all, lines...)
	}
	return all, nil
}

// readFile opens a single file (or stdin for "-") and reads its lines.
func readFile(name string) ([][]byte, error) {
	if name == "-" {
		return readLines(os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open failed: %s: %w", name, err)
	}
	defer f.Close() // best-effort close on read path
	return readLines(f)
}

// readLines reads all lines from r, preserving line content without newlines.
func readLines(r io.Reader) ([][]byte, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var lines [][]byte
	for scanner.Scan() {
		line := scanner.Bytes()
		cp := make([]byte, len(line))
		copy(cp, line)
		lines = append(lines, cp)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// --- Sorting ---

// sortLines sorts lines in place according to cfg.
// R1.3: lexicographic byte ordering under LC_ALL=C by default.
// R1.4: -r reverses the order.
// R1.5: -n sorts by leading numeric value.
func sortLines(lines [][]byte, cfg config) {
	cmp := selectCompare(cfg)
	if cfg.stable {
		sort.SliceStable(lines, func(i, j int) bool {
			return cmp(lines[i], lines[j])
		})
	} else {
		sort.Slice(lines, func(i, j int) bool {
			return cmp(lines[i], lines[j])
		})
	}
}

// selectCompare returns the appropriate less-than function.
func selectCompare(cfg config) func(a, b []byte) bool {
	var less func(a, b []byte) bool
	if cfg.numeric {
		less = numericLess
	} else {
		less = byteLess
	}
	if cfg.reverse {
		fwd := less
		less = func(a, b []byte) bool { return fwd(b, a) }
	}
	return less
}

// byteLess compares two lines lexicographically by raw byte values.
// R1.3: LC_ALL=C byte ordering.
func byteLess(a, b []byte) bool {
	return bytes.Compare(a, b) < 0
}

// numericLess compares two lines by leading numeric value.
// R1.5: parse leading whitespace and optional sign, then numeric value.
func numericLess(a, b []byte) bool {
	va := parseLeadingNumber(a)
	vb := parseLeadingNumber(b)
	if va != vb {
		return va < vb
	}
	return bytes.Compare(a, b) < 0
}

// parseLeadingNumber extracts the leading numeric value from a line.
// Matches GNU sort -n behavior: skip leading blanks, parse optional sign
// and decimal number. Non-numeric lines compare as 0.
func parseLeadingNumber(line []byte) float64 {
	s := strings.TrimLeft(string(line), " \t")
	if len(s) == 0 {
		return 0
	}
	end := numericEnd(s)
	if end == 0 {
		return 0
	}
	val, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return 0
	}
	return val
}

// numericEnd finds the end index of a leading numeric value in s.
func numericEnd(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	hasDot := false
	for i < len(s) {
		if s[i] >= '0' && s[i] <= '9' {
			i++
		} else if s[i] == '.' && !hasDot {
			hasDot = true
			i++
		} else {
			break
		}
	}
	if i == start && hasDot {
		return 0
	}
	if i == start && i > 0 {
		return 0 // sign only, no digits
	}
	return i
}

// --- Deduplication ---

// dedup removes consecutive duplicate lines based on the active comparison.
// R1.6: -u suppresses duplicate lines in output.
func dedup(lines [][]byte, cfg config) [][]byte {
	if len(lines) == 0 {
		return lines
	}
	eq := selectEqual(cfg)
	result := [][]byte{lines[0]}
	for i := 1; i < len(lines); i++ {
		if !eq(lines[i-1], lines[i]) {
			result = append(result, lines[i])
		}
	}
	return result
}

// selectEqual returns the equality function matching the active sort mode.
func selectEqual(cfg config) func(a, b []byte) bool {
	if cfg.numeric {
		return func(a, b []byte) bool {
			va := parseLeadingNumber(a)
			vb := parseLeadingNumber(b)
			return va == vb || (math.IsNaN(va) && math.IsNaN(vb))
		}
	}
	return func(a, b []byte) bool {
		return bytes.Equal(a, b)
	}
}

// --- Output ---

// writeOutput writes sorted lines to stdout or the named file.
// R1.3: -o FILE writes to FILE instead of stdout.
func writeOutput(lines [][]byte, outputFile string) int {
	w, closer, err := openOutput(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		return 2
	}
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, wErr := bw.Write(line); wErr != nil {
			fmt.Fprintf(os.Stderr, "sort: write error: %v\n", wErr)
			return 2
		}
		if wErr := bw.WriteByte('\n'); wErr != nil {
			fmt.Fprintf(os.Stderr, "sort: write error: %v\n", wErr)
			return 2
		}
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "sort: write error: %v\n", err)
		return 2
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "sort: close error: %v\n", err)
			return 2
		}
	}
	return 0
}

// openOutput returns a writer and optional closer for the output destination.
func openOutput(path string) (io.Writer, io.Closer, error) {
	if path == "" {
		return os.Stdout, nil, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open '%s': %w", path, err)
	}
	return f, f, nil
}

// --- Argument parsing ---

// parseArgs processes command-line arguments and returns config.
// Exit code -1 means continue; >= 0 means exit immediately.
func parseArgs(args []string) (config, int) {
	var cfg config
	for i := 0; i < len(args); i++ {
		exit := handleArg(args, &i, &cfg)
		if exit >= 0 {
			return config{}, exit
		}
	}
	return cfg, -1
}

// handleArg dispatches a single argument. Returns -1 to continue.
func handleArg(args []string, i *int, cfg *config) int {
	arg := args[*i]
	switch {
	case arg == "--":
		cfg.files = append(cfg.files, args[*i+1:]...)
		*i = len(args)
		return -1
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case strings.HasPrefix(arg, "--output="):
		cfg.outputFile = arg[len("--output="):]
		return -1
	case arg == "--output":
		return parseLongWithValue(args, i, &cfg.outputFile)
	default:
		return handleArgContinued(arg, args, i, cfg)
	}
}

// handleArgContinued handles remaining argument types.
func handleArgContinued(arg string, args []string, i *int, cfg *config) int {
	switch {
	case parseLongBool(arg, cfg):
		return -1
	case strings.HasPrefix(arg, "-") && arg != "-" && arg[1] != '-':
		return parseShortFlags(arg[1:], args, i, cfg)
	default:
		cfg.files = append(cfg.files, arg)
		return -1
	}
}

// parseLongBool handles long boolean flags. Returns true if matched.
func parseLongBool(arg string, cfg *config) bool {
	switch arg {
	case "--reverse":
		cfg.reverse = true
	case "--numeric-sort":
		cfg.numeric = true
	case "--unique":
		cfg.unique = true
	case "--stable":
		cfg.stable = true
	default:
		return false
	}
	return true
}

// parseLongWithValue handles --flag VALUE (space-separated).
func parseLongWithValue(args []string, i *int, dest *string) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "sort: option '%s' requires an argument\n", args[*i])
		return 2
	}
	*i++
	*dest = args[*i]
	return -1
}

// parseShortFlags processes clustered short flags (e.g., -rnu).
// Returns -1 to continue, >= 0 to exit with that code.
func parseShortFlags(flags string, args []string, i *int, cfg *config) int {
	for j := 0; j < len(flags); j++ {
		ch := flags[j]
		if ch == 'o' {
			return consumeOutputFlag(flags, j, args, i, cfg)
		}
		exit := applyBoolFlag(ch, cfg)
		if exit >= 0 {
			return exit
		}
	}
	return -1
}

// applyBoolFlag applies a single boolean short flag character.
func applyBoolFlag(ch byte, cfg *config) int {
	switch ch {
	case 'r':
		cfg.reverse = true
	case 'n':
		cfg.numeric = true
	case 'u':
		cfg.unique = true
	case 's':
		cfg.stable = true
	default:
		fmt.Fprintf(os.Stderr, "sort: invalid option -- '%c'\n", ch)
		return 2
	}
	return -1
}

// consumeOutputFlag handles -o with value from remaining flags or next arg.
func consumeOutputFlag(
	flags string, j int, args []string, i *int, cfg *config,
) int {
	rest := flags[j+1:]
	if rest != "" {
		cfg.outputFile = rest
		return -1
	}
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "sort: option requires an argument -- 'o'\n")
		return 2
	}
	*i++
	cfg.outputFile = args[*i]
	return -1
}

// --- Help and version ---

// printHelp writes usage information and returns exit code 0.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: sort [OPTION]... [FILE]...
Write sorted concatenation of all FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

Ordering options:

  -n, --numeric-sort          compare according to string numerical value
  -r, --reverse               reverse the result of comparisons

Other options:

  -o, --output=FILE   write result to FILE instead of standard output
  -s, --stable        stabilize sort by disabling last-resort comparison
  -u, --unique        with -c, check for strict ordering;
                        without -c, output only the first of an equal run
      --help     display this help and exit
      --version  output version information and exit
`)
	return 0
}

// printVersion writes version information and returns exit code 0.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "sort (go-unix-utils) %s\n", version)
	return 0
}
