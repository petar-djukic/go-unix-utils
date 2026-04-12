// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/shuf: shuffle lines randomly.
// Implements srd064-shuf R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "shuf"

// helpText is printed when --help is given.
// R4.1: --help prints usage to stdout and exits 0.
const helpText = `Usage: shuf [OPTION]... [FILE]
  or:  shuf -e [OPTION]... [ARG]...
  or:  shuf -i LO-HI [OPTION]...
Write a randomly permuted copy of the input lines to standard output.

With no FILE, or when FILE is -, read standard input.

  -e, --echo                treat each ARG as an input line
  -i, --input-range=LO-HI   treat each number LO through HI as an input line
  -n, --head-count=COUNT     output at most COUNT lines
  -o, --output=FILE          write result to FILE instead of standard output
      --random-source=FILE   get random bytes from FILE
  -r, --repeat               output lines can be repeated
  -z, --zero-terminated      line delimiter is NUL, not newline
      --help        display this help and exit
      --version     output version information and exit
`

// versionText is printed when --version is given.
// R4.2: --version prints version to stdout and exits 0.
const versionText = "shuf (go-unix-utils) 1.0\n"

// config holds parsed command-line flags for shuf.
type config struct {
	inputRange     string
	echo           bool
	headCount      int // -1 means not set
	outputFile     string
	randomSource   string
	repeat         bool
	zeroTerminated bool
	showHelp       bool
	showVersion    bool
	args           []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the shuf logic and returns the exit code.
func run(args []string) int {
	cfg, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return 1
	}
	if cfg.showHelp {
		fmt.Fprint(os.Stdout, helpText)
		return 0
	}
	if cfg.showVersion {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}
	if err := validateExclusivity(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	lines, err := collectLines(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	return emitOutput(cfg, lines)
}

// emitOutput creates the RNG and output writer, then shuffles and writes.
func emitOutput(cfg config, lines []string) int {
	rng, err := makeRNG(cfg.randomSource)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	w, closer, err := openOutput(cfg.outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	defer closer()
	if err := shuffleAndWrite(w, lines, rng, cfg); err != nil {
		return 1
	}
	return 0
}

// parseFlags extracts all recognized flags and positional arguments.
func parseFlags(args []string) (config, error) {
	cfg := config{headCount: -1}
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			return cfg, nil
		}
		adv, err := parseFlagAt(args, i, &cfg)
		if err != nil {
			return cfg, err
		}
		if adv >= 0 {
			i += adv
			continue
		}
		if strings.HasPrefix(args[i], "-") && args[i] != "-" {
			return cfg, fmt.Errorf("invalid option -- '%s'", strings.TrimLeft(args[i], "-"))
		}
		cfg.args = append(cfg.args, args[i])
	}
	return cfg, nil
}

// parseFlagAt attempts to parse a flag at position i.
// Returns (advance, nil) on success, (-1, nil) if not a flag.
func parseFlagAt(args []string, i int, cfg *config) (int, error) {
	switch args[i] {
	case "-e", "--echo":
		cfg.echo = true
		return 0, nil
	case "-r", "--repeat":
		cfg.repeat = true
		return 0, nil
	case "-z", "--zero-terminated":
		// R3.2: use NUL as line delimiter for input and output.
		cfg.zeroTerminated = true
		return 0, nil
	case "--help":
		cfg.showHelp = true
		return 0, nil
	case "--version":
		cfg.showVersion = true
		return 0, nil
	}
	return parseValueFlagAt(args, i, cfg)
}

// parseValueFlagAt handles flags that require a value argument.
func parseValueFlagAt(args []string, i int, cfg *config) (int, error) {
	if val, adv, ok, err := optValue(args, i, "-i", "--input-range"); ok {
		if err != nil {
			return 0, err
		}
		cfg.inputRange = val
		return adv, nil
	}
	if val, adv, ok, err := optValue(args, i, "-n", "--head-count"); ok {
		if err != nil {
			return 0, err
		}
		return adv, parseHeadCount(val, cfg)
	}
	if val, adv, ok, err := optValue(args, i, "-o", "--output"); ok {
		if err != nil {
			return 0, err
		}
		cfg.outputFile = val
		return adv, nil
	}
	if val, adv, ok, err := optValue(args, i, "", "--random-source"); ok {
		if err != nil {
			return 0, err
		}
		cfg.randomSource = val
		return adv, nil
	}
	return -1, nil
}

// optValue extracts the value for a short (-x) or long (--xxx) option.
// Handles forms: -x VAL, -xVAL, --xxx=VAL, --xxx VAL.
// Returns (value, advance, matched, error).
func optValue(args []string, i int, short, long string) (string, int, bool, error) {
	arg := args[i]
	if short != "" && arg == short {
		if i+1 >= len(args) {
			ch := short[len(short)-1]
			return "", 0, true, fmt.Errorf("option requires an argument -- '%c'", ch)
		}
		return args[i+1], 1, true, nil
	}
	if short != "" && strings.HasPrefix(arg, short) && len(arg) > len(short) {
		return arg[len(short):], 0, true, nil
	}
	if long != "" && strings.HasPrefix(arg, long+"=") {
		return arg[len(long)+1:], 0, true, nil
	}
	if long != "" && arg == long {
		if i+1 >= len(args) {
			return "", 0, true, fmt.Errorf("option '%s' requires an argument", long)
		}
		return args[i+1], 1, true, nil
	}
	return "", 0, false, nil
}

// parseHeadCount validates and sets the -n value.
// R2.2: COUNT must be a non-negative integer.
func parseHeadCount(val string, cfg *config) error {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid line count: '%s'", val)
	}
	cfg.headCount = n
	return nil
}

// validateExclusivity checks that -i and -e are mutually exclusive,
// and -i is not combined with file arguments.
func validateExclusivity(cfg config) error {
	if cfg.inputRange != "" && cfg.echo {
		return fmt.Errorf("cannot combine -e and -i options")
	}
	if cfg.inputRange != "" && len(cfg.args) > 0 {
		return fmt.Errorf("extra operand '%s'", cfg.args[0])
	}
	return nil
}

// lineDelim returns the line delimiter byte based on -z flag.
// R3.2: NUL when -z is given, newline otherwise.
func lineDelim(cfg config) byte {
	if cfg.zeroTerminated {
		return 0
	}
	return '\n'
}

// collectLines gathers input lines based on the active mode.
func collectLines(cfg config) ([]string, error) {
	if cfg.inputRange != "" {
		return generateRange(cfg.inputRange)
	}
	if cfg.echo {
		return cfg.args, nil
	}
	return readAllLines(cfg.args, lineDelim(cfg))
}

// generateRange produces integers from LO to HI inclusive.
// R2.1: -i LO-HI generates the integer sequence.
func generateRange(rangeStr string) ([]string, error) {
	lo, hi, err := parseRange(rangeStr)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		lines = append(lines, strconv.Itoa(i))
	}
	return lines, nil
}

// parseRange splits a "LO-HI" string and validates both bounds.
func parseRange(s string) (int, int, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil || lo < 0 {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	hi, err := strconv.Atoi(parts[1])
	if err != nil || hi < 0 {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("invalid input range: '%s'", s)
	}
	return lo, hi, nil
}

// readAllLines reads all lines from the given files.
// R1.2: empty slice or "-" means stdin.
func readAllLines(files []string, delim byte) ([]string, error) {
	if len(files) == 0 {
		files = []string{"-"}
	}
	var lines []string
	for _, name := range files {
		fileLines, err := readLinesFromFile(name, delim)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

// readLinesFromFile reads lines from a single file or stdin.
func readLinesFromFile(name string, delim byte) ([]string, error) {
	r, closer, err := openInput(name)
	if err != nil {
		return nil, err
	}
	defer closer()
	return scanLines(r, delim)
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

// scanLines reads all lines from a reader using the given delimiter.
// R1.4: includes the last line even without a trailing delimiter.
// R3.2: uses NUL as delimiter when -z is given.
func scanLines(r io.Reader, delim byte) ([]string, error) {
	scanner := bufio.NewScanner(r)
	if delim != '\n' {
		scanner.Split(splitOnByte(delim))
	}
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// splitOnByte returns a bufio.SplitFunc that splits on the given byte.
func splitOnByte(delim byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, delim); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
}

// makeRNG creates a random number generator.
// R3.1: --random-source reads bytes from a file to seed the PRNG.
func makeRNG(path string) (*rand.Rand, error) {
	if path == "" {
		return rand.New(rand.NewSource(rand.Int63())), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return rand.New(rand.NewSource(seedFromBytes(data))), nil
}

// seedFromBytes converts up to 8 bytes into an int64 seed.
func seedFromBytes(data []byte) int64 {
	var seed int64
	for i := 0; i < len(data) && i < 8; i++ {
		seed |= int64(data[i]) << (uint(i) * 8)
	}
	return seed
}

// openOutput opens the output file, or returns stdout if path is empty.
// R2.4: -o FILE writes output to FILE instead of stdout.
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

// shuffleAndWrite performs the shuffle or repeat and writes output.
func shuffleAndWrite(w io.Writer, lines []string, rng *rand.Rand, cfg config) error {
	if len(lines) == 0 {
		return nil
	}
	delim := lineDelim(cfg)
	if cfg.repeat {
		return writeRepeat(w, lines, rng, cfg.headCount, delim)
	}
	rng.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
	count := len(lines)
	if cfg.headCount >= 0 && cfg.headCount < count {
		count = cfg.headCount
	}
	return writeLinesTo(w, lines[:count], delim)
}

// writeRepeat outputs random selections with replacement.
// R2.3: -r samples with replacement; without -n, repeats indefinitely.
func writeRepeat(w io.Writer, lines []string, rng *rand.Rand, headCount int, delim byte) error {
	bw := bufio.NewWriter(w)
	n := len(lines)
	for written := 0; headCount < 0 || written < headCount; written++ {
		bw.WriteString(lines[rng.Intn(n)])
		if err := bw.WriteByte(delim); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// writeLinesTo writes each line followed by the delimiter to the writer.
func writeLinesTo(w io.Writer, lines []string, delim byte) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		bw.WriteString(line)
		if err := bw.WriteByte(delim); err != nil {
			return err
		}
	}
	return bw.Flush()
}
