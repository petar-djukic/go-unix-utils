// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd001-testutils R3.1-R3.4: output comparison, diff reporting,
// and error formatting.

package testutils

import (
	"bytes"
	"os/exec"
	"testing"
)

// ---------------------------------------------------------------------------
// R3.1: applyNormalizers applies normalizers to both ref and go output
// ---------------------------------------------------------------------------

// TestApplyNormalizers_BothOutputs verifies R3.1: normalizers are applied to
// both reference and Go output before comparison.
func TestApplyNormalizers_BothOutputs(t *testing.T) {
	t.Parallel()

	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	ref := []byte("hello")
	got := []byte("Hello")

	normRef, normGot := applyNormalizers([]NormalizeFunc{upper}, ref, got)
	if string(normRef) != "HELLO" {
		t.Errorf("ref not normalized: got %q, want %q", normRef, "HELLO")
	}
	if string(normGot) != "HELLO" {
		t.Errorf("got not normalized: got %q, want %q", normGot, "HELLO")
	}
}

// TestApplyNormalizers_NilSlice verifies R3.1: nil normalizer slice is a no-op.
func TestApplyNormalizers_NilSlice(t *testing.T) {
	t.Parallel()

	ref := []byte("unchanged")
	got := []byte("unchanged")
	normRef, normGot := applyNormalizers(nil, ref, got)
	if !bytes.Equal(normRef, ref) {
		t.Errorf("ref changed with nil normalizers: %q", normRef)
	}
	if !bytes.Equal(normGot, got) {
		t.Errorf("got changed with nil normalizers: %q", normGot)
	}
}

// TestApplyNormalizers_EmptySlice verifies R3.1: empty slice is a no-op.
func TestApplyNormalizers_EmptySlice(t *testing.T) {
	t.Parallel()

	ref := []byte("unchanged")
	got := []byte("unchanged")
	normRef, normGot := applyNormalizers([]NormalizeFunc{}, ref, got)
	if !bytes.Equal(normRef, ref) {
		t.Errorf("ref changed with empty normalizers: %q", normRef)
	}
	if !bytes.Equal(normGot, got) {
		t.Errorf("got changed with empty normalizers: %q", normGot)
	}
}

// TestApplyNormalizers_MultipleInOrder verifies R3.1: normalizers apply in order.
func TestApplyNormalizers_MultipleInOrder(t *testing.T) {
	t.Parallel()

	appendA := func(b []byte) []byte { return append(b, 'A') }
	appendB := func(b []byte) []byte { return append(b, 'B') }

	ref := []byte("x")
	got := []byte("y")
	normRef, normGot := applyNormalizers(
		[]NormalizeFunc{appendA, appendB}, ref, got,
	)
	if string(normRef) != "xAB" {
		t.Errorf("ref order wrong: got %q, want %q", normRef, "xAB")
	}
	if string(normGot) != "yAB" {
		t.Errorf("got order wrong: got %q, want %q", normGot, "yAB")
	}
}

// ---------------------------------------------------------------------------
// R3.2-R3.4: compareResults via RunDiffTests integration
// Uses the same system binary as both Go and ref to verify matching passes.
// ---------------------------------------------------------------------------

// TestRunDiffTests_StdoutMatch_R3_2 verifies R3.2: identical stdout from
// both binaries produces no test failure.
func TestRunDiffTests_StdoutMatch_R3_2(t *testing.T) {
	t.Parallel()

	echoBin := lookupEcho(t)
	tests := []DiffTest{
		{Name: "same-stdout", Args: []string{"hello world"}, ExitCode: 0},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

// TestRunDiffTests_StderrMatch_R3_3 verifies R3.3: identical stderr from
// both binaries produces no test failure. Using true which produces no stderr.
func TestRunDiffTests_StderrMatch_R3_3(t *testing.T) {
	t.Parallel()

	trueBin := lookupTrue(t)
	tests := []DiffTest{
		{Name: "no-stderr", ExitCode: 0},
	}
	RunDiffTests(t, trueBin, trueBin, tests)
}

// TestRunDiffTests_ExitCodeMatch_R3_4 verifies R3.4: identical exit codes
// from both binaries produces no test failure.
func TestRunDiffTests_ExitCodeMatch_R3_4(t *testing.T) {
	t.Parallel()

	falseBin := lookupFalse(t)
	tests := []DiffTest{
		{Name: "same-exit-code", ExitCode: 1},
	}
	RunDiffTests(t, falseBin, falseBin, tests)
}

// TestRunDiffTests_NormalizerApplied_R3_1 verifies R3.1: normalizers are
// applied before comparison. Both binaries produce identical output; the
// normalizer transforms both identically so comparison still passes.
func TestRunDiffTests_NormalizerApplied_R3_1(t *testing.T) {
	t.Parallel()

	echoBin := lookupEcho(t)
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	tests := []DiffTest{
		{
			Name:      "normalized-match",
			Args:      []string{"test data"},
			ExitCode:  0,
			Normalize: []NormalizeFunc{upper},
		},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

// TestRunDiffTests_MultipleNormalizers verifies R3.1: multiple normalizers
// are applied in sequence.
func TestRunDiffTests_MultipleNormalizers(t *testing.T) {
	t.Parallel()

	echoBin := lookupEcho(t)
	identity := func(b []byte) []byte { return b }
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	tests := []DiffTest{
		{
			Name:      "chained-normalizers",
			Args:      []string{"hello"},
			ExitCode:  0,
			Normalize: []NormalizeFunc{identity, upper},
		},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

// ---------------------------------------------------------------------------
// truncateBytes helper
// ---------------------------------------------------------------------------

// TestTruncateBytes verifies the stdin truncation used in failure messages.
func TestTruncateBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  []byte
		maxLen int
		want   int
	}{
		{"under limit", []byte("short"), 10, 5},
		{"at limit", []byte("exact"), 5, 5},
		{"over limit", []byte("this is longer"), 4, 4},
		{"nil input", nil, 10, 0},
		{"empty input", []byte{}, 10, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateBytes(tc.input, tc.maxLen)
			if len(got) != tc.want {
				t.Errorf("truncateBytes len = %d, want %d", len(got), tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// lookupEcho finds the echo binary or skips the test.
func lookupEcho(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not found in PATH")
	}
	return bin
}

// lookupTrue finds the true binary or skips the test.
func lookupTrue(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true not found in PATH")
	}
	return bin
}

// lookupFalse finds the false binary or skips the test.
func lookupFalse(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false not found in PATH")
	}
	return bin
}
