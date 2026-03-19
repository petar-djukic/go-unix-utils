// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd019-seq R1.1–R1.4: numeric sequence generation.
package main

import (
	"bytes"
	"io"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

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
