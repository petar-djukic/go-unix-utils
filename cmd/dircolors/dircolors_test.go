// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skipf("reference binary gdircolors not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name: "default no args bourne shell",
			Args: []string{"--sh"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "explicit bourne -b",
			Args: []string{"-b"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "explicit bourne --bourne-shell",
			Args: []string{"--bourne-shell"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "csh flag -c",
			Args: []string{"-c"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "csh flag --csh",
			Args: []string{"--csh"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "csh flag --c-shell",
			Args: []string{"--c-shell"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "auto-detect bourne from SHELL",
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "auto-detect csh from SHELL",
			Env:  []string{"SHELL=/bin/tcsh"},
		},
		{
			Name: "auto-detect csh from SHELL csh",
			Env:  []string{"SHELL=/bin/csh"},
		},
		{
			Name: "last flag wins b then c",
			Args: []string{"-b", "-c"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "last flag wins c then b",
			Args: []string{"-c", "-b"},
			Env:  []string{"SHELL=/bin/bash"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
