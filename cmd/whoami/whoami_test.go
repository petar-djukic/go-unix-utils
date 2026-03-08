// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/whoami against the GNU reference binary (gwhoami).
//
// Implements prd042-whoami acceptance criteria AC1-AC5 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer replaces "gwhoami:" with "whoami:" and removes the
// "Try '...' for more information." line that GNU adds.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gwhoami:"), []byte("whoami:"))
	lines := bytes.Split(b, []byte("\n"))
	var filtered [][]byte
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("Try '")) {
			continue
		}
		filtered = append(filtered, line)
	}
	return bytes.Join(filtered, []byte("\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwhoami")
	if err != nil {
		t.Skipf("reference binary gwhoami not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Print effective username.
		{
			Name: "whoami_default",
			Args: []string{},
		},
		// R2.1: Extra operand causes error.
		{
			Name:      "whoami_extra_operand",
			Args:      []string{"nobody"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
