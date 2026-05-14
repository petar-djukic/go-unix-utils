// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfalse")
	if err != nil {
		t.Skip("reference binary not found")
	}
	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })
	tests := []testutils.DiffTest{
		{
			Name:     "no-args",
			ExitCode: 1,
		},
		{
			Name:     "arbitrary-args",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 1,
		},
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
