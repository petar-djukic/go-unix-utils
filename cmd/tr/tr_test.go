// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies prd054-tr R1.1-R1.4, R2.1-R2.4 via differential testing
// against gtr.
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

		// R2.1: -d delete single character
		{
			Name:  "delete-single-char",
			Args:  []string{"-d", "l"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -d delete with character range
		{
			Name:  "delete-range",
			Args:  []string{"-d", "a-c"},
			Stdin: []byte("abcdefabc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -d delete digits via POSIX class
		{
			Name:  "delete-digit-class",
			Args:  []string{"-d", "[:digit:]"},
			Stdin: []byte("hello 123\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --delete long flag
		{
			Name:  "delete-long-flag",
			Args:  []string{"--delete", "x"},
			Stdin: []byte("foxbox\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -d delete all lowercase
		{
			Name:  "delete-lower-class",
			Args:  []string{"-d", "[:lower:]"},
			Stdin: []byte("Hello World 123\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.2: -s squeeze single set
		{
			Name:  "squeeze-single-set",
			Args:  []string{"-s", "a-c"},
			Stdin: []byte("aabbcc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -s squeeze spaces
		{
			Name:  "squeeze-spaces",
			Args:  []string{"-s", " "},
			Stdin: []byte("hello   world   foo\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -s squeeze with translation (two SETs)
		{
			Name:  "squeeze-with-translate",
			Args:  []string{"-s", "a-z", "A-Z"},
			Stdin: []byte("aaabbbccc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: --squeeze-repeats long flag
		{
			Name:  "squeeze-long-flag",
			Args:  []string{"--squeeze-repeats", "a"},
			Stdin: []byte("aaabba\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -s squeeze newlines
		{
			Name:  "squeeze-newlines",
			Args:  []string{"-s", `\n`},
			Stdin: []byte("hello\n\n\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.3: -d -s combined (delete digits, squeeze spaces)
		{
			Name:  "delete-squeeze-combined",
			Args:  []string{"-ds", "[:digit:]", " "},
			Stdin: []byte("hello 123  world  456\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -d -s combined with separate flags
		{
			Name:  "delete-squeeze-separate-flags",
			Args:  []string{"-d", "-s", "[:digit:]", " "},
			Stdin: []byte("abc 1 2 3 def\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.4: -c complement with translate
		{
			Name:  "complement-translate",
			Args:  []string{"-c", "a-z\\n", "*"},
			Stdin: []byte("hello world 123\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -c complement with delete
		{
			Name:  "complement-delete",
			Args:  []string{"-cd", "a-z\\n"},
			Stdin: []byte("hello 123 world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -C complement (uppercase C)
		{
			Name:  "complement-uppercase-C",
			Args:  []string{"-Cd", "[:alpha:]\\n"},
			Stdin: []byte("abc 123 DEF\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: --complement long flag
		{
			Name:  "complement-long-flag",
			Args:  []string{"--complement", "-d", "[:alpha:]\\n"},
			Stdin: []byte("hello 123 world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -c complement with squeeze
		{
			Name:  "complement-squeeze",
			Args:  []string{"-cs", "a-z", "\\n"},
			Stdin: []byte("hello 123 world\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
