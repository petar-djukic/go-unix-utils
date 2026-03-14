// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd021-tac R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.3
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// tacOpts holds the configuration for tac's record splitting.
type tacOpts struct {
	separator string
	before    bool
	regex     bool
}

func main() {
	// R3.4: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	var opts tacOpts
	// R2.1: -s flag for custom separator.
	flag.StringVar(&opts.separator, "s", "\n", "")
	// R2.2: -b flag for separator-before-record.
	flag.BoolVar(&opts.before, "b", false, "")
	// R2.3, R2.4: -r flag to interpret separator as regex.
	flag.BoolVar(&opts.regex, "r", false, "")
	flag.Parse()

	args := flag.Args()
	exitCode := 0

	if len(args) == 0 {
		// R1.3: no file arguments — read from stdin.
		if err := tacReader(os.Stdin, opts); err != nil {
			fmt.Fprintf(os.Stderr, "tac: %v\n", err)
			os.Exit(1)
		}
	} else {
		// R1.4: process each file independently in argument order.
		for _, arg := range args {
			if err := tacFile(arg, opts); err != nil {
				fmt.Fprintf(os.Stderr, "tac: %v\n", err)
				exitCode = 1
			}
		}
	}

	// R3.1, R3.2: exit 0 on success, 1 if any file failed.
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// tacFile opens name and reverses its records to stdout.
// R1.3: "-" reads from stdin.
func tacFile(name string, opts tacOpts) error {
	if name == "-" {
		return tacReader(os.Stdin, opts)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close() // best-effort cleanup, error ignored
	return tacReader(f, opts)
}

// tacReader reads all content from r, splits into records, and writes records
// in reverse order to stdout.
//
// R1.1: split on separator, write records in reverse order.
// R1.2: trailing separator terminates the last record.
// R2.1–R2.4: custom separator, before-record, and regex modes.
func tacReader(r io.Reader, opts tacOpts) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	records, err := splitRecords(data, opts)
	if err != nil {
		return err
	}

	// Write records in reverse order.
	for i := len(records) - 1; i >= 0; i-- {
		if _, err := os.Stdout.Write(records[i]); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	return nil
}

// splitRecords splits data into records based on the separator options.
func splitRecords(data []byte, opts tacOpts) ([][]byte, error) {
	if opts.regex {
		// R2.3, R2.4: compile separator as regex.
		re, err := regexp.Compile(opts.separator)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", opts.separator, err)
		}
		if opts.before {
			return splitRegexBefore(data, re), nil
		}
		return splitRegexAfter(data, re), nil
	}

	sep := []byte(opts.separator)
	if opts.before {
		return splitLiteralBefore(data, sep), nil
	}
	return splitLiteralAfter(data, sep), nil
}

// splitLiteralAfter splits data on literal sep, keeping the separator at the
// end of each record. The final record may lack a separator if the input
// does not end with one.
//
// R1.1, R2.1: split on literal separator string.
func splitLiteralAfter(data, sep []byte) [][]byte {
	var records [][]byte
	for len(data) > 0 {
		idx := bytes.Index(data, sep)
		if idx < 0 {
			records = append(records, data)
			break
		}
		records = append(records, data[:idx+len(sep)])
		data = data[idx+len(sep):]
	}
	return records
}

// splitLiteralBefore splits data on literal sep, keeping the separator at the
// beginning of each record. The first record may lack a separator if the input
// does not start with one.
//
// R2.2: separator before each record.
func splitLiteralBefore(data, sep []byte) [][]byte {
	positions := findAllLiteral(data, sep)
	if len(positions) == 0 {
		if len(data) > 0 {
			return [][]byte{data}
		}
		return nil
	}

	var records [][]byte
	// Text before first separator is a record without separator.
	if positions[0] > 0 {
		records = append(records, data[:positions[0]])
	}
	// Each separator + text until next separator.
	for i, pos := range positions {
		end := len(data)
		if i+1 < len(positions) {
			end = positions[i+1]
		}
		records = append(records, data[pos:end])
	}
	return records
}

// splitRegexAfter splits data on regex matches, keeping the matched separator
// at the end of each record.
//
// R2.3, R2.4: regex separator after record.
func splitRegexAfter(data []byte, re *regexp.Regexp) [][]byte {
	locs := re.FindAllIndex(data, -1)
	if len(locs) == 0 {
		if len(data) > 0 {
			return [][]byte{data}
		}
		return nil
	}

	var records [][]byte
	start := 0
	for _, loc := range locs {
		records = append(records, data[start:loc[1]])
		start = loc[1]
	}
	if start < len(data) {
		records = append(records, data[start:])
	}
	return records
}

// splitRegexBefore splits data on regex matches, keeping the matched separator
// at the beginning of each record.
//
// R2.2, R2.3: regex separator before record.
func splitRegexBefore(data []byte, re *regexp.Regexp) [][]byte {
	locs := re.FindAllIndex(data, -1)
	if len(locs) == 0 {
		if len(data) > 0 {
			return [][]byte{data}
		}
		return nil
	}

	var records [][]byte
	if locs[0][0] > 0 {
		records = append(records, data[:locs[0][0]])
	}
	for i, loc := range locs {
		end := len(data)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		records = append(records, data[loc[0]:end])
	}
	return records
}

// findAllLiteral returns the start positions of all non-overlapping occurrences
// of sep in data.
func findAllLiteral(data, sep []byte) []int {
	var positions []int
	start := 0
	for {
		idx := bytes.Index(data[start:], sep)
		if idx < 0 {
			break
		}
		positions = append(positions, start+idx)
		start = start + idx + len(sep)
	}
	return positions
}
