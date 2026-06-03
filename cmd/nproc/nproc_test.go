// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd046-nproc R3.1-R3.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnproc")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?nproc`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("nproc"))
	})

	tests := []testutils.DiffTest{
		{
			Name: "default",
		},
		{
			Name: "all",
			Args: []string{"--all"},
		},
		{
			Name: "ignore-1",
			Args: []string{"--ignore=1"},
		},
		{
			Name: "all-ignore-1",
			Args: []string{"--all", "--ignore=1"},
		},
		{
			Name:      "error-extra-operand",
			Args:      []string{"foo"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "error-non-numeric-ignore",
			Args:      []string{"--ignore=abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "error-unknown-flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
