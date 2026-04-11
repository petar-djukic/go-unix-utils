// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgramNameNormalizer replaces "gdf:" with "df:" in stderr
// so the differential test ignores the binary name difference.
var stderrProgramNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gdf:"), []byte("df:"))
}

// digitPattern matches sequences of 4 or more digits.
var digitPattern = regexp.MustCompile(`\b\d{4,}\b`)

// fsUsageNormalizer rounds numbers with 4+ digits to the nearest 10000
// to tolerate filesystem usage changes between binary invocations.
// Preserves column width by zero-padding the rounded value.
var fsUsageNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return digitPattern.ReplaceAllFunc(b, roundNumber)
}

// roundNumber rounds a numeric byte sequence to the nearest 100000
// to tolerate filesystem usage fluctuations between binary invocations.
func roundNumber(m []byte) []byte {
	n, err := strconv.ParseInt(string(m), 10, 64)
	if err != nil {
		return m
	}
	rounded := (n / 100000) * 100000
	return []byte(fmt.Sprintf("%0*d", len(m), rounded))
}

// TestDiff runs differential tests comparing the Go df binary against
// the GNU reference binary (gdf) for default output.
// R1.1-R1.5: core filesystem data collection and output.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{fsUsageNormalizer}

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2: default output with correct columns and alignment.
			Name:      "default_no_args",
			Args:      []string{},
			Normalize: norms,
		},
		{
			// R1.4: FILE argument reports containing filesystem.
			Name:      "root_filesystem",
			Args:      []string{"/"},
			Normalize: norms,
		},
		{
			// R1.4: multiple FILE arguments on different filesystems.
			Name:      "multiple_paths",
			Args:      []string{"/", "/dev"},
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInodes verifies inode display mode matches gdf -i.
// R3.2: inode column display.
func TestDiffInodes(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{fsUsageNormalizer}

	tests := []testutils.DiffTest{
		{
			// R3.2: inode display for all mounted filesystems.
			Name:      "inodes_default",
			Args:      []string{"-i"},
			Normalize: norms,
		},
		{
			// R3.2: inode display for a specific filesystem.
			Name:      "inodes_root",
			Args:      []string{"-i", "/"},
			Normalize: norms,
		},
		{
			// R3.2: long flag form.
			Name:      "inodes_long_flag",
			Args:      []string{"--inodes"},
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutput verifies --output column selection matches gdf.
// R3.7: output field selection.
func TestDiffOutput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{fsUsageNormalizer}

	tests := []testutils.DiffTest{
		{
			// R3.7: select only source and target columns.
			Name:      "output_source_target",
			Args:      []string{"--output=source,target"},
			Normalize: norms,
		},
		{
			// R3.7: select block-usage columns matching default layout.
			Name:      "output_block_fields",
			Args:      []string{"--output=source,size,used,avail,pcent,target"},
			Normalize: norms,
		},
		{
			// R3.7: select inode fields via --output.
			Name:      "output_inode_fields",
			Args:      []string{"--output=source,itotal,iused,iavail,ipcent,target"},
			Normalize: norms,
		},
		{
			// R3.7: --output with file argument.
			Name:      "output_with_file_arg",
			Args:      []string{"--output=source,target,file", "/"},
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrors verifies exit code and stderr for error conditions.
// R4.1: exit 0 on success. R4.2: exit 1 with diagnostic for errors.
func TestDiffErrors(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R4.2: nonexistent file produces exit 1 and diagnostic.
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent/path/xyz_df_test"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgramNameNormalizer},
		},
		{
			// R1.4, R4.2: valid and invalid args mixed; exit 1 but
			// still output valid filesystem entry.
			Name: "mixed_valid_and_invalid",
			Args: []string{
				"/",
				"/nonexistent/path/xyz_df_test",
			},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				stderrProgramNameNormalizer,
				fsUsageNormalizer,
			},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestIncompatibleFlags verifies that -i and --output cannot be combined.
// R3.7: --output is incompatible with -i.
func TestIncompatibleFlags(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--output", "-i")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for -i with --output")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("mutually exclusive")) {
		t.Errorf("expected 'mutually exclusive' in stderr, got: %s", stderr.String())
	}
}
