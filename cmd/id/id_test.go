// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd041-id R1.1, R1.2, R1.3 (differential tests)
package main

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for id.
const refBinaryName = "gid"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// normalizeProgramName replaces the reference binary path/name with a
	// fixed token so stderr messages compare equal despite different argv[0].
	// Use word-boundary-safe pattern: match the full refBin path, or "gid"/"id"
	// only at the start of a line (where argv[0] appears in error messages).
	programNamePattern := regexp.MustCompile(`(?:` + regexp.QuoteMeta(refBin) + `|^gid|^id)`)
	normalizeProgramName := func(b []byte) []byte {
		return programNamePattern.ReplaceAll(b, []byte("PROG"))
	}

	// normalizeErrorSuffix strips OS-specific error detail from "no such user" messages.
	// GNU id appends ": Invalid argument" or similar; our Go implementation does not.
	errorSuffixPattern := regexp.MustCompile(`(no such user)[^\n]*`)
	normalizeErrorSuffix := func(b []byte) []byte {
		return errorSuffixPattern.ReplaceAll(b, []byte("$1"))
	}

	stderrNorm := []testutils.NormalizeFunc{normalizeProgramName, normalizeErrorSuffix}

	// Get the current username for named-user tests.
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}
	currentUsername := currentUser.Username

	tests := []testutils.DiffTest{
		// R1.1: default output — uid=N(name) gid=N(name) groups=...
		{
			Name: "no_args",
			Args: []string{},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: named user lookup.
		{
			Name: "current_user_named",
			Args: []string{currentUsername},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: root user lookup.
		{
			Name: "root_user",
			Args: []string{"root"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: nonexistent user — exit 1 with error on stderr.
		{
			Name:      "nonexistent_user",
			Args:      []string{"nonexistent_user_xyz_99"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion verifies --help and --version exit 0.
// Output content differs between implementations, so stdout/stderr are
// normalized to empty; only exit codes are compared.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	clearOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
