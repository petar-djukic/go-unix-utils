// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/realpath against grealpath (GNU coreutils).
//
// Covers prd049-realpath R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
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

		// R1.5: -s does not resolve symlinks, only cleans path
		{
			Name:     "R1.5_strip_absolute",
			Args:     []string{"-s", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.5: -s cleans .. components with -m (no existence check)
		{
			Name:     "R1.5_strip_dotdot",
			Args:     []string{"-s", "-m", "/tmp/foo/.."},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.5: --strip long form
		{
			Name:     "R1.5_strip_long",
			Args:     []string{"--strip", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.5: --no-symlinks long form
		{
			Name:     "R1.5_no_symlinks_long",
			Args:     []string{"--no-symlinks", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.5: -s with -m allows nonexistent paths
		{
			Name:     "R1.5_strip_missing",
			Args:     []string{"-s", "-m", "/nonexistent_xyz_99999/foo/bar"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.5: -s with -e requires existence
		{
			Name:      "R1.5_strip_strict_nonexistent",
			Args:      []string{"-s", "-e", "/nonexistent_xyz_99999"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.5: -s with -e on existing path
		{
			Name:     "R1.5_strip_strict_existing",
			Args:     []string{"-s", "-e", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},

		// R2.1: --relative-to prints path relative to DIR
		{
			Name:     "R2.1_relative_to",
			Args:     []string{"--relative-to=/", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: --relative-to with same directory
		{
			Name:     "R2.1_relative_to_same",
			Args:     []string{"--relative-to=/tmp", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: --relative-to with sibling
		{
			Name:     "R2.1_relative_to_sibling",
			Args:     []string{"-m", "--relative-to=/usr/local", "/usr/bin"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},

		// R2.2: --relative-base prints relative when path starts with base
		{
			Name:     "R2.2_relative_base_inside",
			Args:     []string{"-m", "--relative-base=/usr", "/usr/local/bin"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: --relative-base prints absolute when path does not start with base
		{
			Name:     "R2.2_relative_base_outside",
			Args:     []string{"-m", "--relative-base=/usr", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},

		// R2.3: both --relative-to and --relative-base
		{
			Name:     "R2.3_both_inside_base",
			Args:     []string{"-m", "--relative-to=/usr", "--relative-base=/usr", "/usr/local/bin"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: both flags, path outside base — absolute output
		{
			Name:     "R2.3_both_outside_base",
			Args:     []string{"-m", "--relative-to=/usr", "--relative-base=/usr", "/tmp"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
