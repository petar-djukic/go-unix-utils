// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cp against gcp (GNU coreutils).
// Implements srd056 R4.4 (differential testing).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gcp"

// TestDiff runs differential tests comparing cmd/cp against gcp.
// R4.4: empty test slice ready for later test cases.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tests := []testutils.DiffTest{}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
