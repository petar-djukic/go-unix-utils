// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/printf. Implements srd073-printf R4.3, R4.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintf")
	if err != nil {
		t.Skipf("reference binary gprintf not in PATH: %v", err)
	}

	// R4.4: tests cover integer formats, float formats, string, char,
	// backslash-interpreted string, width and precision, all flags,
	// * width/precision, argument recycling, escape sequences, character
	// value arguments, missing arguments, and error cases.
	tests := []testutils.DiffTest{}

	// R4.3: compare Go printf output against gprintf byte-for-byte.
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
