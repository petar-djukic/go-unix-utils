// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU tac: concatenate and print files in reverse.
// Implements prd021-tac R1-R4.
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
	sys.InstallSIGPIPEHandler()

	var separator string
	var before bool
	var useRegex bool

	flag.StringVar(&separator, "s", "\n", "use STRING as the separator")
	flag.StringVar(&separator, "separator", "\n", "use STRING as the separator")
	flag.BoolVar(&before, "b", false, "attach separator before instead of after")
	flag.BoolVar(&before, "before", false, "attach separator before instead of after")
	flag.BoolVar(&useRegex, "r", false, "interpret separator as a regex")
	flag.BoolVar(&useRegex, "regex", false, "interpret separator as a regex")
	flag.Parse()

	// R2.3: Compile regex once if -r is set.
	var re *regexp.Regexp
	if useRegex {
		var err error
		re, err = regexp.Compile(separator)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tac: %s: invalid regular expression\n", separator)
			os.Exit(1)
		}
	}

	files := flag.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, f := range files {
		data, err := readInput(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tac: %v\n", err)
			exitCode = 1
			continue
		}

		var output []byte
		if useRegex {
			output = reverseRegex(data, re, before)
		} else {
			output = reverseString(data, []byte(separator), before)
		}

		if _, err := os.Stdout.Write(output); err != nil {
			os.Exit(1)
		}
	}

	os.Exit(exitCode)
}

// readInput reads from stdin if name is "-", otherwise from the named file.
// R1.3: stdin when no files or "-" is specified.
func readInput(name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(name)
}

// reverseString reverses records split by a fixed string separator.
// R1.1, R1.2, R2.1, R2.2.
func reverseString(input, sep []byte, before bool) []byte {
	if len(input) == 0 {
		return nil
	}

	if before {
		// R2.2: Separator before each record.
		hasLeading := bytes.HasPrefix(input, sep)
		if hasLeading {
			input = input[len(sep):]
		}
		parts := bytes.Split(input, sep)
		reverseSlice(parts)
		result := bytes.Join(parts, sep)
		if hasLeading {
			result = append(append(make([]byte, 0, len(sep)+len(result)), sep...), result...)
		}
		return result
	}

	// Default: separator after each record.
	// R1.2: Trailing separator terminates last record, not an empty record.
	hasTrailing := bytes.HasSuffix(input, sep)
	if hasTrailing {
		input = input[:len(input)-len(sep)]
	}
	parts := bytes.Split(input, sep)
	reverseSlice(parts)
	result := bytes.Join(parts, sep)
	if hasTrailing {
		result = append(result, sep...)
	}
	return result
}

// reverseRegex reverses records split by a regex separator.
// R2.3, R2.4.
func reverseRegex(input []byte, re *regexp.Regexp, before bool) []byte {
	if len(input) == 0 {
		return nil
	}

	locs := re.FindAllIndex(input, -1)
	if len(locs) == 0 {
		return append([]byte(nil), input...)
	}

	// Extract alternating text and separator parts.
	// texts[0], seps[0], texts[1], seps[1], ..., texts[n]
	texts := make([][]byte, 0, len(locs)+1)
	seps := make([][]byte, 0, len(locs))
	prev := 0
	for _, loc := range locs {
		texts = append(texts, input[prev:loc[0]])
		seps = append(seps, input[loc[0]:loc[1]])
		prev = loc[1]
	}
	texts = append(texts, input[prev:])

	if before {
		return reassembleBefore(texts, seps)
	}
	return reassembleAfter(texts, seps)
}

// reassembleAfter reverses records where the separator follows each record.
func reassembleAfter(texts, seps [][]byte) []byte {
	lastText := texts[len(texts)-1]
	hasTrailing := len(lastText) == 0

	if hasTrailing {
		// Build records: texts[i] + seps[i] for each separator.
		records := make([][]byte, len(seps))
		for i := range seps {
			records[i] = append(append([]byte(nil), texts[i]...), seps[i]...)
		}
		reverseSlice(records)
		return bytes.Join(records, nil)
	}

	// No trailing separator: last record has no separator.
	records := make([][]byte, len(seps)+1)
	for i := range seps {
		records[i] = append(append([]byte(nil), texts[i]...), seps[i]...)
	}
	records[len(seps)] = append([]byte(nil), lastText...)
	reverseSlice(records)
	return bytes.Join(records, nil)
}

// reassembleBefore reverses records where the separator precedes each record.
func reassembleBefore(texts, seps [][]byte) []byte {
	firstText := texts[0]
	hasLeading := len(firstText) == 0

	if hasLeading {
		// Build records: seps[i] + texts[i+1] for each separator.
		records := make([][]byte, len(seps))
		for i := range seps {
			records[i] = append(append([]byte(nil), seps[i]...), texts[i+1]...)
		}
		reverseSlice(records)
		return bytes.Join(records, nil)
	}

	// No leading separator: first record has no separator.
	records := make([][]byte, len(seps)+1)
	records[0] = append([]byte(nil), firstText...)
	for i := range seps {
		records[i+1] = append(append([]byte(nil), seps[i]...), texts[i+1]...)
	}
	reverseSlice(records)
	return bytes.Join(records, nil)
}

// reverseSlice reverses a slice of byte slices in place.
func reverseSlice(s [][]byte) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
