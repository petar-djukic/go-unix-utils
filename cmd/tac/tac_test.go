// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "abc.txt", "a\nb\nc\n")
	writeFixture(t, dir, "xyz.txt", "x\ny\nz\n")
	writeFixture(t, dir, "notail.txt", "a\nb\nc")

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?tac`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("tac"))
	})
	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.1: basic line reversal via stdin
		{Name: "basic-stdin", Stdin: []byte("a\nb\nc\n"), Env: []string{"LC_ALL=C"}},
		// R1.1: single line
		{Name: "single-line", Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		// R1.1: empty input
		{Name: "empty-input", Stdin: []byte{}, Env: []string{"LC_ALL=C"}},
		// R1.2: trailing newline preserved
		{Name: "trailing-newline", Stdin: []byte("x\ny\nz\n"), Env: []string{"LC_ALL=C"}},
		// R1.2: no trailing newline
		{Name: "no-trailing-newline", Stdin: []byte("a\nb\nc"), Env: []string{"LC_ALL=C"}},
		// R1.3: stdin via no args
		{Name: "stdin-no-args", Stdin: []byte("1\n2\n3\n"), Env: []string{"LC_ALL=C"}},
		// R1.3: stdin via "-"
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("p\nq\nr\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: single file argument
		{
			Name:    "single-file",
			Args:    []string{"abc.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: multiple files processed independently
		{
			Name:    "multi-file",
			Args:    []string{"abc.txt", "xyz.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: file with no trailing newline
		{
			Name:    "file-no-trailing-nl",
			Args:    []string{"notail.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R2.1: custom separator -s
		{
			Name:  "custom-sep-colon",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: custom separator multi-char
		{
			Name:  "custom-sep-multichar",
			Args:  []string{"-s", "::"},
			Stdin: []byte("a::b::c::"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: custom separator with no trailing
		{
			Name:  "custom-sep-no-trailing",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -b places separator before record
		{
			Name:  "before-flag",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -b with trailing content
		{
			Name:  "before-flag-trailing",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c:"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -b with newline separator
		{
			Name:  "before-newline",
			Args:  []string{"-b"},
			Stdin: []byte("\na\nb\nc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: nonexistent file produces error, continues
		{
			Name:      "nonexistent-file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		// R3.2: nonexistent with valid file, processing continues
		{
			Name:      "nonexistent-with-valid",
			Args:      []string{"nonexistent.txt", "abc.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		// R1.1: many lines
		{
			Name:  "many-lines",
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: single char no newline
		{Name: "single-char", Stdin: []byte("x"), Env: []string{"LC_ALL=C"}},
		// R1.2: only newlines
		{Name: "only-newlines", Stdin: []byte("\n\n\n"), Env: []string{"LC_ALL=C"}},
		// R2.1: separator not present in input
		{
			Name:  "sep-not-found",
			Args:  []string{"-s", "|"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -r interprets separator as regex
		{
			Name:  "regex-digit-sep",
			Args:  []string{"-r", "-s", "[0-9]+"},
			Stdin: []byte("a1b22c"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -r with dot regex
		{
			Name:  "regex-dot-sep",
			Args:  []string{"-r", "-s", ":+"},
			Stdin: []byte("a:b::c:::d:"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -r and -s combined with trailing match
		{
			Name:  "regex-combined-trailing",
			Args:  []string{"-r", "-s", "[;,]+"},
			Stdin: []byte("a;b,,c;"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -r and -s combined with -b (before)
		{
			Name:  "regex-before",
			Args:  []string{"-r", "-b", "-s", "[0-9]+"},
			Stdin: []byte("1a2b3c"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -r with no trailing match
		{
			Name:  "regex-no-trailing",
			Args:  []string{"-r", "-s", "[0-9]+"},
			Stdin: []byte("a1b2c"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
