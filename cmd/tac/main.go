// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd021-tac R1.1–R1.4: core reversal behavior.
// Implements prd021-tac R2.1–R2.4: separator options (-s, -b, -r).
// Implements prd021-tac R3.1–R3.4: exit codes and SIGPIPE.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tac"

// config holds parsed command-line options for tac.
type config struct {
	separator string
	before    bool
	regex     bool
	files     []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes files, returning the exit code.
// R1.3: reads stdin when no args or "-" is given.
// R1.4: each file is reversed independently.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg := parseConfig(args)
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	re, err := compileRegex(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s: %s\n", progName, cfg.separator, err)
		return 1
	}
	exitCode := 0
	for _, name := range cfg.files {
		if err := processFile(name, cfg, re, stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 1
		}
	}
	return exitCode
}

// compileRegex compiles the separator as a regex if -r is set. R2.3.
// Prepends (?U) to match GNU tac's backward-scanning behavior, which
// produces shortest (non-greedy) matches at each position.
func compileRegex(cfg config) (*regexp.Regexp, error) {
	if !cfg.regex {
		return nil, nil
	}
	return regexp.Compile("(?U)" + cfg.separator)
}

// parseConfig parses command-line arguments into a config struct.
// R2.1: -s SEP sets custom separator. R2.2: -b for before mode.
// R2.3: -r for regex mode.
func parseConfig(args []string) config {
	cfg := config{separator: "\n"}
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			i = parseLongOption(arg, args, i, &cfg)
			continue
		}
		i = parseShortOptions(arg[1:], args, i, &cfg)
	}
	return cfg
}

// parseLongOption handles --separator=SEP, --before, --regex.
func parseLongOption(arg string, args []string, idx int, cfg *config) int {
	switch {
	case arg == "--before":
		cfg.before = true
	case arg == "--regex":
		cfg.regex = true
	case arg == "--separator" && idx+1 < len(args):
		idx++
		cfg.separator = args[idx]
	case strings.HasPrefix(arg, "--separator="):
		cfg.separator = arg[len("--separator="):]
	default:
		cfg.files = append(cfg.files, arg)
	}
	return idx
}

// parseShortOptions processes short flags like -b, -r, -s SEP,
// or combined -brs SEP. R2.1–R2.4.
func parseShortOptions(flags string, args []string, idx int, cfg *config) int {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'b':
			cfg.before = true
		case 'r':
			cfg.regex = true
		case 's':
			if j+1 < len(flags) {
				cfg.separator = flags[j+1:]
			} else if idx+1 < len(args) {
				idx++
				cfg.separator = args[idx]
			}
			return idx
		default:
			cfg.files = append(cfg.files, "-"+flags)
			return idx
		}
	}
	return idx
}

// processFile reads a single file (or stdin for "-") and writes its
// records reversed. R1.3: "-" means stdin.
func processFile(name string, cfg config, re *regexp.Regexp, stdin io.Reader, stdout io.Writer) error {
	data, err := readInput(name, stdin)
	if err != nil {
		return err
	}
	return writeReversed(string(data), cfg, re, stdout)
}

// readInput reads the entire contents of a file or stdin.
func readInput(name string, stdin io.Reader) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // best-effort close on read-only file
	return io.ReadAll(f)
}

// writeReversed splits data into records and writes them in reverse order.
// R1.1, R1.2, R2.1, R2.2, R2.3, R2.4.
func writeReversed(data string, cfg config, re *regexp.Regexp, stdout io.Writer) error {
	if len(data) == 0 {
		return nil
	}
	positions := findSepPositions(data, cfg, re)
	records := buildRecords(data, positions, cfg.before)
	for i := len(records) - 1; i >= 0; i-- {
		if _, err := io.WriteString(stdout, records[i]); err != nil {
			return err
		}
	}
	return nil
}

// findSepPositions returns [start, end) index pairs for each separator match.
func findSepPositions(data string, cfg config, re *regexp.Regexp) [][2]int {
	if cfg.regex {
		return findRegexPositions(data, re)
	}
	return findStringPositions(data, cfg.separator)
}

// findStringPositions finds all occurrences of a literal separator. R2.1.
func findStringPositions(data, sep string) [][2]int {
	var positions [][2]int
	offset := 0
	for {
		idx := strings.Index(data[offset:], sep)
		if idx < 0 {
			break
		}
		start := offset + idx
		positions = append(positions, [2]int{start, start + len(sep)})
		offset = start + len(sep)
	}
	return positions
}

// findRegexPositions finds all matches of a regex separator. R2.3, R2.4.
func findRegexPositions(data string, re *regexp.Regexp) [][2]int {
	matches := re.FindAllStringIndex(data, -1)
	positions := make([][2]int, len(matches))
	for i, m := range matches {
		positions[i] = [2]int{m[0], m[1]}
	}
	return positions
}

// buildRecords splits data into records at separator positions.
// R2.2: if before is true, separator is attached to the following record.
func buildRecords(data string, positions [][2]int, before bool) []string {
	if before {
		return buildRecordsBefore(data, positions)
	}
	return buildRecordsAfter(data, positions)
}

// buildRecordsAfter builds records with separator at end. R1.2: trailing
// separator terminates the last record rather than creating an empty record.
func buildRecordsAfter(data string, positions [][2]int) []string {
	if len(positions) == 0 {
		return []string{data}
	}
	var records []string
	start := 0
	for _, pos := range positions {
		records = append(records, data[start:pos[1]])
		start = pos[1]
	}
	if start < len(data) {
		records = append(records, data[start:])
	}
	return records
}

// buildRecordsBefore builds records with separator at beginning. R2.2.
// Text before the first separator becomes a standalone record.
func buildRecordsBefore(data string, positions [][2]int) []string {
	if len(positions) == 0 {
		return []string{data}
	}
	var records []string
	if positions[0][0] > 0 {
		records = append(records, data[:positions[0][0]])
	}
	for i, pos := range positions {
		end := len(data)
		if i+1 < len(positions) {
			end = positions[i+1][0]
		}
		records = append(records, data[pos[0]:end])
	}
	return records
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
