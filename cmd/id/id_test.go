// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd041-id R4.1, R4.2.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gid")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{Name: "default_no_flags"},
		{Name: "flag_u", Args: []string{"-u"}},
		{Name: "flag_long_user", Args: []string{"--user"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
