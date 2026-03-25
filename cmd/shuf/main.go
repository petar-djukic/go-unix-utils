// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/shuf implements GNU shuf: shuffle lines randomly.
// Implements prd064-shuf R1.1-R1.4, R2.1, R2.2.
package main

import (
	"bytes"
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
	return cfg, nil
}

// parseFlag parses a single flag starting at args[0].
func parseFlag(cfg *config, args []string) (int, error) {
	arg := args[0]
	switch {
	case arg == "--help":
		printHelp()
		os.Exit(0)
	case arg == "--version":
		printVersion()
		os.Exit(0)
	case arg == "-z" || arg == "--zero-terminated":
		cfg.zeroTerm = true
		return 1, nil
	case strings.HasPrefix(arg, "--head-count="):
		return parseHeadCountValue(cfg, arg[len("--head-count="):])
	case strings.HasPrefix(arg, "--output="):
		cfg.outputFile = arg[len("--output="):]
		return 1, nil
	case arg == "-n" || arg == "--head-count":
		return parseHeadCountNext(cfg, args)
	case arg == "-o" || arg == "--output":
		return parseOutputNext(cfg, args)
	case strings.HasPrefix(arg, "-n"):
		return parseHeadCountValue(cfg, arg[2:])
	case strings.HasPrefix(arg, "-o"):
		cfg.outputFile = arg[2:]
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

// runShuf executes the shuffle operation.
func runShuf(cfg *config) int {
	lines, err := readLines(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		return 1
	}
	shuffleLines(lines)
	return writeOutput(cfg, lines)
}

// readLines reads all input lines from files or stdin.
// R1.2: read from stdin when no file arguments or "-".
// R1.3: read from named files.
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

// shuffleLines randomly permutes lines in place.
// R1.3: produces a permutation (each line exactly once).
func shuffleLines(lines []string) {
	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
}

// writeOutput writes shuffled lines to stdout or the output file.
// R2.1: -o writes to a named file.
// R2.2: -z uses NUL delimiter in output.
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
	delim := byte('\n')
	if cfg.zeroTerm {
		delim = 0
	}
	for i := 0; i < count; i++ {
		if _, err := fmt.Fprintf(w, "%s%c", lines[i], delim); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
			return 1
		}
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

// printHelp outputs usage information to stdout.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION]... [FILE]\n", progName)
	fmt.Printf("  or:  %s -i LO-HI [OPTION]...\n", progName)
	fmt.Printf("  or:  %s -e [OPTION]... [ARG]...\n", progName)
	fmt.Println("Write a random permutation of the input lines to standard output.")
	fmt.Println()
	fmt.Println("  -n, --head-count=COUNT  output at most COUNT lines")
	fmt.Println("  -o, --output=FILE       write result to FILE instead of standard output")
	fmt.Println("  -z, --zero-terminated   line delimiter is NUL, not newline")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
}

// printVersion outputs version information to stdout.
func printVersion() {
	fmt.Printf("%s (go-unix-utils)\n", progName)
}

// exitWithError prints an error message and exits with status 1.
func exitWithError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", progName, msg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
	os.Exit(1)
}
