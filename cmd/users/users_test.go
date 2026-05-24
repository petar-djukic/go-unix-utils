// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd096-users R2.1, R2.2, R2.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gusers")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?users`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("users"))
	})
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName}

	tests := []testutils.DiffTest{
		{Name: "no_args", ExitCode: 0},
		{Name: "extra_operand", Args: []string{"/dev/null", "/dev/null"}, ExitCode: 1, Normalize: errNorm},
		{Name: "unrecognized_option", Args: []string{"--foo"}, ExitCode: 1, Normalize: errNorm},
		{Name: "invalid_short_option", Args: []string{"-x"}, ExitCode: 1, Normalize: errNorm},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
