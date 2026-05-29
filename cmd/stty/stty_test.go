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
	refBin, err := exec.LookPath("gstty")
	if err != nil {
		t.Skip("reference binary not found")
	}
	discardOut := testutils.NormalizeFunc(func([]byte) []byte { return nil })
	tests := []testutils.DiffTest{
		{Name: "default-no-tty", ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "all-flag", Args: []string{"-a"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "all-long", Args: []string{"--all"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "save-flag", Args: []string{"-g"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "save-long", Args: []string{"--save"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "file-dev-null", Args: []string{"-F", "/dev/null"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "file-long", Args: []string{"--file=/dev/null"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-echo", Args: []string{"echo"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "set-neg-echo", Args: []string{"-echo"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "help", Args: []string{"--help"}, ExitCode: 0, Normalize: []testutils.NormalizeFunc{discardOut}},
		{Name: "version", Args: []string{"--version"}, ExitCode: 0, Normalize: []testutils.NormalizeFunc{discardOut}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
