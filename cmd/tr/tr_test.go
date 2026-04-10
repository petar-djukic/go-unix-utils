// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/tr: differential tests against gtr.
// Implements srd054-tr R4.3.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skip("reference binary gtr not in PATH")
	}
	tests := []testutils.DiffTest{
		// R2.1: -d delete mode
		{
			Name:  "delete single char",
			Args:  []string{"-d", "l"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "delete digit class",
			Args:  []string{"-d", "[:digit:]"},
			Stdin: []byte("hello 123\n"),
		},
		{
			Name:  "delete range",
			Args:  []string{"-d", "a-c"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "delete all vowels",
			Args:  []string{"-d", "aeiou"},
			Stdin: []byte("the quick brown fox\n"),
		},
		// R2.2: -s squeeze mode
		{
			Name:  "squeeze single set",
			Args:  []string{"-s", "a-c"},
			Stdin: []byte("aabbcc\n"),
		},
		{
			Name:  "squeeze spaces",
			Args:  []string{"-s", " "},
			Stdin: []byte("hello   world   foo\n"),
		},
		{
			Name:  "squeeze with translate",
			Args:  []string{"-s", "a-z", "A-Z"},
			Stdin: []byte("aabbbccdd\n"),
		},
		{
			Name:  "squeeze newlines",
			Args:  []string{"-s", "\\n"},
			Stdin: []byte("a\n\n\nb\n\nc\n"),
		},
		// R2.3: -d -s combined
		{
			Name:  "delete and squeeze",
			Args:  []string{"-ds", "[:digit:]", " "},
			Stdin: []byte("1 2  3   abc\n"),
		},
		{
			Name:  "delete digits squeeze spaces",
			Args:  []string{"-d", "-s", "[:digit:]", "[:space:]"},
			Stdin: []byte("a1  b2  c3\n"),
		},
		// R2.4: -c complement
		{
			Name:  "complement delete",
			Args:  []string{"-cd", "a-z\\n"},
			Stdin: []byte("hello 123 world!\n"),
		},
		{
			Name:  "complement translate",
			Args:  []string{"-c", "a-z\\n", "*"},
			Stdin: []byte("hello world 123\n"),
		},
		{
			Name:  "complement squeeze",
			Args:  []string{"-cs", "a-z", "\\n"},
			Stdin: []byte("hello 123 world!\n"),
		},
		{
			Name:  "complement delete squeeze",
			Args:  []string{"-cds", "[:alpha:]", "\\n"},
			Stdin: []byte("abc 123 def\n"),
		},
		// Additional edge cases
		{
			Name:  "delete empty input",
			Args:  []string{"-d", "x"},
			Stdin: []byte(""),
		},
		{
			Name:  "squeeze no repeats",
			Args:  []string{"-s", "abc"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "squeeze all same",
			Args:  []string{"-s", "a"},
			Stdin: []byte("aaaa\n"),
		},
		{
			Name:  "complement with upper class",
			Args:  []string{"-cd", "[:upper:]\\n"},
			Stdin: []byte("Hello World 123\n"),
		},
		// R3.1: character class translation pairs
		{
			Name:  "lower to upper",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "upper to lower",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("HELLO WORLD\n"),
		},
		{
			Name:  "lower to upper mixed",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("Hello World 123!\n"),
		},
		{
			Name:  "upper to lower mixed",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("Hello World 123!\n"),
		},
		{
			Name:  "lower to upper empty input",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte(""),
		},
		{
			Name:  "lower to upper no letters",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("123 !@#\n"),
		},
		// R3.2: missing SET2 error
		{
			Name:      "missing set2 error",
			Args:      []string{"abc"},
			Stdin:     []byte(""),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "missing operand no args",
			Args:      []string{},
			Stdin:     []byte(""),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		// R3.3: equivalence classes
		{
			Name:      "equiv class rejected in set2 translate",
			Args:      []string{"a", "[=b=]"},
			Stdin:     []byte("aaa\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:  "equiv class in set1 delete",
			Args:  []string{"-d", "[=a=]"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "equiv class in set1 squeeze",
			Args:  []string{"-s", "[=a=]"},
			Stdin: []byte("aaabbb\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normStderr normalizes stderr for error case comparison.
// Strips "Try '...'" help hint lines and normalizes binary name
// prefix (gtr: → tr:) so Go and reference outputs can match.
func normStderr(data []byte) []byte {
	var result []byte
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("Try '")) {
			continue
		}
		line = normProgName(line)
		if len(result) > 0 {
			result = append(result, '\n')
		}
		result = append(result, line...)
	}
	return result
}

// normProgName replaces "gtr: " prefix with "tr: " for comparison.
func normProgName(line []byte) []byte {
	if bytes.HasPrefix(line, []byte("gtr: ")) {
		return append([]byte("tr: "), line[5:]...)
	}
	return line
}
