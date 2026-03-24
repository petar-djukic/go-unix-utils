// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R1.1–R1.7, R3.1–R3.3:
// core entry point with basic line sorting, reverse (-r), numeric (-n),
// unique (-u), output file (-o), stable (-s), field separator (-t),
// and key definitions (-k).
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds parsed command-line state.
type config struct {
	reverse    bool
	numeric    bool
	unique     bool
	stable     bool
	outputFile string
	separator  string    // R3.1: -t field separator
	keys       []sortKey // R3.2: -k key definitions
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
func sortLines(lines [][]byte, cfg config) {
	cmp := makeCompare(cfg)
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

// --- Deduplication ---

// dedup removes consecutive duplicate lines based on the active comparison.
// R1.6: -u suppresses duplicate lines in output.
func dedup(lines [][]byte, cfg config) [][]byte {
	if len(lines) == 0 {
		return lines
	}
	eq := makeEqual(cfg)
	result := [][]byte{lines[0]}
	for i := 1; i < len(lines); i++ {
		if !eq(lines[i-1], lines[i]) {
			result = append(result, lines[i])
		}
	}
	return result
}

// --- Output ---

// writeOutput writes sorted lines to stdout or the named file.
func writeOutput(lines [][]byte, outputFile string) int {
	w, closer, err := openOutput(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		return 2
	}
	if code := writeLines(w, lines); code != 0 {
		return code
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "sort: close error: %v\n", err)
			return 2
		}
	}
	return 0
}

// writeLines writes all lines to w with newline terminators.
func writeLines(w io.Writer, lines [][]byte) int {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.Write(line); err != nil {
			fmt.Fprintf(os.Stderr, "sort: write error: %v\n", err)
			return 2
		}
		if err := bw.WriteByte('\n'); err != nil {
			fmt.Fprintf(os.Stderr, "sort: write error: %v\n", err)
			return 2
		}
	}
	if err := bw.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "sort: write error: %v\n", err)
		return 2
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
		return handleArgLong(arg, args, i, cfg)
	}
}

// handleArgLong handles long options for keys, separator, and booleans.
func handleArgLong(arg string, args []string, i *int, cfg *config) int {
	switch {
	case strings.HasPrefix(arg, "--field-separator="):
		cfg.separator = arg[len("--field-separator="):]
		return -1
	case arg == "--field-separator":
		return parseLongWithValue(args, i, &cfg.separator)
	case strings.HasPrefix(arg, "--key="):
		return addKeyDef(arg[len("--key="):], cfg)
	case arg == "--key":
		return parseLongKey(args, i, cfg)
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

// parseLongKey handles --key VALUE (space-separated).
func parseLongKey(args []string, i *int, cfg *config) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "sort: option '--key' requires an argument\n")
		return 2
	}
	*i++
	return addKeyDef(args[*i], cfg)
}

// addKeyDef parses a key definition string and appends it to cfg.
func addKeyDef(s string, cfg *config) int {
	key, err := parseKeyDef(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sort: %v\n", err)
		return 2
	}
	cfg.keys = append(cfg.keys, key)
	return -1
}

// parseShortFlags processes clustered short flags (e.g., -rnu).
// Returns -1 to continue, >= 0 to exit with that code.
func parseShortFlags(flags string, args []string, i *int, cfg *config) int {
	for j := 0; j < len(flags); j++ {
		ch := flags[j]
		switch ch {
		case 'o':
			return consumeValueFlag(flags, j, args, i, &cfg.outputFile, ch)
		case 't':
			return consumeValueFlag(flags, j, args, i, &cfg.separator, ch)
		case 'k':
			return consumeKeyFlag(flags, j, args, i, cfg)
		default:
			exit := applyBoolFlag(ch, cfg)
			if exit >= 0 {
				return exit
			}
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

// consumeValueFlag handles a short flag that takes a value (-o, -t).
func consumeValueFlag(
	flags string, j int, args []string, i *int, dest *string, ch byte,
) int {
	rest := flags[j+1:]
	if rest != "" {
		*dest = rest
		return -1
	}
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "sort: option requires an argument -- '%c'\n", ch)
		return 2
	}
	*i++
	*dest = args[*i]
	return -1
}

// consumeKeyFlag handles -k with value from remaining flags or next arg.
func consumeKeyFlag(
	flags string, j int, args []string, i *int, cfg *config,
) int {
	rest := flags[j+1:]
	if rest != "" {
		return addKeyDef(rest, cfg)
	}
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "sort: option requires an argument -- 'k'\n")
		return 2
	}
	*i++
	return addKeyDef(args[*i], cfg)
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

Key options:

  -k, --key=KEYDEF            sort via a key; KEYDEF gives location and type
  -t, --field-separator=SEP   use SEP instead of non-blank to blank transition

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
