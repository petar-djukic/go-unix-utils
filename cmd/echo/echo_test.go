// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies cmd/echo against the GNU reference binary gecho.
// Implements prd020-echo R4.1-R4.3.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gecho")
	if err != nil {
		t.Skipf("reference binary gecho not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// TODO R4.2: Add test cases for all flag and argument combinations.
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
