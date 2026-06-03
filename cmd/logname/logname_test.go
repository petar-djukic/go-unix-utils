// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd053-logname R3.1-R3.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glogname")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?logname`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("logname"))
	})

	tests := []testutils.DiffTest{
		{Name: "no_args", Env: []string{"LC_ALL=C"}},
		{Name: "extra_operand", Args: []string{"extraarg"}, ExitCode: 1, Env: []string{"LC_ALL=C"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
		{Name: "unknown_flag", Args: []string{"--unknown"}, ExitCode: 1, Env: []string{"LC_ALL=C"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
