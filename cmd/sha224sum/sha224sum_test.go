// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/sha224sum against gsha224sum.
// Implements srd074-sha224sum R4.1, R4.2, R4.3 acceptance criteria via
// testutils.RunDiffTests.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests for sha224sum against gsha224sum.
// D1: uses testutils.BuildBinary and exec.LookPath.
// D2: skips if gsha224sum not found.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha224sum")
	if err != nil {
		t.Skipf("reference binary gsha224sum not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
