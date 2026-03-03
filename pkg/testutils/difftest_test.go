// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Unit tests for the differential testing harness.
// Implements: prd001-testutils R1-R5; rel00.0-uc004-testutils
package testutils_test

import (
	"bytes"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestRunDiffTests_EchoPass verifies that RunDiffTests passes when both
// binaries produce identical stdout, stderr, and exit code. Uses /bin/echo
// as both the Go binary and reference binary for deterministic output.
// (prd001-testutils R1.1, R2.1, R2.4, R3.6)
func TestRunDiffTests_EchoPass(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name: "echo_hello",
			Args: []string{"hello", "world"},
		},
		{
			Name: "echo_no_args",
		},
	}
	testutils.RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestRunDiffTests_StdinPass verifies that RunDiffTests passes when both
// binaries receive identical stdin and produce matching output. Uses /bin/cat
// to echo stdin back to stdout. (prd001-testutils R1.2, R2.1)
func TestRunDiffTests_StdinPass(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "cat_stdin",
			Stdin: []byte("hello from stdin\n"),
		},
	}
	testutils.RunDiffTests(t, "/bin/cat", "/bin/cat", tests)
}

// TestTimestampNormalizer verifies that TimestampNormalizer replaces all three
// supported timestamp patterns with the "TIMESTAMP" placeholder.
// (prd001-testutils R4.2)
func TestTimestampNormalizer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "month_day_time",
			input: "Feb 19 12:34:56 some log",
			want:  "TIMESTAMP some log",
		},
		{
			name:  "iso_datetime_T",
			input: "2026-02-19T12:34:56 event",
			want:  "TIMESTAMP event",
		},
		{
			name:  "iso_datetime_space",
			input: "2026-02-19 12:34:56 event",
			want:  "TIMESTAMP event",
		},
		{
			name:  "time_only",
			input: "at 12:34:56 done",
			want:  "at TIMESTAMP done",
		},
		{
			name:  "no_timestamp",
			input: "plain text with no time",
			want:  "plain text with no time",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testutils.TimestampNormalizer([]byte(tt.input))
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("TimestampNormalizer(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestComposeNormalizers verifies that ComposeNormalizers applies functions in
// registration order. fn1 replaces "A" with "B", fn2 replaces "B" with "C".
// Applied in order to "A": A->B->C. If order were reversed, the result would
// be "B" (fn2 finds no "B" in "A", then fn1 turns "A" into "B").
// (prd001-testutils R4.3, R4.4)
func TestComposeNormalizers(t *testing.T) {
	fn1 := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("A"), []byte("B")) }
	fn2 := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("B"), []byte("C")) }

	composed := testutils.ComposeNormalizers(fn1, fn2)
	got := composed([]byte("A"))
	want := []byte("C")
	if !bytes.Equal(got, want) {
		t.Errorf("ComposeNormalizers(A->B, B->C)(\"A\") = %q, want %q", got, want)
	}
}

// TestComposeNormalizers_Empty verifies that ComposeNormalizers with no
// arguments returns a no-op normalizer that passes input through unchanged.
// (prd001-testutils R4.4)
func TestComposeNormalizers_Empty(t *testing.T) {
	composed := testutils.ComposeNormalizers()
	input := []byte("unchanged")
	got := composed(input)
	if !bytes.Equal(got, input) {
		t.Errorf("ComposeNormalizers()(\"unchanged\") = %q, want %q", got, input)
	}
}

// TestRunDiffTests_EnvPropagation verifies that custom Env entries are
// propagated to both binaries via the environment. Uses /bin/sh to echo
// a custom environment variable. (prd001-testutils R1.3, R2.6)
func TestRunDiffTests_EnvPropagation(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name: "custom_env_var",
			Args: []string{"-c", "echo $DIFFTEST_CUSTOM_VAR"},
			Env:  []string{"DIFFTEST_CUSTOM_VAR=hello_from_env"},
		},
	}
	testutils.RunDiffTests(t, "/bin/sh", "/bin/sh", tests)
}

// TestRunDiffTests_LcAllDefault verifies that LC_ALL=C is set by default when
// DiffTest.Env is nil. (prd001-testutils R2.6)
func TestRunDiffTests_LcAllDefault(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name: "lc_all_default",
			Args: []string{"-c", "echo $LC_ALL"},
		},
	}
	testutils.RunDiffTests(t, "/bin/sh", "/bin/sh", tests)
}

// TestRunDiffTests_ExpectedFiles verifies that ExpectedFiles comparison
// succeeds when the binary writes expected content to WorkDir.
// (prd001-testutils R5.1, R5.2)
func TestRunDiffTests_ExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	tests := []testutils.DiffTest{
		{
			Name:    "file_written",
			Args:    []string{"-c", "echo hello > output.txt"},
			WorkDir: dir,
			ExpectedFiles: map[string][]byte{
				"output.txt": []byte("hello\n"),
			},
		},
	}
	testutils.RunDiffTests(t, "/bin/sh", "/bin/sh", tests)
}

// TestRunDiffTests_ExitCodeMatch verifies that RunDiffTests passes when both
// binaries exit with the expected non-zero exit code.
// (prd001-testutils R3.4)
func TestRunDiffTests_ExitCodeMatch(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:     "exit_one",
			Args:     []string{"-c", "exit 1"},
			ExitCode: 1,
		},
	}
	testutils.RunDiffTests(t, "/bin/sh", "/bin/sh", tests)
}
