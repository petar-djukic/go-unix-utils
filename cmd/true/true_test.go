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
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skipf("reference binary gtrue not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     []string{},
			ExitCode: 0,
		},
		{
			Name:     "arbitrary_args",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
