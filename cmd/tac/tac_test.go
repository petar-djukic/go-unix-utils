// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go tac binary against
// the GNU reference binary gtac.
//
// Implements prd021-tac R4.1, R4.2, R4.3.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skipf("reference binary gtac not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
