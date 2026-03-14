// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sort implements the sort (sort lines of text files) command.
// Implements: prd053-sort R1.1, R1.2, R1.3, R1.4, R1.5, R1.6
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// config holds all parsed command-line options.
type config struct {
	reverse    bool   // -r: reverse sort order
	unique     bool   // -u: output only the first of equal consecutive lines
	outputFile string // -o FILE: write output to FILE
	files      []string
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
			case arg == "--output" || strings.HasPrefix(arg, "--output="):
				val, err := parseLongOptValue(arg, "--output", args, &i)
				if err != nil {
					return nil, err
				}
				cfg.outputFile = val
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

	// R1.1: Sort lexicographically using byte values (LC_ALL=C).
	// D2: byte-level comparison, no locale-aware collation.
	sort.SliceStable(lines, func(i, j int) bool {
		cmp := bytes.Compare(lines[i], lines[j])
		if cfg.reverse {
			// R1.4: -r reverses the sort order.
			return cmp > 0
		}
		return cmp < 0
	})

	// R1.5: -u outputs only the first of consecutive equal lines after sorting.
	if cfg.unique {
		lines = dedup(lines)
	}

	// R1.6: -o FILE writes output to FILE instead of stdout.
	return writeOutput(cfg, lines)
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

// dedup removes consecutive duplicate lines from a sorted slice.
// R1.5: -u suppresses consecutive equal lines.
func dedup(lines [][]byte) [][]byte {
	if len(lines) == 0 {
		return lines
	}
	result := [][]byte{lines[0]}
	for i := 1; i < len(lines); i++ {
		if !bytes.Equal(lines[i], lines[i-1]) {
			result = append(result, lines[i])
		}
	}
	return result
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
