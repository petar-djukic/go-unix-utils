// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd060-date R1.1–R1.4, R2.1–R2.4, R3.1–R3.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff compares the Go date binary against gdate (GNU coreutils).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdate")
	if err != nil {
		t.Skipf("reference binary gdate not in PATH: %v", err)
	}

	// R3.2: create a reference file with a known modification time.
	refDir := t.TempDir()
	refFile := filepath.Join(refDir, "reffile")
	writeRefFile(t, refFile)

	tests := buildR1R2Tests()
	tests = append(tests, buildR3Tests(refFile)...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildR1R2Tests returns differential test cases for R1 and R2.
func buildR1R2Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			// R1.1: default format with no arguments.
			Name:      "default_format",
			Normalize: []testutils.NormalizeFunc{normalizeTimePortion},
		},
		{
			// R1.2: +FORMAT argument.
			Name: "format_year",
			Args: []string{"+%Y"},
		},
		{
			// R1.2, R1.3: multiple conversion specs in one format.
			Name: "format_date_components",
			Args: []string{"+%Y-%m-%d"},
		},
		{
			// R1.3: weekday and month names.
			Name: "format_weekday_month_names",
			Args: []string{"+%A %B"},
		},
		{
			// R1.3: day of year.
			Name: "format_day_of_year",
			Args: []string{"+%j"},
		},
		{
			// R1.3: day of week (1=Monday, 7=Sunday).
			Name: "format_day_of_week_u",
			Args: []string{"+%u"},
		},
		{
			// R1.3: day of week (0=Sunday, 6=Saturday).
			Name: "format_day_of_week_w",
			Args: []string{"+%w"},
		},
		{
			// R1.3: timezone name.
			Name: "format_timezone_name",
			Args: []string{"+%Z"},
		},
		{
			// R1.3: literal percent.
			Name: "format_literal_percent",
			Args: []string{"+%%"},
		},
		{
			// R1.4: lowercase am/pm (GNU extension).
			Name: "format_lowercase_ampm",
			Args: []string{"+%P"},
		},
		{
			// R1.4: no-padding modifier.
			Name: "format_no_padding",
			Args: []string{"+%-m %-d"},
		},
		{
			// R1.4: space-padding modifier.
			Name: "format_space_padding",
			Args: []string{"+%_m"},
		},
		{
			// R1.2: composite format %F.
			Name: "format_composite_F",
			Args: []string{"+%F"},
		},
		{
			// R2.1, R2.2: epoch zero via -d flag with format.
			Name: "date_d_epoch_zero",
			Args: []string{"-d", "@0", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.2: large epoch timestamp.
			Name: "date_d_epoch_large",
			Args: []string{"-d", "@1234567890", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.1: --date= long form with epoch.
			Name: "date_long_flag_epoch",
			Args: []string{"--date=@1700000000", "+%s"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.1: --date with space separator.
			Name: "date_long_flag_space_epoch",
			Args: []string{"--date", "@1700000000", "+%Y-%m-%d"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.2: epoch with default format.
			Name: "date_d_epoch_default_format",
			Args: []string{"-d", "@0"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.2: negative epoch (before 1970).
			Name: "date_d_epoch_negative",
			Args: []string{"-d", "@-86400", "+%Y-%m-%d"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.3: ISO 8601 date only.
			Name: "date_d_iso_date",
			Args: []string{"-d", "2024-01-15", "+%Y-%m-%d"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.3: ISO 8601 date-time with space.
			Name: "date_d_iso_datetime_space",
			Args: []string{"-d", "2024-01-15 10:30:00", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.3: ISO 8601 date-time with T separator.
			Name: "date_d_iso_datetime_T",
			Args: []string{"-d", "2024-06-15T14:30:00", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.1: -d with concatenated value.
			Name: "date_d_concat_epoch",
			Args: []string{"-d@0", "+%Y-%m-%d"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R2.4: invalid date string produces exit code 1.
			Name:      "date_d_invalid",
			Args:      []string{"-d", "not-a-date"},
			ExitCode:  1,
			Env:       []string{"TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{normalizeProgName},
		},
		{
			// R2.1: empty date string treated as midnight today by GNU date.
			Name: "date_d_empty_string",
			Args: []string{"-d", "", "+%H:%M:%S"},
			Env:  []string{"TZ=UTC"},
		},
	}
}

// buildR3Tests returns differential test cases for R3 requirements.
func buildR3Tests(refFile string) []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			// R3.1: -u flag displays UTC.
			Name: "utc_flag_u",
			Args: []string{"-u", "-d", "@1700000000", "+%Y-%m-%d %H:%M:%S %Z"},
		},
		{
			// R3.1: --utc long flag displays UTC.
			Name: "utc_flag_long",
			Args: []string{"--utc", "-d", "@1700000000", "+%Z"},
		},
		{
			// R3.1: --universal long flag displays UTC.
			Name: "utc_flag_universal",
			Args: []string{"--universal", "-d", "@1700000000", "+%Z"},
		},
		{
			// R3.1: -u combined with format string.
			Name: "utc_with_epoch_seconds",
			Args: []string{"-u", "-d", "@1700000000", "+%s"},
		},
		{
			// R3.2: -r flag displays file modification time.
			Name: "ref_file_short_flag",
			Args: []string{"-r", refFile, "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R3.2: --reference= long form with file path.
			Name: "ref_file_long_flag_equals",
			Args: []string{"--reference=" + refFile, "+%Y-%m-%d"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R3.2: --reference with space separator.
			Name: "ref_file_long_flag_space",
			Args: []string{"--reference", refFile, "+%Y-%m-%d"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R3.2: -r with concatenated file path.
			Name: "ref_file_concat",
			Args: []string{"-r" + refFile, "+%Y-%m-%d"},
			Env:  []string{"TZ=UTC"},
		},
		{
			// R3.2, R3.1: -r combined with -u for UTC output.
			Name: "ref_file_with_utc",
			Args: []string{"-u", "-r", refFile, "+%Y-%m-%d %H:%M:%S %Z"},
		},
		{
			// R3.3: missing reference file exits 1.
			Name:      "ref_file_missing",
			Args:      []string{"-r", "/nonexistent/path/to/file"},
			ExitCode:  1,
			Env:       []string{"TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{normalizeProgName, normalizeErrCase},
		},
	}
}

// writeRefFile creates a test file and sets its modification time to a
// known epoch for deterministic differential testing.
func writeRefFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test\n"), 0o644); err != nil {
		t.Fatalf("creating reference file: %v", err)
	}
	// Set mtime to 2024-06-15 12:00:00 UTC for deterministic output.
	mtime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("setting reference file mtime: %v", err)
	}
}

// TestStrftime verifies strftime formatting with a fixed time for
// deterministic assertions on time-dependent specifiers.
func TestStrftime(t *testing.T) {
	t.Parallel()
	// 2024-03-05 14:07:09.123456789 UTC is a Tuesday.
	fixed := time.Date(2024, 3, 5, 14, 7, 9, 123456789, time.UTC)
	tests := []struct {
		name   string
		format string
		want   string
	}{
		{"epoch_seconds", "%s", "1709647629"},
		{"nanoseconds", "%N", "123456789"},
		{"year", "%Y", "2024"},
		{"month_zero_pad", "%m", "03"},
		{"day_zero_pad", "%d", "05"},
		{"hour_24", "%H", "14"},
		{"minute", "%M", "07"},
		{"second", "%S", "09"},
		{"day_space_pad", "%e", " 5"},
		{"hour_12", "%I", "02"},
		{"hour_24_space", "%k", "14"},
		{"hour_12_space", "%l", " 2"},
		{"day_of_year", "%j", "065"},
		{"weekday_mon_1_7", "%u", "2"},
		{"weekday_sun_0_6", "%w", "2"},
		{"am_pm_upper", "%p", "PM"},
		{"am_pm_lower", "%P", "pm"},
		{"no_pad_month", "%-m", "3"},
		{"no_pad_day", "%-d", "5"},
		{"space_pad_day", "%_d", " 5"},
		{"zero_pad_day", "%0d", "05"},
		{"literal_percent", "%%", "%"},
		{"newline", "%n", "\n"},
		{"tab", "%t", "\t"},
		{"composite_F", "%F", "2024-03-05"},
		{"composite_T", "%T", "14:07:09"},
		{"composite_R", "%R", "14:07"},
		{"composite_D", "%D", "03/05/24"},
		{"timezone_utc", "%Z", "UTC"},
		{"tz_offset_utc", "%z", "+0000"},
		{"weekday_full", "%A", "Tuesday"},
		{"weekday_abbrev", "%a", "Tue"},
		{"month_full", "%B", "March"},
		{"month_abbrev", "%b", "Mar"},
		{"century", "%C", "20"},
		{"year_short", "%y", "24"},
		{"mixed_text_and_specs", "Today is %A, %B %d", "Today is Tuesday, March 05"},
		{"empty_format", "", ""},
		{"trailing_percent", "abc%", "abc%"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := strftime(fixed, tc.format)
			if got != tc.want {
				t.Errorf("strftime(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}

// TestParseDate verifies date string parsing for R2.2 and R2.3.
func TestParseDate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantSec int64 // expected Unix timestamp (only checked if !wantErr)
	}{
		{"epoch_zero", "@0", false, 0},
		{"epoch_positive", "@1234567890", false, 1234567890},
		{"epoch_negative", "@-86400", false, -86400},
		{"epoch_invalid", "@notanumber", true, 0},
		{"iso_date", "@1705276800", false, 1705276800},
		{"empty_string", "", true, 0},
		{"garbage", "not-a-date", true, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDate(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseDate(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDate(%q) error: %v", tc.input, err)
			}
			if got.Unix() != tc.wantSec {
				t.Errorf("parseDate(%q).Unix() = %d, want %d", tc.input, got.Unix(), tc.wantSec)
			}
		})
	}
}

// TestRun verifies the run function for R2.1, R2.4, R3.1–R3.3 integration.
func TestRun(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create a reference file for -r tests.
	refDir := t.TempDir()
	refFile := filepath.Join(refDir, "testfile")
	writeRefFile(t, refFile)

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"d_epoch", []string{"-d", "@0", "+%Y"}, 0},
		{"date_equals_epoch", []string{"--date=@0", "+%Y"}, 0},
		{"d_invalid", []string{"-d", "garbage"}, 1},
		{"d_missing_arg", []string{"-d"}, 1},
		// R3.1: UTC mode.
		{"utc_flag", []string{"-u", "-d", "@0", "+%Z"}, 0},
		{"utc_long", []string{"--utc", "-d", "@0", "+%Z"}, 0},
		{"universal_long", []string{"--universal", "-d", "@0", "+%Z"}, 0},
		// R3.2: reference file.
		{"ref_file", []string{"-r", refFile, "+%Y"}, 0},
		{"ref_file_long", []string{"--reference=" + refFile, "+%Y"}, 0},
		// R3.3: missing reference file.
		{"ref_file_missing", []string{"-r", "/nonexistent/file"}, 1},
		{"ref_missing_arg", []string{"-r"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr strings.Builder
			code := run(tc.args, &stdout, &stderr, now)
			if code != tc.wantCode {
				t.Errorf("run(%v) = %d, want %d; stderr=%q",
					tc.args, code, tc.wantCode, stderr.String())
			}
		})
	}
}

// TestRunUTCOutput verifies that -u flag actually produces UTC output.
func TestRunUTCOutput(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var stdout, stderr strings.Builder
	code := run([]string{"-u", "-d", "@0", "+%Z"}, &stdout, &stderr, now)
	if code != 0 {
		t.Fatalf("run returned %d; stderr=%q", code, stderr.String())
	}
	got := strings.TrimSpace(stdout.String())
	if got != "UTC" {
		t.Errorf("got timezone %q, want %q", got, "UTC")
	}
}

// TestRunRefFileOutput verifies -r returns the file's modification time.
func TestRunRefFileOutput(t *testing.T) {
	t.Parallel()
	refDir := t.TempDir()
	refFile := filepath.Join(refDir, "testfile")
	writeRefFile(t, refFile)

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var stdout, stderr strings.Builder
	code := run([]string{"-r", refFile, "+%Y-%m-%d"}, &stdout, &stderr, now)
	if code != 0 {
		t.Fatalf("run returned %d; stderr=%q", code, stderr.String())
	}
	got := strings.TrimSpace(stdout.String())
	if got != "2024-06-15" {
		t.Errorf("got %q, want %q", got, "2024-06-15")
	}
}

// normalizeTimePortion replaces HH:MM:SS with a placeholder to avoid
// flakiness when the two binaries execute across a second boundary.
func normalizeTimePortion(data []byte) []byte {
	re := regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)
	return re.ReplaceAll(data, []byte("HH:MM:SS"))
}

// normalizeErrCase lowercases stderr to handle casing differences
// between Go ("no such file or directory") and GNU ("No such file or directory").
func normalizeErrCase(data []byte) []byte {
	return []byte(strings.ToLower(string(data)))
}

// normalizeProgName replaces the program name prefix to handle
// differences between "date" and "gdate" in error messages.
// R2.4: allows error message comparison across binary names.
func normalizeProgName(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^g?date:`)
	return re.ReplaceAll(data, []byte("DATE:"))
}
