// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?date\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("date"))
}

func setupBinaries(t *testing.T) (string, string) {
	t.Helper()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdate")
	if err != nil {
		t.Skip("reference binary gdate not found")
	}
	return goBin, refBin
}

func TestDiffDefault(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "default_epoch0", Args: []string{"-d", "@0"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "default_epoch", Args: []string{"-d", "@1700000000"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFormatStrings(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "format_ymd", Args: []string{"-d", "@1700000000", "+%Y-%m-%d"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "format_hms", Args: []string{"-d", "@1700000000", "+%H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "format_combined", Args: []string{"-d", "@1700000000", "+%Y/%m/%d %H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "format_literal", Args: []string{"-d", "@0", "+hello"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "format_percent", Args: []string{"-d", "@0", "+100%%"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "format_empty", Args: []string{"-d", "@0", "+"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffSpecifiers(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "spec_Y", Args: []string{"-d", "@1700000000", "+%Y"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_m", Args: []string{"-d", "@1700000000", "+%m"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_d", Args: []string{"-d", "@1700000000", "+%d"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_H", Args: []string{"-d", "@1700000000", "+%H"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_M", Args: []string{"-d", "@1700000000", "+%M"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_S", Args: []string{"-d", "@1700000000", "+%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_Z", Args: []string{"-d", "@1700000000", "+%Z"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_s", Args: []string{"-d", "@1700000000", "+%s"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_N", Args: []string{"-d", "@1700000000", "+%N"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_A", Args: []string{"-d", "@1700000000", "+%A"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_B", Args: []string{"-d", "@1700000000", "+%B"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_j", Args: []string{"-d", "@1700000000", "+%j"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_u", Args: []string{"-d", "@1700000000", "+%u"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_w", Args: []string{"-d", "@1700000000", "+%w"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffGNUExtensions(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "spec_P_am", Args: []string{"-d", "@0", "+%P"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "spec_P_pm", Args: []string{"-d", "@43200", "+%P"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_dash_d", Args: []string{"-d", "@0", "+%-d"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_space_d", Args: []string{"-d", "@0", "+%_d"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_zero_d", Args: []string{"-d", "@0", "+%0d"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_dash_m", Args: []string{"-d", "@0", "+%-m"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_space_m", Args: []string{"-d", "@0", "+%_m"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_dash_H", Args: []string{"-d", "@0", "+%-H"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_space_H", Args: []string{"-d", "@0", "+%_H"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_dash_j", Args: []string{"-d", "@0", "+%-j"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "pad_space_j", Args: []string{"-d", "@0", "+%_j"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffDateParsing(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "date_epoch_zero", Args: []string{"-d", "@0", "+%Y-%m-%d %H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "date_epoch_positive", Args: []string{"-d", "@1234567890", "+%Y-%m-%d %H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "date_long_flag", Args: []string{"--date=@1700000000", "+%Y-%m-%d %H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "date_iso_datetime", Args: []string{"-d", "2024-01-15 10:30:00", "+%Y-%m-%d %H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "date_iso_datetime_t", Args: []string{"-d", "2024-01-15T10:30:00", "+%Y-%m-%d %H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "date_iso_date_only", Args: []string{"-d", "2024-01-15", "+%Y-%m-%d"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "date_invalid", Args: []string{"-d", "invalid"}, ExitCode: 1, Env: []string{"LC_ALL=C", "TZ=UTC"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
		{Name: "date_epoch_with_format", Args: []string{"-d", "@0", "+%s"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffUTCMode(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "utc_short", Args: []string{"-u", "-d", "@1700000000", "+%Y-%m-%d %H:%M:%S %Z"}, Env: []string{"LC_ALL=C", "TZ=America/New_York"}},
		{Name: "utc_long", Args: []string{"--utc", "-d", "@1700000000", "+%Y-%m-%d %H:%M:%S %Z"}, Env: []string{"LC_ALL=C", "TZ=America/New_York"}},
		{Name: "universal_long", Args: []string{"--universal", "-d", "@1700000000", "+%Y-%m-%d %H:%M:%S %Z"}, Env: []string{"LC_ALL=C", "TZ=America/New_York"}},
		{Name: "utc_epoch_seconds", Args: []string{"-u", "-d", "@1700000000", "+%s"}, Env: []string{"LC_ALL=C", "TZ=America/New_York"}},
		{Name: "utc_default_format", Args: []string{"-u", "-d", "@1700000000"}, Env: []string{"LC_ALL=C", "TZ=America/New_York"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffExitCodes(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "exit0_default", Args: []string{"-d", "@0"}, ExitCode: 0, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "exit0_format", Args: []string{"-d", "@0", "+%Y-%m-%d"}, ExitCode: 0, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "exit0_utc", Args: []string{"-u", "-d", "@0", "+%s"}, ExitCode: 0, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "exit1_invalid_date", Args: []string{"-d", "not-a-date"}, ExitCode: 1, Env: []string{"LC_ALL=C", "TZ=UTC"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
		{Name: "exit1_invalid_epoch", Args: []string{"-d", "@notanumber"}, ExitCode: 1, Env: []string{"LC_ALL=C", "TZ=UTC"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
		{Name: "exit1_missing_ref", Args: []string{"-r", "/nonexistent_file_for_date_test"}, ExitCode: 1, Env: []string{"LC_ALL=C", "TZ=UTC"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffReferenceFile(t *testing.T) {
	goBin, refBin := setupBinaries(t)
	tests := []testutils.DiffTest{
		{Name: "ref_short", Args: []string{"-r", "/dev/null", "+%Y"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "ref_long", Args: []string{"--reference=/dev/null", "+%Y"}, Env: []string{"LC_ALL=C", "TZ=UTC"}},
		{Name: "ref_with_utc", Args: []string{"-u", "-r", "/dev/null", "+%Y-%m-%d %H:%M:%S"}, Env: []string{"LC_ALL=C", "TZ=America/New_York"}},
		{Name: "ref_missing_file", Args: []string{"-r", "/nonexistent_file_for_date_test", "+%Y"}, ExitCode: 1, Env: []string{"LC_ALL=C", "TZ=UTC"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
		{Name: "ref_missing_long", Args: []string{"--reference=/nonexistent_file_for_date_test", "+%Y"}, ExitCode: 1, Env: []string{"LC_ALL=C", "TZ=UTC"}, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
