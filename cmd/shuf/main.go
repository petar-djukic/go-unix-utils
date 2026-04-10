// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/shuf: shuffle lines randomly.
// Implements srd064-shuf R1.1, R1.2, R1.3, R1.4, R2.1, R3.3.
package main

import (
	"bufio"
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

// config holds parsed command-line flags for shuf.
type config struct {
	inputRange string
	echo       bool
	args       []string
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
		return 1
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
	// R1.3: uniform random permutation.
	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
	if err := writeLines(lines); err != nil {
		return 1
	}
	return 0
}

// parseFlags extracts -i, -e, and positional arguments.
func parseFlags(args []string) (config, error) {
	var cfg config
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			return cfg, nil
		}
		if arg == "-e" || arg == "--echo" {
			cfg.echo = true
			continue
		}
		if handled, adv, err := parseInputRange(args, i, &cfg); handled {
			if err != nil {
				return cfg, err
			}
			i += adv
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return cfg, fmt.Errorf("invalid option -- '%s'", strings.TrimLeft(arg, "-"))
		}
		cfg.args = append(cfg.args, arg)
	}
	return cfg, nil
}

// parseInputRange handles -i and --input-range flag variants.
// Returns (handled, advance, error).
func parseInputRange(args []string, i int, cfg *config) (bool, int, error) {
	arg := args[i]
	switch {
	case arg == "-i":
		if i+1 >= len(args) {
			return true, 0, fmt.Errorf("option requires an argument -- 'i'")
		}
		cfg.inputRange = args[i+1]
		return true, 1, nil
	case strings.HasPrefix(arg, "--input-range="):
		cfg.inputRange = arg[len("--input-range="):]
		return true, 0, nil
	case arg == "--input-range":
		if i+1 >= len(args) {
			return true, 0, fmt.Errorf("option '--input-range' requires an argument")
		}
		cfg.inputRange = args[i+1]
		return true, 1, nil
	}
	return false, 0, nil
}

// validateExclusivity checks R2.3: -i and -e are mutually exclusive;
// -i must not be combined with file arguments.
func validateExclusivity(cfg config) error {
	if cfg.inputRange != "" && cfg.echo {
		return fmt.Errorf("cannot combine -e and -i options")
	}
	if cfg.inputRange != "" && len(cfg.args) > 0 {
		return fmt.Errorf("extra operand '%s'", cfg.args[0])
	}
	return nil
}

// collectLines gathers input lines based on the active mode.
func collectLines(cfg config) ([]string, error) {
	if cfg.inputRange != "" {
		return generateRange(cfg.inputRange)
	}
	if cfg.echo {
		return cfg.args, nil
	}
	return readAllLines(cfg.args)
}

// generateRange produces integers from LO to HI inclusive.
// R2.1: -i LO-HI generates the integer sequence.
// R2.4: validates LO and HI are non-negative with LO <= HI.
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
func readAllLines(files []string) ([]string, error) {
	if len(files) == 0 {
		files = []string{"-"}
	}
	var lines []string
	for _, name := range files {
		fileLines, err := readLinesFromFile(name)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

// readLinesFromFile reads lines from a single file or stdin.
func readLinesFromFile(name string) ([]string, error) {
	r, closer, err := openInput(name)
	if err != nil {
		return nil, err
	}
	defer closer()
	return scanLines(r)
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

// scanLines reads all lines from a reader using bufio.Scanner.
// R1.4: includes the last line even without a trailing newline.
func scanLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// writeLines writes each line to stdout followed by a newline.
func writeLines(lines []string) error {
	w := bufio.NewWriter(os.Stdout)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return w.Flush()
}
