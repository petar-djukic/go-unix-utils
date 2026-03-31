// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// users_test.go implements differential tests for cmd/users against gusers.

package main_test

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name and full path
// in stderr with the canonical program name so stderr comparison succeeds
// despite binary name differences (gusers vs users, path differences).
var stderrProgNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	// Replace full path references like /opt/homebrew/bin/gusers with users.
	re := regexp.MustCompile(`(/[^\s']*/)?(g?)users`)
	return re.ReplaceAll(data, []byte("users"))
}


func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gusers")
	if err != nil {
		t.Skip("reference binary gusers not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: prints logged-in usernames sorted and space-separated.
			// R2.1: exits 0 on success.
			Name: "users_list",
			Args: []string{},
		},
		{
			// R2.2: exits 1 on extra operand error.
			Name:     "extra_operand",
			Args:     []string{"/dev/null", "/dev/null"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				stderrProgNameNormalizer,
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
