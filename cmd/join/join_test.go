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
	refBin, err := exec.LookPath("gjoin")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
