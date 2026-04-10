// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/tr: differential tests against gtr.
// Implements srd054-tr R4.3.
package main

import (
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
