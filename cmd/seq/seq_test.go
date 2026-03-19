// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd019-seq R1.1–R1.4, R2.4, R3.1–R3.3: numeric sequence generation,
// large integers, format strings, format validation, and equal-width padding.
package main

import (
	"bytes"
	"io"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgName normalizes the program name prefix in stderr so that
// "gseq:" and "seq:" compare equal in differential tests.
func normProgName(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gseq:"), []byte("seq:"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skipf("reference binary gseq not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.1: single argument form (seq LAST)
		{Name: "single-arg-5", Args: []string{"5"}},
		{Name: "single-arg-1", Args: []string{"1"}},
		// R1.1: two argument form (seq FIRST LAST)
		{Name: "two-args-2-5", Args: []string{"2", "5"}},
		{Name: "two-args-negative-range", Args: []string{"-3", "-1"}},
		// R1.1: three argument form (seq FIRST INCREMENT LAST)
		{Name: "three-args-1-2-10", Args: []string{"1", "2", "10"}},
		{Name: "descending-5-to-1", Args: []string{"5", "-1", "1"}},
		{Name: "step-3", Args: []string{"1", "3", "15"}},
		// R1.3: FIRST equals LAST
		{Name: "first-equals-last-two-args", Args: []string{"3", "3"}},
		{Name: "first-equals-last-three-args", Args: []string{"5", "1", "5"}},
		// R1.4: empty sequence
		{Name: "empty-pos-step-first-gt-last", Args: []string{"5", "1", "1"}},
		{Name: "empty-neg-step-first-lt-last", Args: []string{"1", "-1", "5"}},
		// R1.4: floating-point sequences
		{Name: "float-0.5-0.5-2.5", Args: []string{"0.5", "0.5", "2.5"}},
		{Name: "float-0.1-0.1-0.5", Args: []string{"0.1", "0.1", "0.5"}},
		{Name: "float-two-args", Args: []string{"0.5", "3"}},
		{Name: "negative-descending", Args: []string{"-1", "-1", "-5"}},
		{Name: "large-range", Args: []string{"1", "100"}},
		// R2.4: large integers up to 2^53
		{Name: "large-int-2-53", Args: []string{"9007199254740990", "9007199254740992"}},
		// R3.1: format string
		{Name: "format-point2f", Args: []string{"-f", "%.2f", "1", "3"}},
		{Name: "format-e-notation", Args: []string{"-f", "%e", "1", "3"}},
		{Name: "format-g", Args: []string{"-f", "%g", "1", "5"}},
		// R3.2: invalid format strings (exit code 1)
		// Normalize program name: gseq → seq in stderr.
		{Name: "format-no-specifier", Args: []string{"-f", "hello", "1", "3"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normProgName}},
		{Name: "format-unknown-d", Args: []string{"-f", "%d", "1", "3"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normProgName}},
		{Name: "format-too-many", Args: []string{"-f", "%f%f", "1", "3"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normProgName}},
		// R3.3: equal-width
		{Name: "equal-width-8-12", Args: []string{"-w", "8", "12"}},
		{Name: "equal-width-1-10", Args: []string{"-w", "1", "10"}},
		{Name: "equal-width-neg", Args: []string{"-w", "-5", "5"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSequenceOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"single-arg-5", []string{"5"}, "1\n2\n3\n4\n5\n"},
		{"two-args-2-5", []string{"2", "5"}, "2\n3\n4\n5\n"},
		{"three-args-1-2-10", []string{"1", "2", "10"}, "1\n3\n5\n7\n9\n"},
		{"float-seq", []string{"0.5", "0.5", "2.5"}, "0.5\n1.0\n1.5\n2.0\n2.5\n"},
		{"descending", []string{"5", "-1", "1"}, "5\n4\n3\n2\n1\n"},
		{"empty-sequence", []string{"5", "1", "1"}, ""},
		{"first-equals-last", []string{"3", "3"}, "3\n"},
		{"negative-range", []string{"-3", "-1"}, "-3\n-2\n-1\n"},
		{"single-1", []string{"1"}, "1\n"},
		{"negative-descend", []string{"-1", "-1", "-5"}, "-1\n-2\n-3\n-4\n-5\n"},
		// R2.4: large integers up to 2^53
		{"large-int-2-53", []string{"9007199254740990", "9007199254740992"},
			"9007199254740990\n9007199254740991\n9007199254740992\n"},
		// R3.1: format string
		{"format-point2f", []string{"-f", "%.2f", "1", "3"}, "1.00\n2.00\n3.00\n"},
		{"format-g", []string{"-f", "%g", "1", "5"}, "1\n2\n3\n4\n5\n"},
		// R3.3: equal-width
		{"equal-width-8-12", []string{"-w", "8", "12"}, "08\n09\n10\n11\n12\n"},
		{"equal-width-1-10", []string{"-w", "1", "10"},
			"01\n02\n03\n04\n05\n06\n07\n08\n09\n10\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			code := run(tc.args, &buf, io.Discard)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"no-args", nil, 1},
		{"non-numeric", []string{"abc"}, 1},
		{"zero-step", []string{"1", "0", "5"}, 1},
		{"too-many-args", []string{"1", "2", "3", "4"}, 1},
		{"unrecognized-flag", []string{"-x"}, 1},
		// R3.2: format validation errors
		{"format-no-specifier", []string{"-f", "hello", "1", "3"}, 1},
		{"format-too-many", []string{"-f", "%f%f", "1", "3"}, 1},
		{"format-unknown-d", []string{"-f", "%d", "1", "3"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d", code, tc.wantCode)
			}
			if stderr.Len() == 0 {
				t.Error("expected error message on stderr")
			}
		})
	}
}
