// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd064-shuf R1.1–R1.4: default shuffle behavior.
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

// sortedLines splits output into lines, sorts them, and returns the sorted slice.
// Used to compare shuffle output ignoring order.
func sortedLines(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return lines
}

// TestShufStructural tests structural properties of shuf output.
// Since shuf output is non-deterministic, we verify line counts and
// value sets rather than exact byte equality (R1.3, R4.3).
func TestShufStructural(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name      string
		stdin     string
		wantLines []string // expected lines in sorted order
	}{
		{
			name:      "three lines from stdin",
			stdin:     "a\nb\nc\n",
			wantLines: []string{"a", "b", "c"},
		},
		{
			name:      "single line",
			stdin:     "hello\n",
			wantLines: []string{"hello"},
		},
		{
			name:      "last line no trailing newline",
			stdin:     "x\ny\nz",
			wantLines: []string{"x", "y", "z"},
		},
		{
			name:      "empty input",
			stdin:     "",
			wantLines: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin)
			cmd.Stdin = bytes.NewBufferString(tc.stdin)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("shuf failed: %v", err)
			}
			got := sortedLines(out)
			if len(got) != len(tc.wantLines) {
				t.Fatalf("line count: got %d, want %d\noutput: %q",
					len(got), len(tc.wantLines), string(out))
			}
			for i := range got {
				if got[i] != tc.wantLines[i] {
					t.Fatalf("sorted line %d: got %q, want %q",
						i, got[i], tc.wantLines[i])
				}
			}
		})
	}
}

// TestShufFromFile tests reading lines from a file argument (R1.1).
func TestShufFromFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, inputFile)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf failed: %v", err)
	}
	got := sortedLines(out)
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("line count: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("sorted line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestShufMultipleFiles tests reading lines from multiple file arguments (R1.1).
func TestShufMultipleFiles(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	if err := os.WriteFile(file1, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("c\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, file1, file2)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf failed: %v", err)
	}
	got := sortedLines(out)
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("line count: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("sorted line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestShufDashStdin tests that "-" reads from stdin (R1.2).
func TestShufDashStdin(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "-")
	cmd.Stdin = bytes.NewBufferString("one\ntwo\nthree\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf - failed: %v", err)
	}
	got := sortedLines(out)
	want := []string{"one", "three", "two"}
	if len(got) != len(want) {
		t.Fatalf("line count: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("sorted line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDiff runs differential tests against the GNU reference binary (gshuf).
// These verify structural properties since output order is non-deterministic.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skip("reference binary gshuf not in PATH")
	}

	// For shuf, we cannot use RunDiffTests directly since output is random.
	// Instead, run both binaries and verify structural properties match.
	tests := []struct {
		name      string
		args      []string
		stdin     string
		wantCount int
	}{
		{
			name:      "stdin three lines",
			stdin:     "a\nb\nc\n",
			wantCount: 3,
		},
		{
			name:      "stdin single line",
			stdin:     "hello\n",
			wantCount: 1,
		},
		{
			name:      "stdin no trailing newline",
			stdin:     "x\ny",
			wantCount: 2,
		},
		{
			name:      "empty stdin",
			stdin:     "",
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			verifyStructural(t, goBin, refBin, tc.args, tc.stdin, tc.wantCount)
		})
	}
}

// verifyStructural runs both binaries and checks that both produce the
// expected line count and contain the same set of lines.
func verifyStructural(t *testing.T, goBin, refBin string, args []string, stdin string, wantCount int) {
	t.Helper()
	goOut := runShuf(t, goBin, args, stdin)
	refOut := runShuf(t, refBin, args, stdin)

	goLines := sortedLines(goOut)
	refLines := sortedLines(refOut)

	if len(goLines) != wantCount {
		t.Fatalf("go binary: got %d lines, want %d\noutput: %q",
			len(goLines), wantCount, string(goOut))
	}
	if len(refLines) != wantCount {
		t.Fatalf("ref binary: got %d lines, want %d\noutput: %q",
			len(refLines), wantCount, string(refOut))
	}

	// Both should contain the same set of lines (sorted comparison).
	for i := range goLines {
		if goLines[i] != refLines[i] {
			t.Fatalf("line set mismatch at %d: go=%q ref=%q",
				i, goLines[i], refLines[i])
		}
	}
}

// runShuf executes a shuf binary with the given args and stdin.
func runShuf(t *testing.T, binary string, args []string, stdin string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s failed: %v", binary, err)
	}
	return out
}
