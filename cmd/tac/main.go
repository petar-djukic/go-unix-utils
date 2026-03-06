// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the tac utility (prd021-tac R1-R4).
// tac concatenates and prints files in reverse, line by line (last line first).
// It supports custom record separators (-s), separator-before-record mode (-b),
// and regex separators (-r).
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

func main() {
	// R3.4: Handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	// R2.1: -s sets the record separator.
	separator := flag.String("s", "\n", "")
	// R2.2: -b places separator before record.
	before := flag.Bool("b", false, "")
	// R2.3: -r interprets separator as regex.
	useRegex := flag.Bool("r", false, "")
	flag.Parse()

	args := flag.Args()
	// R1.3: Read stdin when no file arguments are given.
	if len(args) == 0 {
		args = []string{"-"}
	}

	exitCode := 0
	for _, arg := range args {
		data, err := readInput(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tac: %v\n", err)
			exitCode = 1
			continue
		}

		result, err := reverseRecords(data, *separator, *before, *useRegex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tac: %v\n", err)
			os.Exit(1)
		}

		// R3.3: Exit 1 on write error.
		if _, err := os.Stdout.Write(result); err != nil {
			os.Exit(1)
		}
	}

	os.Exit(exitCode)
}

// readInput reads from stdin if name is "-", otherwise reads the named file.
func readInput(name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(name)
}

// reverseRecords splits data on the separator and reverses the records.
func reverseRecords(data []byte, sep string, before, useRegex bool) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	if useRegex {
		re, err := regexp.Compile(sep)
		if err != nil {
			return nil, fmt.Errorf("invalid regular expression %q: %w", sep, err)
		}
		return reverseRegex(data, re, before), nil
	}

	return reverseFixed(data, []byte(sep), before), nil
}

// reverseFixed reverses records separated by a fixed string.
func reverseFixed(data, sep []byte, before bool) []byte {
	parts := bytes.Split(data, sep)

	if before {
		// R2.2: Separator precedes record.
		hadLeading := len(parts) > 0 && len(parts[0]) == 0
		if hadLeading {
			parts = parts[1:]
		}
		reverseSlice(parts)
		result := bytes.Join(parts, sep)
		if hadLeading {
			result = append(append([]byte(nil), sep...), result...)
		}
		return result
	}

	// Trailing mode (default): separator follows record.
	// R1.2: Trailing separator is a terminator, not before an empty record.
	hadTrailing := len(parts) > 0 && len(parts[len(parts)-1]) == 0
	if hadTrailing {
		parts = parts[:len(parts)-1]
	}
	reverseSlice(parts)
	result := bytes.Join(parts, sep)
	if hadTrailing {
		result = append(result, sep...)
	}
	return result
}

// reverseRegex reverses records separated by a regular expression.
func reverseRegex(data []byte, re *regexp.Regexp, before bool) []byte {
	matches := re.FindAllIndex(data, -1)
	if len(matches) == 0 {
		return data
	}

	var records [][]byte
	var seps [][]byte

	if before {
		// R2.2: Separator precedes record.
		hadLeading := matches[0][0] == 0
		if !hadLeading {
			records = append(records, data[:matches[0][0]])
		}
		for i, m := range matches {
			seps = append(seps, data[m[0]:m[1]])
			end := len(data)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			records = append(records, data[m[1]:end])
		}
		reverseSlice(records)
		reverseSlice(seps)
		var buf bytes.Buffer
		if hadLeading {
			for i, r := range records {
				buf.Write(seps[i])
				buf.Write(r)
			}
		} else {
			for i, r := range records {
				if i < len(seps) {
					buf.Write(seps[i])
				}
				buf.Write(r)
			}
		}
		return buf.Bytes()
	}

	// Trailing mode: separator follows record.
	pos := 0
	for _, m := range matches {
		records = append(records, data[pos:m[0]])
		seps = append(seps, data[m[0]:m[1]])
		pos = m[1]
	}
	if pos < len(data) {
		records = append(records, data[pos:])
	}
	hadTrailing := pos == len(data)

	reverseSlice(records)
	reverseSlice(seps)

	var buf bytes.Buffer
	if hadTrailing {
		for i, r := range records {
			buf.Write(r)
			buf.Write(seps[i])
		}
	} else {
		for i, r := range records {
			buf.Write(r)
			if i < len(seps) {
				buf.Write(seps[i])
			}
		}
	}
	return buf.Bytes()
}

// reverseSlice reverses a slice of byte slices in place.
func reverseSlice(s [][]byte) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
