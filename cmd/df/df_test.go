// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/df.
// Implements prd106-df R3.1–R3.7, R4.1–R4.3 test coverage.

package main

import (
	"bytes"
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

// stderrNormalizer normalizes stderr for comparison: strips binary path
// prefixes, removes GNU "Try ..." hints, and lowercases for portability.
var stderrNormalizer = testutils.NormalizeFunc(func(b []byte) []byte {
	// Replace binary path prefix with "df:".
	re := regexp.MustCompile(`(?m)^[^\s:]+:`)
	result := re.ReplaceAll(b, []byte("df:"))
	// Remove "Try '...' for more information." lines.
	tryRe := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n?`)
	result = tryRe.ReplaceAll(result, nil)
	// Lowercase for portable comparison of OS error strings.
	return bytes.ToLower(result)
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
		// R3.5: -t TYPE includes only matching filesystem types.
		{
			Name:      "type_filter_apfs_with_root",
			Args:      []string{"-t", "apfs", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		{
			Name:      "type_filter_long_flag_with_root",
			Args:      []string{"--type=apfs", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// R3.6: -x TYPE excludes matching filesystem types.
		{
			Name:      "exclude_type_devfs_with_root",
			Args:      []string{"-x", "devfs", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		{
			Name:      "exclude_type_long_flag_with_root",
			Args:      []string{"--exclude-type=devfs", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// R3.7: --output column selection.
		{
			Name:      "output_source_size_target",
			Args:      []string{"--output=source,size,used,avail,pcent,target", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		{
			Name:      "output_all_fields",
			Args:      []string{"--output", "/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// R4.1: exit 0 on successful filesystem report.
		{
			Name:      "exit_zero_on_success",
			Args:      []string{"/"},
			Normalize: []testutils.NormalizeFunc{numNormalizer},
		},
		// R4.2: exit 1 on inaccessible FILE argument with diagnostic to stderr.
		{
			Name:      "exit_one_nonexistent_file",
			Args:      []string{"/nonexistent_path_df_test"},
			Normalize: []testutils.NormalizeFunc{numNormalizer, stderrNormalizer},
		},
		// R4.2: exit 1 on invalid option.
		{
			Name:      "exit_one_invalid_option",
			Args:      []string{"--invalid-option-df-test"},
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
