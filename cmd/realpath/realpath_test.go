// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/realpath against grealpath (GNU coreutils).
//
// Covers prd049-realpath R1.1, R1.2, R1.3, R1.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where GNU includes the full binary path.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skip("reference binary grealpath not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: resolve absolute path with no symlinks
		{
			Name:     "R1.1_absolute_existing",
			Args:     []string{"/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: resolve relative path "."
		{
			Name:     "R1.1_dot",
			Args:     []string{"."},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: resolve relative path ".."
		{
			Name:     "R1.1_dotdot",
			Args:     []string{".."},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: resolve root
		{
			Name:     "R1.1_root",
			Args:     []string{"/"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: multiple existing paths
		{
			Name:     "R1.1_multiple_existing",
			Args:     []string{"/tmp", "/"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: default mode allows last component to be missing
		{
			Name:     "R1.1_last_missing_ok",
			Args:     []string{"/tmp/nonexistent_xyz_99999"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: default mode errors when parent directory is missing
		{
			Name:      "R1.2_parent_missing",
			Args:      []string{"/nonexistent_xyz_99999/child/grandchild"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.2: mixed existing and failing paths
		{
			Name:      "R1.2_mixed_paths",
			Args:      []string{"/tmp", "/nonexistent_xyz_99999/child/grandchild"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: -e with existing path succeeds
		{
			Name:     "R1.3_e_existing",
			Args:     []string{"-e", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: -e with nonexistent path fails
		{
			Name:      "R1.3_e_nonexistent",
			Args:      []string{"-e", "/nonexistent_xyz_99999"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: --canonicalize-existing long form
		{
			Name:     "R1.3_canonicalize_existing_long",
			Args:     []string{"--canonicalize-existing", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: -e with missing last component fails (stricter than default)
		{
			Name:      "R1.3_e_last_missing",
			Args:      []string{"-e", "/tmp/nonexistent_xyz_99999"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: -m with nonexistent path succeeds
		{
			Name:     "R1.4_m_nonexistent",
			Args:     []string{"-m", "/nonexistent_xyz_99999/foo/bar"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.4: -m with existing path
		{
			Name:     "R1.4_m_existing",
			Args:     []string{"-m", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.4: --canonicalize-missing long form
		{
			Name:     "R1.4_canonicalize_missing_long",
			Args:     []string{"--canonicalize-missing", "/nonexistent_xyz_99999"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// No operand — error exit 1
		{
			Name:      "no_args_error",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// Unknown flag — error exit 1
		{
			Name:      "unknown_flag_error",
			Args:      []string{"--bogus-flag"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
