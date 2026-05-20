// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.1: default last 10 lines
		{
			Name:  "default-10-lines",
			Stdin: seq(1, 20),
		},
		// R1.1: fewer than 10 lines outputs all
		{
			Name:  "default-fewer-than-10",
			Stdin: seq(1, 5),
		},
		// R1.2: explicit -n count
		{
			Name:  "explicit-n-5",
			Args:  []string{"-n", "5"},
			Stdin: seq(1, 20),
		},
		// R1.2: -n with attached value
		{
			Name:  "n-attached-value",
			Args:  []string{"-n5"},
			Stdin: seq(1, 20),
		},
		// R1.2: --lines= long form
		{
			Name:  "lines-equals",
			Args:  []string{"--lines=5"},
			Stdin: seq(1, 20),
		},
		// R1.2: --lines separate argument
		{
			Name:  "lines-separate",
			Args:  []string{"--lines", "5"},
			Stdin: seq(1, 20),
		},
		// R1.3: +N starts from line N
		{
			Name:  "plus-offset-5",
			Args:  []string{"-n", "+5"},
			Stdin: seq(1, 20),
		},
		// R1.3: +1 outputs everything
		{
			Name:  "plus-offset-1",
			Args:  []string{"-n", "+1"},
			Stdin: seq(1, 10),
		},
		// R1.3: +N exceeds input
		{
			Name:  "plus-offset-exceeds",
			Args:  []string{"-n", "+100"},
			Stdin: seq(1, 10),
		},
		// R1.4: stdin with no arguments
		{
			Name:  "stdin-no-args",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R1.4: stdin via explicit -
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// Edge: no trailing newline
		{
			Name:  "no-trailing-newline",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc"),
		},
		// Edge: count exceeds input
		{
			Name:  "count-exceeds-input",
			Args:  []string{"-n", "100"},
			Stdin: []byte("one\ntwo\n"),
		},
		// Edge: -n 0 outputs nothing
		{
			Name:  "n-zero",
			Args:  []string{"-n", "0"},
			Stdin: seq(1, 10),
		},
		// Edge: empty input
		{
			Name:  "empty-input",
			Stdin: []byte{},
		},
		// Edge: single line
		{
			Name:  "single-line",
			Stdin: []byte("only\n"),
		},
		// Error: invalid number
		{
			Name:      "invalid-number",
			Args:      []string{"-n", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	dir := t.TempDir()
	writeFixture(t, dir, "input.txt", string(seq(1, 20)))

	tests := []testutils.DiffTest{
		// R1.1: default from file
		{
			Name:    "file-default",
			Args:    []string{"input.txt"},
			WorkDir: dir,
		},
		// R1.2: explicit -n from file
		{
			Name:    "file-n-5",
			Args:    []string{"-n", "5", "input.txt"},
			WorkDir: dir,
		},
		// R1.3: +N from file
		{
			Name:    "file-plus-offset",
			Args:    []string{"-n", "+5", "input.txt"},
			WorkDir: dir,
		},
		// R4.2, R4.4: nonexistent file
		{
			Name:      "nonexistent-file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
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

func seq(start, end int) []byte {
	var b []byte
	for i := start; i <= end; i++ {
		b = fmt.Appendf(b, "%d\n", i)
	}
	return b
}
