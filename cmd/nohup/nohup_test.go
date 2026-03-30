// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gnohup:" with "nohup:" in stderr output
// so the Go binary and reference binary error messages can be compared.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gnohup:"), []byte("nohup:"))
}

// normalizeTryPath normalizes "Try '/path/to/gnohup --help'" to
// "Try 'nohup --help'" so paths don't cause divergence.
var tryPathRe = regexp.MustCompile(`Try '[^']*(?:g?nohup)`)

func normalizeTryPath(data []byte) []byte {
	return tryPathRe.ReplaceAll(data, []byte("Try 'nohup"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnohup")
	if err != nil {
		t.Skipf("reference binary gnohup not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			// R1.1, R1.4: basic execution immune to SIGHUP, passes args.
			Name: "basic",
			Args: []string{"echo", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.4: multiple arguments passed to command.
			Name: "multiple_args",
			Args: []string{"echo", "a", "b", "c"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.4: exit status propagated from command.
			Name:     "exit_status_propagated",
			Args:     []string{"sh", "-c", "exit 42"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 42,
		},
		{
			// R2.2: exit 127 when command not found.
			Name:      "command_not_found",
			Args:      []string{"nonexistent_command_xyz_12345"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			// R2.2: exit 125 when no operand given.
			Name:      "missing_operand",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeTryPath},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
