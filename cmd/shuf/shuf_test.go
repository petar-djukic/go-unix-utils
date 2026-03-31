// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// shuf_test.go implements differential and structural tests for
// prd064-shuf R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.4.

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

	// R2.1: range mode
	t.Run("range_mode_i_1_5", func(t *testing.T) {
		assertRangeMatch(t, goBin, refBin, []string{"-i", "1-5"}, 5, 1, 5)
	})

	// R2.2: head count
	t.Run("head_count_n3", func(t *testing.T) {
		stdin := []byte("a\nb\nc\nd\ne\n")
		assertLineCount(t, goBin, refBin, []string{"-n", "3"}, stdin, 3)
	})

	// R2.2 + R2.1: head count with range
	t.Run("range_with_head_count", func(t *testing.T) {
		assertRangeHeadCount(t, goBin, refBin,
			[]string{"-i", "1-10", "-n", "3"}, 3, 1, 10)
	})

	// R2.3: repeat with head count
	t.Run("repeat_with_head_count", func(t *testing.T) {
		stdin := []byte("a\nb\nc\n")
		assertRepeatMatch(t, goBin, refBin,
			[]string{"-r", "-n", "10"}, stdin, 10, []string{"a", "b", "c"})
	})

	// R2.4: output to file
	t.Run("output_file", func(t *testing.T) {
		assertOutputFile(t, goBin, refBin)
	})

	// R2.1 error: -i with file arguments
	t.Run("range_with_files_error", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "f.txt")
		os.WriteFile(f, []byte("a\n"), 0o644)
		assertBothFail(t, goBin, refBin, []string{"-i", "1-5", f}, nil)
	})

	// R3.1: random source
	t.Run("random_source", func(t *testing.T) {
		assertRandomSource(t, goBin, refBin)
	})

	// R3.2: zero-terminated
	t.Run("zero_terminated", func(t *testing.T) {
		stdin := []byte("a\x00b\x00c\x00")
		assertZeroTermMatch(t, goBin, refBin, []string{"-z"}, stdin, 3)
	})

	// R3.3: echo mode
	t.Run("echo_mode", func(t *testing.T) {
		assertStructuralMatch(t, goBin, refBin,
			[]string{"-e", "alpha", "beta", "gamma"}, nil, 3)
	})

	// R3.3: echo with head count
	t.Run("echo_with_head_count", func(t *testing.T) {
		assertLineCount(t, goBin, refBin,
			[]string{"-e", "-n", "2", "a", "b", "c", "d"}, nil, 2)
	})

	// R3.4: empty echo input
	t.Run("echo_empty", func(t *testing.T) {
		assertStructuralMatch(t, goBin, refBin, []string{"-e"}, nil, 0)
	})

	// R4.1: exit 0 on successful operation
	t.Run("exit_0_on_success", func(t *testing.T) {
		assertExitCode(t, goBin, []string{"-e", "a", "b"}, nil, 0)
		assertExitCode(t, refBin, []string{"-e", "a", "b"}, nil, 0)
	})

	// R4.2: exit 1 on invalid range
	t.Run("exit_1_invalid_range", func(t *testing.T) {
		assertBothFail(t, goBin, refBin, []string{"-i", "abc"}, nil)
	})

	// R4.2: exit 1 on reversed range
	t.Run("exit_1_reversed_range", func(t *testing.T) {
		assertBothFail(t, goBin, refBin, []string{"-i", "10-1"}, nil)
	})

	// R4.2: exit 1 on conflicting -e and -i
	t.Run("exit_1_echo_with_range", func(t *testing.T) {
		assertBothFail(t, goBin, refBin, []string{"-e", "-i", "1-5", "a"}, nil)
	})

	// R4.4: shuffle stdin lines (covered above, explicit re-check)
	t.Run("r4_4_shuffle_stdin", func(t *testing.T) {
		stdin := []byte("d\ne\nf\n")
		assertStructuralMatch(t, goBin, refBin, nil, stdin, 3)
	})

	// R4.4: -i range with -n and -r combined
	t.Run("r4_4_range_repeat_head", func(t *testing.T) {
		assertRepeatRangeMatch(t, goBin, refBin,
			[]string{"-i", "1-3", "-r", "-n", "8"}, 8, 1, 3)
	})
}

// assertOutputFile verifies R2.4: -o writes to file for both binaries.
func assertOutputFile(t *testing.T, goBin, refBin string) {
	t.Helper()
	dir := t.TempDir()
	outGo := filepath.Join(dir, "go_out.txt")
	outRef := filepath.Join(dir, "ref_out.txt")
	stdin := []byte("x\ny\nz\n")

	runBinNoOutput(t, goBin, []string{"-o", outGo}, stdin)
	runBinNoOutput(t, refBin, []string{"-o", outRef}, stdin)

	goData, err := os.ReadFile(outGo)
	if err != nil {
		t.Fatalf("reading go output file: %v", err)
	}
	refData, err := os.ReadFile(outRef)
	if err != nil {
		t.Fatalf("reading ref output file: %v", err)
	}
	goLines := nonEmptyLines(string(goData))
	refLines := nonEmptyLines(string(refData))
	if len(goLines) != 3 || len(refLines) != 3 {
		t.Errorf("expected 3 lines each, go=%d ref=%d",
			len(goLines), len(refLines))
	}
	assertSameSet(t, goLines, refLines)
}

// assertRandomSource verifies R3.1: --random-source is accepted by both binaries.
func assertRandomSource(t *testing.T, goBin, refBin string) {
	t.Helper()
	dir := t.TempDir()
	rsrc := filepath.Join(dir, "randsrc")
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}
	os.WriteFile(rsrc, data, 0o644)
	stdin := []byte("a\nb\nc\n")
	goOut := runBin(t, goBin, []string{"--random-source=" + rsrc}, stdin)
	refOut := runBin(t, refBin, []string{"--random-source=" + rsrc}, stdin)
	goLines := nonEmptyLines(goOut)
	refLines := nonEmptyLines(refOut)
	if len(goLines) != 3 {
		t.Errorf("go: expected 3 lines, got %d", len(goLines))
	}
	if len(refLines) != 3 {
		t.Errorf("ref: expected 3 lines, got %d", len(refLines))
	}
	assertSameSet(t, goLines, refLines)
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
	got := sortedCopy(lines)
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
	got := sortedCopy(lines)
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
	got := sortedCopy(lines)
	if !equalSlices(got, expected) {
		t.Errorf("expected set %v, got %v", expected, got)
	}
}

// TestRangeMode verifies R2.1: -i LO-HI generates integers in range.
func TestRangeMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	out := runBin(t, goBin, []string{"-i", "1-5"}, nil)
	lines := nonEmptyLines(out)

	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}

	seen := make(map[int]bool)
	for _, line := range lines {
		n, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("non-integer output: %q", line)
		}
		if n < 1 || n > 5 {
			t.Errorf("value %d out of range [1,5]", n)
		}
		if seen[n] {
			t.Errorf("duplicate value %d", n)
		}
		seen[n] = true
	}
}

// TestHeadCount verifies R2.2: -n limits output count.
func TestHeadCount(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	stdin := []byte("a\nb\nc\nd\ne\n")
	out := runBin(t, goBin, []string{"-n", "2"}, stdin)
	lines := nonEmptyLines(out)

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

// TestRepeatMode verifies R2.3: -r allows duplicates.
func TestRepeatMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	stdin := []byte("a\nb\n")
	out := runBin(t, goBin, []string{"-r", "-n", "20"}, stdin)
	lines := nonEmptyLines(out)

	if len(lines) != 20 {
		t.Fatalf("expected 20 lines, got %d", len(lines))
	}
	for _, line := range lines {
		if line != "a" && line != "b" {
			t.Errorf("unexpected value %q", line)
		}
	}
}

// TestOutputFile verifies R2.4: -o writes to file.
func TestOutputFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	stdin := []byte("p\nq\nr\n")
	runBinNoOutput(t, goBin, []string{"-o", outFile}, stdin)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	lines := nonEmptyLines(string(data))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines in output file, got %d", len(lines))
	}
	got := sortedCopy(lines)
	if !equalSlices(got, []string{"p", "q", "r"}) {
		t.Errorf("expected {p,q,r}, got %v", got)
	}
}

// TestEchoMode verifies R3.3: -e treats args as input lines.
func TestEchoMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	out := runBin(t, goBin, []string{"-e", "alpha", "beta", "gamma"}, nil)
	lines := nonEmptyLines(out)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	got := sortedCopy(lines)
	if !equalSlices(got, []string{"alpha", "beta", "gamma"}) {
		t.Errorf("expected {alpha,beta,gamma}, got %v", got)
	}
}

// TestZeroTerminated verifies R3.2: -z uses NUL delimiter.
func TestZeroTerminated(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	stdin := []byte("a\x00b\x00c\x00")
	out := runBin(t, goBin, []string{"-z"}, stdin)
	items := splitNul(out)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(items), items)
	}
	got := sortedCopy(items)
	if !equalSlices(got, []string{"a", "b", "c"}) {
		t.Errorf("expected {a,b,c}, got %v", got)
	}
}

// TestRandomSourceDeterministic verifies R3.1: same random source yields same output.
func TestRandomSourceDeterministic(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	rsrc := filepath.Join(dir, "randsrc")
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	os.WriteFile(rsrc, data, 0o644)

	stdin := []byte("a\nb\nc\nd\ne\n")
	out1 := runBin(t, goBin, []string{"--random-source=" + rsrc}, stdin)
	out2 := runBin(t, goBin, []string{"--random-source=" + rsrc}, stdin)
	if out1 != out2 {
		t.Errorf("same random source gave different output:\n  1: %q\n  2: %q", out1, out2)
	}
	lines := nonEmptyLines(out1)
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
}

// TestEmptyInput verifies R3.4: empty input produces no output.
func TestEmptyInput(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	out := runBin(t, goBin, nil, nil)
	lines := nonEmptyLines(out)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d: %v", len(lines), lines)
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
	assertSameSet(t, goLines, refLines)
}

// assertZeroTermMatch runs both binaries with -z and compares NUL-delimited output.
func assertZeroTermMatch(
	t *testing.T, goBin, refBin string,
	args []string, stdin []byte, expectedCount int,
) {
	t.Helper()
	goOut := runBin(t, goBin, args, stdin)
	refOut := runBin(t, refBin, args, stdin)
	goItems := splitNul(goOut)
	refItems := splitNul(refOut)
	if len(goItems) != expectedCount {
		t.Errorf("go: expected %d items, got %d", expectedCount, len(goItems))
	}
	if len(refItems) != expectedCount {
		t.Errorf("ref: expected %d items, got %d", expectedCount, len(refItems))
	}
	assertSameSet(t, goItems, refItems)
}

// assertRangeMatch verifies both binaries produce the same integer set.
func assertRangeMatch(
	t *testing.T, goBin, refBin string,
	args []string, expectedCount, lo, hi int,
) {
	t.Helper()
	goOut := runBin(t, goBin, args, nil)
	refOut := runBin(t, refBin, args, nil)

	goLines := nonEmptyLines(goOut)
	refLines := nonEmptyLines(refOut)

	if len(goLines) != expectedCount {
		t.Errorf("go: expected %d lines, got %d", expectedCount, len(goLines))
	}
	if len(refLines) != expectedCount {
		t.Errorf("ref: expected %d lines, got %d", expectedCount, len(refLines))
	}
	assertIntRange(t, "go", goLines, lo, hi)
	assertIntRange(t, "ref", refLines, lo, hi)
}

// assertRangeHeadCount verifies range with -n limit.
func assertRangeHeadCount(
	t *testing.T, goBin, refBin string,
	args []string, expectedCount, lo, hi int,
) {
	t.Helper()
	goOut := runBin(t, goBin, args, nil)
	refOut := runBin(t, refBin, args, nil)

	goLines := nonEmptyLines(goOut)
	refLines := nonEmptyLines(refOut)

	if len(goLines) != expectedCount {
		t.Errorf("go: expected %d lines, got %d", expectedCount, len(goLines))
	}
	if len(refLines) != expectedCount {
		t.Errorf("ref: expected %d lines, got %d", expectedCount, len(refLines))
	}
	assertIntRange(t, "go", goLines, lo, hi)
	assertIntRange(t, "ref", refLines, lo, hi)
}

// assertRepeatMatch verifies repeat mode output.
func assertRepeatMatch(
	t *testing.T, goBin, refBin string,
	args []string, stdin []byte, expectedCount int, allowed []string,
) {
	t.Helper()
	goOut := runBin(t, goBin, args, stdin)
	refOut := runBin(t, refBin, args, stdin)

	goLines := nonEmptyLines(goOut)
	refLines := nonEmptyLines(refOut)

	if len(goLines) != expectedCount {
		t.Errorf("go: expected %d lines, got %d", expectedCount, len(goLines))
	}
	if len(refLines) != expectedCount {
		t.Errorf("ref: expected %d lines, got %d", expectedCount, len(refLines))
	}
	allowSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowSet[a] = true
	}
	for _, line := range goLines {
		if !allowSet[line] {
			t.Errorf("go: unexpected value %q", line)
		}
	}
}

// assertExitCode runs a binary and checks its exit code.
// R4.1: used to verify exit 0 on success.
func assertExitCode(
	t *testing.T, bin string,
	args []string, stdin []byte, expectedCode int,
) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("unexpected error running %s: %v", bin, err)
		}
	}
	if code != expectedCode {
		t.Errorf("%s: expected exit code %d, got %d", bin, expectedCode, code)
	}
}

// assertRepeatRangeMatch verifies -i range with -r produces values in range.
// R4.4: combined -i, -r, -n test coverage.
func assertRepeatRangeMatch(
	t *testing.T, goBin, refBin string,
	args []string, expectedCount, lo, hi int,
) {
	t.Helper()
	goOut := runBin(t, goBin, args, nil)
	refOut := runBin(t, refBin, args, nil)
	goLines := nonEmptyLines(goOut)
	refLines := nonEmptyLines(refOut)
	if len(goLines) != expectedCount {
		t.Errorf("go: expected %d lines, got %d", expectedCount, len(goLines))
	}
	if len(refLines) != expectedCount {
		t.Errorf("ref: expected %d lines, got %d", expectedCount, len(refLines))
	}
	assertIntRange(t, "go", goLines, lo, hi)
	assertIntRange(t, "ref", refLines, lo, hi)
}

// assertBothFail verifies both binaries exit non-zero.
func assertBothFail(
	t *testing.T, goBin, refBin string,
	args []string, stdin []byte,
) {
	t.Helper()
	goErr := runBinExpectFail(t, goBin, args, stdin)
	refErr := runBinExpectFail(t, refBin, args, stdin)
	if goErr == nil {
		t.Error("go binary should have failed but succeeded")
	}
	if refErr == nil {
		t.Error("ref binary should have failed but succeeded")
	}
}

// assertIntRange checks that all lines are integers in [lo, hi].
func assertIntRange(t *testing.T, label string, lines []string, lo, hi int) {
	t.Helper()
	for _, line := range lines {
		n, err := strconv.Atoi(line)
		if err != nil {
			t.Errorf("%s: non-integer %q", label, line)
			continue
		}
		if n < lo || n > hi {
			t.Errorf("%s: %d out of range [%d,%d]", label, n, lo, hi)
		}
	}
}

// assertSameSet verifies two line slices contain the same sorted values.
func assertSameSet(t *testing.T, a, b []string) {
	t.Helper()
	aSorted := sortedCopy(a)
	bSorted := sortedCopy(b)
	if !equalSlices(aSorted, bSorted) {
		t.Errorf("value sets differ:\n  a: %v\n  b: %v", aSorted, bSorted)
	}
}

// assertLineCount verifies both binaries produce the expected line count.
func assertLineCount(
	t *testing.T, goBin, refBin string,
	args []string, stdin []byte, expectedCount int,
) {
	t.Helper()
	goOut := runBin(t, goBin, args, stdin)
	refOut := runBin(t, refBin, args, stdin)

	goLines := nonEmptyLines(goOut)
	refLines := nonEmptyLines(refOut)

	if len(goLines) != expectedCount {
		t.Errorf("go: expected %d lines, got %d", expectedCount, len(goLines))
	}
	if len(refLines) != expectedCount {
		t.Errorf("ref: expected %d lines, got %d", expectedCount, len(refLines))
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

// runBinNoOutput executes a binary, ignoring stdout (for -o tests).
func runBinNoOutput(t *testing.T, bin string, args []string, stdin []byte) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("binary %s failed: %v\nstderr: %s", bin, err, stderr.String())
	}
}

// runBinExpectFail executes a binary and returns the error (expects failure).
func runBinExpectFail(t *testing.T, bin string, args []string, stdin []byte) error {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	return cmd.Run()
}

// nonEmptyLines splits output into lines, filtering empty trailing lines.
func nonEmptyLines(s string) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// splitNul splits NUL-delimited output into items, filtering trailing empty items.
func splitNul(s string) []string {
	parts := strings.Split(strings.TrimRight(s, "\x00"), "\x00")
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
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
