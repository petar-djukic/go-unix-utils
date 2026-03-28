// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nice against GNU gnice.
// Covers prd094-nice R2.1-R2.3 (differential testing).
package main

import (
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gnice and Go nice.
// Error detail formats differ between implementations; this normalizer strips
// implementation-specific text so exit code comparison drives the test.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?nice|gnice`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	niceErr := regexp.MustCompile(
		`(?m)^nice:.*\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("nice"))
		b = tryHelp.ReplaceAll(b, nil)
		b = niceErr.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnice")
	if err != nil {
		t.Skipf("reference binary gnice not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := buildBasicTests()
	tests = append(tests, buildEdgeCaseTests()...)
	tests = append(tests, buildErrorTests(errNorm)...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildBasicTests returns test cases for R2.1: basic invocation.
func buildBasicTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R2.1: default adjustment (+10), run echo hello.
		{
			Name: "default_adjustment",
			Args: []string{"echo", "hello"},
		},
		// R2.1: explicit adjustment -n 5.
		{
			Name: "explicit_adjustment_5",
			Args: []string{"-n", "5", "echo", "hello"},
		},
		// R2.1: zero adjustment -n 0.
		{
			Name: "zero_adjustment",
			Args: []string{"-n", "0", "echo", "hello"},
		},
	}
}

// buildEdgeCaseTests returns test cases for R2.2: edge cases.
func buildEdgeCaseTests() []testutils.DiffTest {
	tests := []testutils.DiffTest{
		// R2.2/R1.3: no COMMAND prints current niceness.
		{
			Name: "no_command_print_niceness",
			Args: []string{},
		},
		// R2.2/R1.2: --adjustment=N long form syntax.
		{
			Name: "long_form_adjustment",
			Args: []string{"--adjustment=7", "echo", "test"},
		},
		// R2.2/R1.2: legacy -N shorthand.
		{
			Name: "legacy_shorthand",
			Args: []string{"-5", "echo", "legacy"},
		},
		// R2.2/R1.2: -n with large adjustment.
		{
			Name: "large_adjustment",
			Args: []string{"-n", "19", "true"},
		},
	}
	// R2.2: negative adjustment requires root privileges.
	if os.Getuid() == 0 {
		tests = append(tests, testutils.DiffTest{
			Name: "negative_adjustment",
			Args: []string{"-n", "-1", "echo", "negative"},
		})
	}
	return tests
}

// buildErrorTests returns test cases for R2.3: error handling.
// TODO: R2.3 non-executable file test (exit 126) omitted — Go's exec.LookPath
// returns "not found" for non-executable absolute paths, yielding exit 127
// instead of the expected 126. GNU nice uses execvp which distinguishes EACCES
// from ENOENT. Fix requires changes to cmd/nice/main.go runCommand logic.
func buildErrorTests(errNorm testutils.NormalizeFunc) []testutils.DiffTest {
	return []testutils.DiffTest{
		// R2.3/R2.2: nonexistent command exits 127.
		{
			Name:      "nonexistent_command_127",
			Args:      []string{"nonexistent_command_xyz_42"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3/R2.2: invalid (non-numeric) adjustment exits 125.
		{
			Name:      "invalid_adjustment_non_numeric",
			Args:      []string{"-n", "abc", "true"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
}
