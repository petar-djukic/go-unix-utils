// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/shuf implements GNU shuf: shuffle lines randomly.
// Implements prd064-shuf R1.1-R1.4, R2.1-R2.4, R3.1, R3.4.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "shuf"

// config holds parsed command-line options.
type config struct {
	headCount    int
	hasHeadCount bool
	outputFile   string
	zeroTerm     bool
	repeat       bool
	inputRange   string
	rangeLo      int
	rangeHi      int
	randomSource string
	files        []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		exitWithError(err.Error())
	}
	os.Exit(runShuf(cfg))
}

// parseArgs parses command-line arguments into a config.
// R1.1: parse flags and file arguments.
func parseArgs(args []string) (*config, error) {
	cfg := &config{}
	i := 0
	for i < len(args) {
		if args[i] == "--" {
			i++
			break
		}
		if !strings.HasPrefix(args[i], "-") || args[i] == "-" {
			break
		}
		consumed, err := parseFlag(cfg, args[i:])
		if err != nil {
			return nil, err
		}
		i += consumed
	}
	cfg.files = args[i:]
	return cfg, validateParsedArgs(cfg)
}

// validateParsedArgs validates parsed arguments for consistency.
func validateParsedArgs(cfg *config) error {
	if cfg.inputRange == "" {
		return nil
	}
	if err := parseRange(cfg); err != nil {
		return err
	}
	if len(cfg.files) > 0 {
		return fmt.Errorf("extra operand '%s'", cfg.files[0])
	}
	return nil
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, args []string) (int, error) {
	if strings.HasPrefix(args[0], "--") {
		return parseLongFlag(cfg, args)
	}
	return parseShortFlag(cfg, args)
}

// parseLongFlag handles --prefixed flags.
func parseLongFlag(cfg *config, args []string) (int, error) {
	arg := args[0]
	switch {
	case arg == "--help":
		printHelp()
		os.Exit(0)
	case arg == "--version":
		printVersion()
		os.Exit(0)
	case arg == "--zero-terminated":
		cfg.zeroTerm = true
		return 1, nil
	case arg == "--repeat":
		cfg.repeat = true
		return 1, nil
	case strings.HasPrefix(arg, "--head-count="):
		return parseHeadCountValue(cfg, arg[len("--head-count="):])
	case arg == "--head-count":
		return parseHeadCountNext(cfg, args)
	case strings.HasPrefix(arg, "--output="):
		cfg.outputFile = arg[len("--output="):]
		return 1, nil
	case arg == "--output":
		return parseOutputNext(cfg, args)
	case strings.HasPrefix(arg, "--input-range="):
		cfg.inputRange = arg[len("--input-range="):]
		return 1, nil
	case arg == "--input-range":
		return parseInputRangeNext(cfg, args)
	case strings.HasPrefix(arg, "--random-source="):
		cfg.randomSource = arg[len("--random-source="):]
		return 1, nil
	case arg == "--random-source":
		return parseRandomSourceNext(cfg, args)
	}
	return 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// parseShortFlag handles single-dash flags.
func parseShortFlag(cfg *config, args []string) (int, error) {
	arg := args[0]
	switch {
	case arg == "-z":
		cfg.zeroTerm = true
		return 1, nil
	case arg == "-r":
		cfg.repeat = true
		return 1, nil
	case arg == "-n":
		return parseHeadCountNext(cfg, args)
	case strings.HasPrefix(arg, "-n"):
		return parseHeadCountValue(cfg, arg[2:])
	case arg == "-o":
		return parseOutputNext(cfg, args)
	case strings.HasPrefix(arg, "-o"):
		cfg.outputFile = arg[2:]
		return 1, nil
	case arg == "-i":
		return parseInputRangeNext(cfg, args)
	case strings.HasPrefix(arg, "-i"):
		cfg.inputRange = arg[2:]
		return 1, nil
	}
	return 0, fmt.Errorf("unrecognized option '%s'", arg)
}

// parseHeadCountValue parses an inline head-count value.
func parseHeadCountValue(cfg *config, val string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid line count: '%s'", val)
	}
	cfg.headCount = n
	cfg.hasHeadCount = true
	return 1, nil
}

// parseHeadCountNext parses -n COUNT where the value is the next arg.
func parseHeadCountNext(cfg *config, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("option requires an argument -- 'n'")
	}
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid line count: '%s'", args[1])
	}
	cfg.headCount = n
	cfg.hasHeadCount = true
	return 2, nil
}

// parseOutputNext parses -o FILE where the value is the next arg.
func parseOutputNext(cfg *config, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("option requires an argument -- 'o'")
	}
	cfg.outputFile = args[1]
	return 2, nil
}

// parseInputRangeNext parses -i LO-HI where the value is the next arg.
func parseInputRangeNext(cfg *config, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("option requires an argument -- 'i'")
	}
	cfg.inputRange = args[1]
	return 2, nil
}

// parseRandomSourceNext parses --random-source FILE where the value is the next arg.
func parseRandomSourceNext(cfg *config, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("option requires an argument -- 'random-source'")
	}
	cfg.randomSource = args[1]
	return 2, nil
}

// parseRange parses the LO-HI input range string.
// R2.1: generate integers from LO to HI inclusive.
func parseRange(cfg *config) error {
	parts := strings.SplitN(cfg.inputRange, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid input range: '%s'", cfg.inputRange)
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid input range: '%s'", cfg.inputRange)
	}
	hi, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid input range: '%s'", cfg.inputRange)
	}
	if lo > hi {
		return fmt.Errorf("invalid input range: '%s'", cfg.inputRange)
	}
	cfg.rangeLo = lo
	cfg.rangeHi = hi
	return nil
}

// runShuf executes the shuffle operation.
func runShuf(cfg *config) int {
	lines, err := getLines(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	if len(lines) == 0 {
		return 0
	}
	rng, err := makeRand(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	if cfg.repeat {
		return writeRepeatOutput(cfg, lines, rng)
	}
	shuffleLines(lines, rng)
	return writeOutput(cfg, lines)
}

// getLines returns input lines from range generation or file/stdin reading.
func getLines(cfg *config) ([]string, error) {
	if cfg.inputRange != "" {
		return generateRange(cfg), nil
	}
	return readLines(cfg)
}

// generateRange produces a slice of integer strings from rangeLo to rangeHi.
// R2.1: generate integers from LO to HI inclusive.
func generateRange(cfg *config) []string {
	n := cfg.rangeHi - cfg.rangeLo + 1
	lines := make([]string, n)
	for i := range n {
		lines[i] = strconv.Itoa(cfg.rangeLo + i)
	}
	return lines
}

// readLines reads all input lines from files or stdin.
// R1.2: read from stdin when no file arguments or "-".
// R1.4: handle lines terminated by newline; include last line without terminator.
func readLines(cfg *config) ([]string, error) {
	delim := byte('\n')
	if cfg.zeroTerm {
		delim = 0
	}
	if len(cfg.files) == 0 {
		return readFromReader(os.Stdin, delim)
	}
	var allLines []string
	for _, f := range cfg.files {
		lines, err := readFileLines(f, delim)
		if err != nil {
			return nil, err
		}
		allLines = append(allLines, lines...)
	}
	return allLines, nil
}

// readFileLines reads lines from a single file or stdin if "-".
func readFileLines(name string, delim byte) ([]string, error) {
	if name == "-" {
		return readFromReader(os.Stdin, delim)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	defer f.Close()
	return readFromReader(f, delim)
}

// readFromReader reads all content and splits by delimiter.
func readFromReader(r io.Reader, delim byte) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	// R1.4: strip trailing delimiter to avoid empty trailing element.
	data = bytes.TrimSuffix(data, []byte{delim})
	if len(data) == 0 {
		return nil, nil
	}
	parts := bytes.Split(data, []byte{delim})
	lines := make([]string, len(parts))
	for i, p := range parts {
		lines[i] = string(p)
	}
	return lines, nil
}

// makeRand creates a random number generator from the random source file.
// R3.1: --random-source=FILE uses FILE as random byte source.
func makeRand(cfg *config) (*rand.Rand, error) {
	if cfg.randomSource == "" {
		return nil, nil
	}
	data, err := os.ReadFile(cfg.randomSource)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", cfg.randomSource, err)
	}
	seed := sha256.Sum256(data)
	return rand.New(rand.NewChaCha8(seed)), nil
}

// randIntn returns a random int in [0, n) using rng or the global source.
func randIntn(rng *rand.Rand, n int) int {
	if rng == nil {
		return rand.IntN(n)
	}
	return rng.IntN(n)
}

// shuffleLines randomly permutes lines in place.
// R1.3: produces a permutation (each line exactly once).
func shuffleLines(lines []string, rng *rand.Rand) {
	if rng == nil {
		rand.Shuffle(len(lines), func(i, j int) {
			lines[i], lines[j] = lines[j], lines[i]
		})
		return
	}
	rng.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
}

// writeOutput writes shuffled lines to stdout or the output file.
// R2.2: -n limits output count. R2.4: -o writes to a named file.
func writeOutput(cfg *config, lines []string) int {
	w, cleanup, err := openOutput(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	defer cleanup()
	count := len(lines)
	if cfg.hasHeadCount && cfg.headCount < count {
		count = cfg.headCount
	}
	delim := outputDelim(cfg)
	for i := 0; i < count; i++ {
		if _, err := fmt.Fprintf(w, "%s%c", lines[i], delim); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
			return 1
		}
	}
	return 0
}

// writeRepeatOutput writes lines with replacement, possibly indefinitely.
// R2.3: -r allows repeated output lines. Without -n, runs until killed.
func writeRepeatOutput(cfg *config, lines []string, rng *rand.Rand) int {
	w, cleanup, err := openOutput(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	defer cleanup()
	delim := outputDelim(cfg)
	n := len(lines)
	count := 0
	for !cfg.hasHeadCount || count < cfg.headCount {
		idx := randIntn(rng, n)
		if _, err := fmt.Fprintf(w, "%s%c", lines[idx], delim); err != nil {
			return 1
		}
		count++
	}
	return 0
}

// openOutput returns a writer for the output destination.
func openOutput(cfg *config) (io.Writer, func(), error) {
	if cfg.outputFile == "" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(cfg.outputFile)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// outputDelim returns the output line delimiter.
func outputDelim(cfg *config) byte {
	if cfg.zeroTerm {
		return 0
	}
	return '\n'
}

// printHelp outputs usage information to stdout.
// R3.4: --help prints usage and exits 0.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION]... [FILE]\n", progName)
	fmt.Printf("  or:  %s -i LO-HI [OPTION]...\n", progName)
	fmt.Printf("  or:  %s -e [OPTION]... [ARG]...\n", progName)
	fmt.Println("Write a random permutation of the input lines to standard output.")
	fmt.Println()
	fmt.Println("  -i, --input-range=LO-HI  treat each number LO through HI as an input line")
	fmt.Println("  -n, --head-count=COUNT   output at most COUNT lines")
	fmt.Println("  -o, --output=FILE        write result to FILE instead of standard output")
	fmt.Println("  -r, --repeat             output lines can repeat")
	fmt.Println("      --random-source=FILE get random bytes from FILE")
	fmt.Println("  -z, --zero-terminated    line delimiter is NUL, not newline")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
}

// printVersion outputs version information to stdout.
// R3.4: --version prints version and exits 0.
func printVersion() {
	fmt.Printf("%s (go-unix-utils)\n", progName)
}

// exitWithError prints an error message and exits with status 1.
func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	os.Exit(1)
}
