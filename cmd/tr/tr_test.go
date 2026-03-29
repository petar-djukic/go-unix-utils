// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies prd054-tr R1.1-R1.4 via differential testing against gtr.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skipf("reference binary gtr not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.1: basic character translation
		{
			Name:  "basic-translate",
			Args:  []string{"abc", "xyz"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: full alphabet case conversion via range
		{
			Name:  "lowercase-to-uppercase-range",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: SET2 shorter than SET1, last char repeated
		{
			Name:  "set2-shorter-extends",
			Args:  []string{"abcde", "xy"},
			Stdin: []byte("abcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: single char translation
		{
			Name:  "single-char",
			Args:  []string{"x", "y"},
			Stdin: []byte("fox box\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty stdin
		{
			Name:  "empty-stdin",
			Args:  []string{"a", "b"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: no matching chars in input
		{
			Name:  "no-match",
			Args:  []string{"xyz", "abc"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: backslash escape \t
		{
			Name:  "escape-tab",
			Args:  []string{`\t`, " "},
			Stdin: []byte("hello\tworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: octal escape \141 = 'a'
		{
			Name:  "octal-escape",
			Args:  []string{`\141`, "X"},
			Stdin: []byte("abcabc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: backslash escape \\
		{
			Name:  "escape-backslash",
			Args:  []string{`\\`, "x"},
			Stdin: []byte("a\\b\\c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: range uppercase to lowercase
		{
			Name:  "uppercase-to-lowercase-range",
			Args:  []string{"A-Z", "a-z"},
			Stdin: []byte("HELLO WORLD\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: multiple ranges in both SETs
		{
			Name:  "multiple-ranges",
			Args:  []string{"a-zA-Z", "A-Za-z"},
			Stdin: []byte("Hello World\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: repetition [c*n]
		{
			Name:  "repetition-count",
			Args:  []string{"abc", "[x*3]"},
			Stdin: []byte("abcabc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: repetition [c*] fill
		{
			Name:  "repetition-fill",
			Args:  []string{"a-z", "[x*]"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: POSIX class [:lower:] to [:upper:]
		{
			Name:  "class-lower-to-upper",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("hello world 123\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: POSIX class [:upper:] to [:lower:]
		{
			Name:  "class-upper-to-lower",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("HELLO WORLD 123\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: POSIX class [:digit:] with fill
		{
			Name:  "class-digit-fill",
			Args:  []string{"[:digit:]", "[x*]"},
			Stdin: []byte("abc123def456\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: octal escape \012 = newline
		{
			Name:  "octal-newline",
			Args:  []string{`\012`, "x"},
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
