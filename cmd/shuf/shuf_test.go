// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd064-shuf R1.1–R1.4: default shuffle behavior.
// Differential tests for prd064-shuf R2.1–R2.4: range mode, head count, repeat, output file.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

// splitLines splits output into lines (non-sorted).
func splitLines(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
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

// TestShufRange tests -i LO-HI range mode (R2.1).
func TestShufRange(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantMin   int
		wantMax   int
	}{
		{
			name:      "range 1-5",
			args:      []string{"-i", "1-5"},
			wantCount: 5,
			wantMin:   1,
			wantMax:   5,
		},
		{
			name:      "range 0-0 single value",
			args:      []string{"-i", "0-0"},
			wantCount: 1,
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "inline form -i10-20",
			args:      []string{"-i10-20"},
			wantCount: 11,
			wantMin:   10,
			wantMax:   20,
		},
		{
			name:      "equals form --input-range=3-7",
			args:      []string{"--input-range=3-7"},
			wantCount: 5,
			wantMin:   3,
			wantMax:   7,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin, tc.args...)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("shuf failed: %v", err)
			}
			lines := splitLines(out)
			if len(lines) != tc.wantCount {
				t.Fatalf("line count: got %d, want %d\noutput: %q",
					len(lines), tc.wantCount, string(out))
			}
			verifyIntRange(t, lines, tc.wantMin, tc.wantMax)
			verifySortedUnique(t, lines)
		})
	}
}

// TestShufHeadCount tests -n COUNT limiting (R2.2).
func TestShufHeadCount(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name      string
		args      []string
		stdin     string
		wantCount int
	}{
		{
			name:      "head count 2 from 5 stdin lines",
			args:      []string{"-n", "2"},
			stdin:     "a\nb\nc\nd\ne\n",
			wantCount: 2,
		},
		{
			name:      "head count 0",
			args:      []string{"-n", "0"},
			stdin:     "a\nb\nc\n",
			wantCount: 0,
		},
		{
			name:      "head count exceeds input",
			args:      []string{"-n", "100"},
			stdin:     "a\nb\n",
			wantCount: 2,
		},
		{
			name:      "head count with range",
			args:      []string{"-i", "1-10", "-n", "3"},
			wantCount: 3,
		},
		{
			name:      "inline form -n5",
			args:      []string{"-n5"},
			stdin:     "a\nb\nc\nd\ne\nf\ng\n",
			wantCount: 5,
		},
		{
			name:      "equals form --head-count=2",
			args:      []string{"--head-count=2"},
			stdin:     "a\nb\nc\nd\n",
			wantCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin, tc.args...)
			if tc.stdin != "" {
				cmd.Stdin = bytes.NewBufferString(tc.stdin)
			}
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("shuf failed: %v", err)
			}
			lines := splitLines(out)
			if len(lines) != tc.wantCount {
				t.Fatalf("line count: got %d, want %d\noutput: %q",
					len(lines), tc.wantCount, string(out))
			}
		})
	}
}

// TestShufRepeat tests -r repeat mode (R2.3).
func TestShufRepeat(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name      string
		args      []string
		stdin     string
		wantCount int
		wantMin   int
		wantMax   int
		checkInts bool
	}{
		{
			name:      "repeat with head count from stdin",
			args:      []string{"-r", "-n", "10"},
			stdin:     "a\nb\nc\n",
			wantCount: 10,
		},
		{
			name:      "repeat with range and head count",
			args:      []string{"-r", "-n", "15", "-i", "1-5"},
			wantCount: 15,
			wantMin:   1,
			wantMax:   5,
			checkInts: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin, tc.args...)
			if tc.stdin != "" {
				cmd.Stdin = bytes.NewBufferString(tc.stdin)
			}
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("shuf failed: %v", err)
			}
			lines := splitLines(out)
			if len(lines) != tc.wantCount {
				t.Fatalf("line count: got %d, want %d\noutput: %q",
					len(lines), tc.wantCount, string(out))
			}
			if tc.checkInts {
				verifyIntRange(t, lines, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// TestShufOutputFile tests -o FILE output redirection (R2.4).
func TestShufOutputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	cmd := exec.Command(goBin, "-i", "1-5", "-o", outFile)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf failed: %v", err)
	}
	if len(stdout) != 0 {
		t.Fatalf("expected no stdout, got %q", string(stdout))
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	lines := splitLines(data)
	if len(lines) != 5 {
		t.Fatalf("line count: got %d, want 5\nfile content: %q",
			len(lines), string(data))
	}
	verifyIntRange(t, lines, 1, 5)
	verifySortedUnique(t, lines)
}

// TestShufOutputFileEquals tests --output=FILE form (R2.4).
func TestShufOutputFileEquals(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	outFile := filepath.Join(dir, "result.txt")

	cmd := exec.Command(goBin, "--output="+outFile, "-i", "1-3")
	if err := cmd.Run(); err != nil {
		t.Fatalf("shuf failed: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	lines := splitLines(data)
	if len(lines) != 3 {
		t.Fatalf("line count: got %d, want 3", len(lines))
	}
	verifyIntRange(t, lines, 1, 3)
}

// TestShufRangeWithFileError tests that -i with file args is rejected (R2.1).
func TestShufRangeWithFileError(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	f := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(f, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, "-i", "1-5", f)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for -i with file args, got: %s", string(out))
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("exit code: got %d, want 1", exitErr.ExitCode())
	}
}

// TestShufInvalidRange tests error on invalid range string (R4.2).
func TestShufInvalidRange(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name string
		args []string
	}{
		{name: "no dash", args: []string{"-i", "15"}},
		{name: "reversed range", args: []string{"-i", "10-5"}},
		{name: "non-numeric", args: []string{"-i", "abc-def"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin, tc.args...)
			err := cmd.Run()
			if err == nil {
				t.Fatal("expected error for invalid range")
			}
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected ExitError, got %T: %v", err, err)
			}
			if exitErr.ExitCode() != 1 {
				t.Fatalf("exit code: got %d, want 1", exitErr.ExitCode())
			}
		})
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
		{
			name:      "range 1-5",
			args:      []string{"-i", "1-5"},
			wantCount: 5,
		},
		{
			name:      "range with head count",
			args:      []string{"-i", "1-10", "-n", "3"},
			wantCount: 3,
		},
		{
			name:      "head count from stdin",
			args:      []string{"-n", "2"},
			stdin:     "a\nb\nc\nd\ne\n",
			wantCount: 2,
		},
		{
			name:      "repeat with head count",
			args:      []string{"-r", "-n", "10"},
			stdin:     "a\nb\nc\n",
			wantCount: 10,
		},
		{
			name:      "repeat range with head count",
			args:      []string{"-r", "-n", "15", "-i", "1-5"},
			wantCount: 15,
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
// expected line count and contain the same set of lines (when not using -r).
func verifyStructural(t *testing.T, goBin, refBin string, args []string, stdin string, wantCount int) {
	t.Helper()
	goOut := runShuf(t, goBin, args, stdin)
	refOut := runShuf(t, refBin, args, stdin)

	goLines := splitLines(goOut)
	refLines := splitLines(refOut)

	if len(goLines) != wantCount {
		t.Fatalf("go binary: got %d lines, want %d\noutput: %q",
			len(goLines), wantCount, string(goOut))
	}
	if len(refLines) != wantCount {
		t.Fatalf("ref binary: got %d lines, want %d\noutput: %q",
			len(refLines), wantCount, string(refOut))
	}

	// When neither -r nor -n is used, both should contain the same line set.
	hasSubset := false
	for _, a := range args {
		if a == "-r" || a == "--repeat" || a == "-n" || a == "--head-count" {
			hasSubset = true
			break
		}
	}
	if !hasSubset {
		goSorted := sortedLines(goOut)
		refSorted := sortedLines(refOut)
		for i := range goSorted {
			if goSorted[i] != refSorted[i] {
				t.Fatalf("line set mismatch at %d: go=%q ref=%q",
					i, goSorted[i], refSorted[i])
			}
		}
	}
}

// runShuf executes a shuf binary with the given args and stdin.
func runShuf(t *testing.T, binary string, args []string, stdin string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	if stdin != "" {
		cmd.Stdin = bytes.NewBufferString(stdin)
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s failed: %v", binary, err)
	}
	return out
}

// verifyIntRange checks that all lines are integers within [min, max].
func verifyIntRange(t *testing.T, lines []string, min, max int) {
	t.Helper()
	for _, line := range lines {
		n, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("non-integer line: %q", line)
		}
		if n < min || n > max {
			t.Fatalf("value %d out of range [%d, %d]", n, min, max)
		}
	}
}

// verifySortedUnique checks that all lines are unique when sorted.
func verifySortedUnique(t *testing.T, lines []string) {
	t.Helper()
	sorted := make([]string, len(lines))
	copy(sorted, lines)
	sort.Strings(sorted)
	for i := 1; i < len(sorted); i++ {
		if sorted[i] == sorted[i-1] {
			t.Fatalf("duplicate value: %q", sorted[i])
		}
	}
}
