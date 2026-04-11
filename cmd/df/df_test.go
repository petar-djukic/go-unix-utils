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

// fsUsageNormalizer rounds numbers with 4+ digits to the nearest 100000
// to tolerate filesystem usage changes between binary invocations.
// Preserves column width by zero-padding the rounded value.
var fsUsageNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return digitPattern.ReplaceAllFunc(b, roundNumber)
}

// roundNumber rounds a numeric byte sequence to 2 significant figures
// to tolerate filesystem usage fluctuations between binary invocations.
func roundNumber(m []byte) []byte {
	n, err := strconv.ParseInt(string(m), 10, 64)
	if err != nil {
		return m
	}
	rounded := roundToSigFigs(n, 2)
	return fmt.Appendf(nil, "%0*d", len(m), rounded)
}

// roundToSigFigs rounds n to the specified number of significant figures.
func roundToSigFigs(n int64, figs int) int64 {
	if n <= 0 {
		return n
	}
	mag := int64(1)
	temp := n
	for temp >= 10 {
		temp /= 10
		mag *= 10
	}
	divisor := mag
	for range figs - 1 {
		divisor /= 10
	}
	if divisor == 0 {
		return n
	}
	return (n / divisor) * divisor
}

// humanSizePattern matches human-readable size values like "1.5G", "234M", "1.2kB".
var humanSizePattern = regexp.MustCompile(`\b\d+(\.\d)?\s*(K|M|G|T|P|E|kB|MB|GB|TB)\b`)

// humanSizeNormalizer replaces human-readable size values with "0X"
// to tolerate minor filesystem usage changes between binary invocations.
var humanSizeNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return humanSizePattern.ReplaceAllFunc(b, func(m []byte) []byte {
		return []byte("0X")
	})
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

// TestDiffHumanReadable verifies human-readable output matches gdf -h.
// R2.1: binary unit display (K, M, G, T).
func TestDiffHumanReadable(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{humanSizeNormalizer}

	tests := []testutils.DiffTest{
		{
			Name:      "human_readable_default",
			Args:      []string{"-h"},
			Normalize: norms,
		},
		{
			Name:      "human_readable_long_flag",
			Args:      []string{"--human-readable"},
			Normalize: norms,
		},
		{
			Name:      "human_readable_root",
			Args:      []string{"-h", "/"},
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSI verifies SI output matches gdf -H.
// R2.2: SI unit display (kB, MB, GB, TB).
func TestDiffSI(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{humanSizeNormalizer}

	tests := []testutils.DiffTest{
		{
			Name:      "si_default",
			Args:      []string{"-H"},
			Normalize: norms,
		},
		{
			Name:      "si_long_flag",
			Args:      []string{"--si"},
			Normalize: norms,
		},
		{
			Name:      "si_root",
			Args:      []string{"-H", "/"},
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffPrintType verifies filesystem type display matches gdf -T.
// R3.1: Type column display.
func TestDiffPrintType(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{fsUsageNormalizer}

	tests := []testutils.DiffTest{
		{
			Name:      "print_type_default",
			Args:      []string{"-T"},
			Normalize: norms,
		},
		{
			Name:      "print_type_long_flag",
			Args:      []string{"--print-type"},
			Normalize: norms,
		},
		{
			Name:      "print_type_root",
			Args:      []string{"-T", "/"},
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCombined verifies combined flag usage matches gdf.
// R2.1, R2.3, R3.1: -T with -h and last-flag-wins behavior.
func TestDiffCombined(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{humanSizeNormalizer}

	tests := []testutils.DiffTest{
		{
			// R3.1 + R2.1: Type column with human-readable sizes.
			Name:      "print_type_human_readable",
			Args:      []string{"-T", "-h"},
			Normalize: norms,
		},
		{
			// R3.1 + R2.2: Type column with SI sizes.
			Name:      "print_type_si",
			Args:      []string{"-T", "-H"},
			Normalize: norms,
		},
		{
			// R2.3: last flag wins — -h then -H gives SI output.
			Name:      "last_wins_h_then_H",
			Args:      []string{"-h", "-H"},
			Normalize: norms,
		},
		{
			// R2.3: last flag wins — -H then -h gives human-readable output.
			Name:      "last_wins_H_then_h",
			Args:      []string{"-H", "-h"},
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

// TestDiffTypeFilter verifies filesystem type filtering matches gdf.
// R3.5: -t TYPE inclusion. R3.6: -x TYPE exclusion.
func TestDiffTypeFilter(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{fsUsageNormalizer}

	tests := []testutils.DiffTest{
		{
			// R3.5: include only apfs filesystems.
			Name:      "type_include_apfs",
			Args:      []string{"-t", "apfs"},
			Normalize: norms,
		},
		{
			// R3.5: long flag form.
			Name:      "type_include_long_flag",
			Args:      []string{"--type=apfs"},
			Normalize: norms,
		},
		{
			// R3.6: exclude devfs filesystems.
			Name:      "type_exclude_devfs",
			Args:      []string{"-x", "devfs"},
			Normalize: norms,
		},
		{
			// R3.6: long flag form.
			Name:      "type_exclude_long_flag",
			Args:      []string{"--exclude-type=devfs"},
			Normalize: norms,
		},
		{
			// R3.5 + R3.1: type filter with type column display.
			Name:      "type_include_with_print_type",
			Args:      []string{"-t", "apfs", "-T"},
			Normalize: norms,
		},
		{
			// R3.5: include a nonexistent type shows no entries.
			Name:     "type_include_nonexistent",
			Args:     []string{"-t", "nonexistent_type_xyz"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				fsUsageNormalizer,
				stderrProgramNameNormalizer,
			},
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

// TestIncompatibleFlags verifies that mutually exclusive flag combinations
// are rejected with an error.
// R3.7: --output is incompatible with -i, -T, and -h/-H.
func TestIncompatibleFlags(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"output_with_inodes", []string{"--output", "-i"}, "mutually exclusive"},
		{"output_with_print_type", []string{"--output", "-T"}, "mutually exclusive"},
		{"output_with_human", []string{"--output", "-h"}, "mutually exclusive"},
		{"output_with_si", []string{"--output", "-H"}, "mutually exclusive"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin, tc.args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err == nil {
				t.Fatalf("expected non-zero exit for %v", tc.args)
			}
			if !bytes.Contains(stderr.Bytes(), []byte(tc.want)) {
				t.Errorf("expected %q in stderr, got: %s", tc.want, stderr.String())
			}
		})
	}
}
