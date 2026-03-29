// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tac implements GNU tac: concatenate and print files in reverse.
//
// Implements prd021-tac: R1 (core reversal), R2 (separator options),
// R3 (exit codes and SIGPIPE), R4 (differential testing).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R2.1: -s SEP uses SEP as the record separator instead of newline.
var separator = flag.String("s", "\n", "use STRING as the separator instead of newline")

// R2.2: -b places the separator before each record instead of after it.
var before = flag.Bool("b", false, "attach the separator before instead of after")

// R2.3: -r interprets the separator as a regular expression.
var regex = flag.Bool("r", false, "interpret the separator as a regular expression")

func main() {
	// R3.4: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := processFiles(files)
	os.Exit(exitCode)
}

// processFiles processes each file and returns the exit code.
// R1.4: each file is processed independently in argument order.
// R3.1: returns 0 on success. R3.2: returns 1 on any error.
func processFiles(files []string) int {
	exitCode := 0
	for _, name := range files {
		if err := processFile(name); err != nil {
			fmt.Fprintf(os.Stderr, "tac: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile reads one file (or stdin for "-") and writes its
// records in reverse order.
// R1.1: reads entire input, splits on separator, writes reversed.
// R1.3: reads stdin when filename is "-".
func processFile(name string) error {
	data, err := readInput(name)
	if err != nil {
		return err
	}
	reversed, err := reverseRecords(data)
	if err != nil {
		return err
	}
	return writeOutput(reversed)
}

// readInput opens the named file or stdin and reads all bytes.
func readInput(name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(name)
}

// reverseRecords splits data into records on the separator and
// returns them in reverse order.
// R1.1: splits on separator, reverses order.
// R1.2: trailing separator terminates the last record.
// R2.1-R2.4: supports custom separator, before mode, and regex mode.
func reverseRecords(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	s := string(data)

	parts, err := splitRecords(s)
	if err != nil {
		return nil, err
	}

	// R1.2: trailing separator terminates last record.
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	reverseSlice(parts)
	return []byte(strings.Join(parts, "")), nil
}

// splitRecords dispatches to the appropriate split strategy based on
// the -r and -b flags.
func splitRecords(s string) ([]string, error) {
	sep := *separator
	if *regex {
		return splitByRegex(s, sep, *before)
	}
	if *before {
		return splitBeforeStr(s, sep), nil
	}
	return strings.SplitAfter(s, sep), nil
}

// splitByRegex splits s using a regex separator pattern.
// R2.3, R2.4: regex separator with optional before mode.
func splitByRegex(s, pattern string, beforeMode bool) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid separator pattern %q: %w", pattern, err)
	}
	locs := re.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return []string{s}, nil
	}
	return splitAtBoundaries(s, locs, beforeMode), nil
}

// splitBeforeStr splits s with the string separator attached before
// each record instead of after it.
// R2.2: before mode for string separators.
func splitBeforeStr(s, sep string) []string {
	locs := findAllString(s, sep)
	if len(locs) == 0 {
		return []string{s}
	}
	return splitAtBoundaries(s, locs, true)
}

// findAllString returns [start, end] pairs for all non-overlapping
// occurrences of sep in s.
func findAllString(s, sep string) [][]int {
	var result [][]int
	start := 0
	for {
		idx := strings.Index(s[start:], sep)
		if idx < 0 {
			break
		}
		pos := start + idx
		result = append(result, []int{pos, pos + len(sep)})
		start = pos + len(sep)
	}
	return result
}

// splitAtBoundaries splits s at the given match boundaries.
// In before mode, each separator is attached to the following record.
// In after mode, each separator is attached to the preceding record.
func splitAtBoundaries(s string, locs [][]int, beforeMode bool) []string {
	if beforeMode {
		return splitBefore(s, locs)
	}
	return splitAfterLocs(s, locs)
}

// splitBefore splits s with separators attached before each record.
func splitBefore(s string, locs [][]int) []string {
	var parts []string
	if locs[0][0] > 0 {
		parts = append(parts, s[:locs[0][0]])
	}
	for i, loc := range locs {
		end := len(s)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		parts = append(parts, s[loc[0]:end])
	}
	return parts
}

// splitAfterLocs splits s with separators attached after each record.
func splitAfterLocs(s string, locs [][]int) []string {
	var parts []string
	prev := 0
	for _, loc := range locs {
		parts = append(parts, s[prev:loc[1]])
		prev = loc[1]
	}
	if prev < len(s) {
		parts = append(parts, s[prev:])
	}
	return parts
}

// reverseSlice reverses a string slice in place.
func reverseSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// writeOutput writes reversed records to stdout.
// R3.3: returns error on write failure.
func writeOutput(data []byte) error {
	_, err := os.Stdout.Write(data)
	return err
}
