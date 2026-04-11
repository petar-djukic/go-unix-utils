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

// roundNumber rounds a numeric byte sequence to the nearest 10000.
func roundNumber(m []byte) []byte {
	n, err := strconv.ParseInt(string(m), 10, 64)
	if err != nil {
		return m
	}
	rounded := (n / 10000) * 10000
	return []byte(fmt.Sprintf("%0*d", len(m), rounded))
}

// TestDiff runs differential tests comparing the Go df binary against
// the GNU reference binary (gdf) for default output.
// R1.1-R1.4: core filesystem data collection and output.
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
