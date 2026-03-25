// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/df.
// Implements prd106-df R3.1–R3.4 test coverage.

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// numNormalizer replaces sequences of digits with "N" to handle timing
// differences in block/inode counts between the two binary invocations.
var numNormalizer = testutils.NormalizeFunc(func(b []byte) []byte {
	re := regexp.MustCompile(`[0-9]+`)
	return re.ReplaceAll(b, []byte("N"))
})

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R3.1: -T prints filesystem type column.
		{
			Name:      "print_type_with_root",
			Args:      []string{"-T", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		{
			Name:      "print_type_long_flag_with_root",
			Args:      []string{"--print-type", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// R3.2: -i displays inode information.
		{
			Name: "inodes_with_root",
			Args: []string{"-i", "/"},
		},
		{
			Name:      "inodes_long_flag_with_root",
			Args:      []string{"--inodes", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// R3.3: -a includes pseudo-filesystems (tested with path to verify
		// flag acceptance; path-mode output is identical since / is non-pseudo).
		{
			Name:      "all_with_root",
			Args:      []string{"-a", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		{
			Name:      "all_long_flag_with_root",
			Args:      []string{"--all", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// R3.4: -l restricts to local filesystems (tested with path to verify
		// flag acceptance; / is local so output matches non-filtered).
		{
			Name:      "local_with_root",
			Args:      []string{"-l", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		{
			Name:      "local_long_flag_with_root",
			Args:      []string{"--local", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// Combined flags with path.
		{
			Name:      "type_and_inodes_with_root",
			Args:      []string{"-T", "-i", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		{
			Name:      "all_and_local_with_root",
			Args:      []string{"-a", "-l", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
