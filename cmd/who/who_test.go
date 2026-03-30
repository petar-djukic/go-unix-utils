// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// who_test.go implements differential tests for cmd/who against gwho.
// Covers prd097-who R1.1, R1.2, R1.3, R1.4.

package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwho")
	if err != nil {
		t.Skip("reference binary gwho not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: prints logged-in users with terminal and time.
			Name: "who_default",
			Args: []string{},
		},
		{
			// R1.3: "who am i" prints only the current terminal entry.
			Name: "who_am_i",
			Args: []string{"am", "i"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
