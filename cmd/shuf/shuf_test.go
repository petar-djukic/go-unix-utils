// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// shuf_test.go implements differential and structural tests for
// prd064-shuf R1.1–R1.4.

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs structural differential tests against gshuf.
// R4.3: verifies structural properties (line count, value sets)
// rather than exact byte equality since output is non-deterministic.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skipf("reference binary gshuf not in PATH: %v", err)
	}

	t.Run("empty_input", func(t *testing.T) {
		assertStructuralMatch(t, goBin, refBin, nil, nil, 0)
	})

	t.Run("stdin_three_lines", func(t *testing.T) {
		stdin := []byte("a\nb\nc\n")
		assertStructuralMatch(t, goBin, refBin, nil, stdin, 3)
	})

	t.Run("stdin_no_trailing_newline", func(t *testing.T) {
		stdin := []byte("x\ny\nz")
		assertStructuralMatch(t, goBin, refBin, nil, stdin, 3)
	})

	t.Run("file_argument", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "input.txt")
		os.WriteFile(f, []byte("alpha\nbeta\ngamma\n"), 0o644)
		assertStructuralMatch(t, goBin, refBin, []string{f}, nil, 3)
	})

	t.Run("dash_means_stdin", func(t *testing.T) {
		stdin := []byte("one\ntwo\n")
		assertStructuralMatch(t, goBin, refBin, []string{"-"}, stdin, 2)
	})

	t.Run("single_line", func(t *testing.T) {
		stdin := []byte("only\n")
		assertStructuralMatch(t, goBin, refBin, nil, stdin, 1)
	})
}

// TestShufflePermutation verifies R1.3: each line appears exactly once.
func TestShufflePermutation(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	stdin := []byte("a\nb\nc\nd\ne\n")
	out := runBin(t, goBin, nil, stdin)
	lines := nonEmptyLines(out)

	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}

	expected := []string{"a", "b", "c", "d", "e"}
	got := make([]string, len(lines))
	copy(got, lines)
	sort.Strings(got)
	if !equalSlices(got, expected) {
		t.Errorf("expected set %v, got %v", expected, got)
	}
}

// TestMultipleFiles verifies R1.1: lines from multiple files are combined.
func TestMultipleFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	f1 := filepath.Join(dir, "f1.txt")
	f2 := filepath.Join(dir, "f2.txt")
	os.WriteFile(f1, []byte("a\nb\n"), 0o644)
	os.WriteFile(f2, []byte("c\nd\n"), 0o644)

	out := runBin(t, goBin, []string{f1, f2}, nil)
	lines := nonEmptyLines(out)

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	expected := []string{"a", "b", "c", "d"}
	got := make([]string, len(lines))
	copy(got, lines)
	sort.Strings(got)
	if !equalSlices(got, expected) {
		t.Errorf("expected set %v, got %v", expected, got)
	}
}

// TestNoTrailingNewline verifies R1.4: last line without newline is included.
func TestNoTrailingNewline(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	stdin := []byte("first\nsecond")
	out := runBin(t, goBin, nil, stdin)
	lines := nonEmptyLines(out)

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}

	expected := []string{"first", "second"}
	got := make([]string, len(lines))
	copy(got, lines)
	sort.Strings(got)
	if !equalSlices(got, expected) {
		t.Errorf("expected set %v, got %v", expected, got)
	}
}

// assertStructuralMatch runs both binaries and compares structural properties.
func assertStructuralMatch(
	t *testing.T, goBin, refBin string,
	args []string, stdin []byte, expectedLines int,
) {
	t.Helper()
	goOut := runBin(t, goBin, args, stdin)
	refOut := runBin(t, refBin, args, stdin)

	goLines := nonEmptyLines(goOut)
	refLines := nonEmptyLines(refOut)

	if len(goLines) != expectedLines {
		t.Errorf("go binary: expected %d lines, got %d",
			expectedLines, len(goLines))
	}
	if len(refLines) != expectedLines {
		t.Errorf("ref binary: expected %d lines, got %d",
			expectedLines, len(refLines))
	}

	// Verify same set of values (sorted comparison).
	goSorted := sortedCopy(goLines)
	refSorted := sortedCopy(refLines)
	if !equalSlices(goSorted, refSorted) {
		t.Errorf("value sets differ:\n  go:  %v\n  ref: %v",
			goSorted, refSorted)
	}
}

// runBin executes a binary with args and stdin, returning stdout.
func runBin(t *testing.T, bin string, args []string, stdin []byte) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("binary %s failed: %v\nstderr: %s", bin, err, stderr.String())
	}
	return stdout.String()
}

// nonEmptyLines splits output into lines, filtering empty trailing lines.
func nonEmptyLines(s string) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// sortedCopy returns a sorted copy of the slice.
func sortedCopy(s []string) []string {
	c := make([]string, len(s))
	copy(c, s)
	sort.Strings(c)
	return c
}

// equalSlices compares two string slices for equality.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
