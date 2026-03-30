// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the split utility.
// Implements prd067-split R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultLines     = 1000
	defaultPrefix    = "x"
	defaultSuffixLen = 2
)

// config holds parsed command-line options for split.
type config struct {
	lines     int
	file      string
	prefix    string
	suffixLen int
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments, opens input, and splits by lines.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	r, err := openInput(cfg.file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	defer r.Close()
	if err := splitByLines(r, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "split: %v\n", err)
		return 1
	}
	return 0
}

// parseArgs parses CLI arguments into a config.
// R1.2: PREFIX positional argument. R1.3: -l/--lines flag. R1.4: FILE or stdin.
func parseArgs(args []string) (config, error) {
	cfg := config{
		lines:     defaultLines,
		prefix:    defaultPrefix,
		suffixLen: defaultSuffixLen,
	}
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		parsed, advance, err := parseOption(arg, args, i)
		if err != nil {
			return cfg, err
		}
		if parsed {
			cfg.lines = advance.lines
			i = advance.nextIndex
			continue
		}
		positional = append(positional, arg)
	}
	return cfg, applyPositional(&cfg, positional)
}

// optionResult holds the result of parsing a single option.
type optionResult struct {
	lines     int
	nextIndex int
}

// parseOption attempts to parse a single flag from args[i].
// Returns (true, result, nil) if an option was consumed, (false, _, nil) if positional.
func parseOption(arg string, args []string, i int) (bool, optionResult, error) {
	switch {
	case arg == "-l" || arg == "--lines":
		if i+1 >= len(args) {
			return false, optionResult{}, fmt.Errorf("option '%s' requires an argument", arg)
		}
		n, err := parseLinesValue(args[i+1])
		if err != nil {
			return false, optionResult{}, err
		}
		return true, optionResult{lines: n, nextIndex: i + 1}, nil
	case strings.HasPrefix(arg, "-l") && len(arg) > 2:
		n, err := parseLinesValue(arg[2:])
		if err != nil {
			return false, optionResult{}, err
		}
		return true, optionResult{lines: n, nextIndex: i}, nil
	case strings.HasPrefix(arg, "--lines="):
		n, err := parseLinesValue(strings.TrimPrefix(arg, "--lines="))
		if err != nil {
			return false, optionResult{}, err
		}
		return true, optionResult{lines: n, nextIndex: i}, nil
	case strings.HasPrefix(arg, "--"):
		return false, optionResult{}, fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && arg != "-":
		return false, optionResult{}, fmt.Errorf("invalid option -- '%c'", arg[1])
	}
	return false, optionResult{}, nil
}

// parseLinesValue parses a positive integer line count string.
func parseLinesValue(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of lines: '%s'", s)
	}
	return n, nil
}

// applyPositional maps positional arguments to config fields.
// R1.2: [FILE [PREFIX]] positional arguments.
func applyPositional(cfg *config, args []string) error {
	switch len(args) {
	case 0:
		cfg.file = "-"
	case 1:
		cfg.file = args[0]
	case 2:
		cfg.file = args[0]
		cfg.prefix = args[1]
	default:
		return fmt.Errorf("extra operand '%s'", args[2])
	}
	return nil
}

// openInput opens the input file, or returns stdin for "-".
// R1.4: read from stdin when FILE is "-" or absent.
func openInput(file string) (io.ReadCloser, error) {
	if file == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(file)
}

// splitByLines reads from r and writes chunks of cfg.lines lines each.
// R1.1: default 1000 lines. R1.3: configurable via -l.
func splitByLines(r io.Reader, cfg config) error {
	br := bufio.NewReaderSize(r, 64*1024)
	for fileIndex := 0; ; fileIndex++ {
		suffix, err := generateSuffix(fileIndex, cfg.suffixLen)
		if err != nil {
			return err
		}
		filename := cfg.prefix + suffix
		done, werr := writeLineChunk(br, filename, cfg.lines)
		if werr != nil {
			return werr
		}
		if done {
			return nil
		}
	}
}

// writeLineChunk reads up to maxLines lines and writes them to filename.
// Returns true when EOF is reached (no more data to read).
func writeLineChunk(br *bufio.Reader, filename string, maxLines int) (bool, error) {
	var buf bytes.Buffer
	eof := false
	for i := 0; i < maxLines; i++ {
		line, err := br.ReadBytes('\n')
		buf.Write(line)
		if err == io.EOF {
			eof = true
			break
		}
		if err != nil {
			return false, err
		}
	}
	if buf.Len() > 0 {
		if err := os.WriteFile(filename, buf.Bytes(), 0o666); err != nil {
			return false, err
		}
	}
	return eof, nil
}

// generateSuffix returns the alphabetic suffix for the given index and length.
// R1.1: suffix pattern aa, ab, ..., az, ba, ..., zz for length 2.
func generateSuffix(index, length int) (string, error) {
	suffix := make([]byte, length)
	idx := index
	for i := length - 1; i >= 0; i-- {
		suffix[i] = 'a' + byte(idx%26)
		idx /= 26
	}
	if idx > 0 {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	return string(suffix), nil
}
