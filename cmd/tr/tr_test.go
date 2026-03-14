// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd054-tr R1.1, R1.2, R1.3, R1.4 (differential tests)
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go tr binary against the
// GNU reference binary (gtr) for R1.1-R1.4 scenarios.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skipf("reference binary gtr not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Basic character translation.
		{
			Name:  "translate single char",
			Args:  []string{"e", "a"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "translate multiple chars",
			Args:  []string{"helo", "HELO"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "translate identity",
			Args:  []string{"abc", "abc"},
			Stdin: []byte("abc\n"),
		},

		// R1.1, R1.3: Character range translation.
		{
			Name:  "lowercase to uppercase range",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "uppercase to lowercase range",
			Args:  []string{"A-Z", "a-z"},
			Stdin: []byte("HELLO WORLD\n"),
		},
		{
			Name:  "digit range translation",
			Args:  []string{"0-9", "a-j"},
			Stdin: []byte("0123456789\n"),
		},
		{
			Name:  "partial range",
			Args:  []string{"a-f", "A-F"},
			Stdin: []byte("abcdefgh\n"),
		},

		// R1.1: SET2 shorter than SET1 -- extend with last char.
		{
			Name:  "set2 shorter extends with last char",
			Args:  []string{"abcdef", "xy"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "set2 single char maps all",
			Args:  []string{"abc", "x"},
			Stdin: []byte("aabbcc\n"),
		},
		{
			Name:  "set2 shorter with ranges",
			Args:  []string{"a-z", "X"},
			Stdin: []byte("hello\n"),
		},

		// R1.3: Escape sequences.
		{
			Name:  "escape newline in set1",
			Args:  []string{`\n`, "X"},
			Stdin: []byte("line1\nline2\n"),
		},
		{
			Name:  "escape tab in set1",
			Args:  []string{`\t`, " "},
			Stdin: []byte("col1\tcol2\n"),
		},
		{
			Name:  "escape backslash",
			Args:  []string{`\\`, "X"},
			Stdin: []byte("a\\b\\c\n"),
		},
		{
			Name:  "octal escape",
			Args:  []string{`\141`, "X"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "octal escape 012 is newline",
			Args:  []string{`\012`, "X"},
			Stdin: []byte("a\nb\n"),
		},

		// R1.4: POSIX character classes.
		{
			Name:  "class lower to upper",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "class upper to lower",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("HELLO WORLD\n"),
		},
		{
			Name:  "class digit translation",
			Args:  []string{"[:digit:]", "XXXXXXXXXX"},
			Stdin: []byte("abc123def\n"),
		},

		// R1.2: Empty input.
		{
			Name:  "empty input",
			Args:  []string{"a", "b"},
			Stdin: []byte(""),
		},

		// R1.3: Mixed individual chars and ranges.
		{
			Name:  "mixed chars and range",
			Args:  []string{"aeiou", "AEIOU"},
			Stdin: []byte("hello world\n"),
		},

		// Binary data passthrough.
		{
			Name:  "non-matching chars pass through",
			Args:  []string{"x", "y"},
			Stdin: []byte("hello\n"),
		},

		// Multiple lines.
		{
			Name:  "multiple lines",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("line one\nline two\nline three\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
