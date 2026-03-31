// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/date — differential tests against gdate.
// Covers prd060-date R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdate")
	if err != nil {
		t.Skip("reference binary gdate not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: default format with fixed epoch
		{
			Name:     "default_format_epoch",
			Args:     []string{"-d", "@0"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.2: custom +FORMAT
		{
			Name:     "custom_format_ymd_hms",
			Args:     []string{"-d", "@0", "+%Y-%m-%d %H:%M:%S"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %s seconds since epoch
		{
			Name:     "epoch_seconds",
			Args:     []string{"-u", "-d", "@1700000000", "+%s"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: %A full weekday name
		{
			Name:     "weekday_name",
			Args:     []string{"-d", "@0", "+%A"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %B full month name
		{
			Name:     "month_name",
			Args:     []string{"-d", "@0", "+%B"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %j day of year
		{
			Name:     "day_of_year",
			Args:     []string{"-d", "@0", "+%j"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %u day of week (1-7, Monday=1)
		{
			Name:     "day_of_week_u",
			Args:     []string{"-d", "@0", "+%u"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %w day of week (0-6, Sunday=0)
		{
			Name:     "day_of_week_w",
			Args:     []string{"-d", "@0", "+%w"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %N nanoseconds
		{
			Name:     "nanoseconds",
			Args:     []string{"-d", "@0", "+%N"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %Z timezone name
		{
			Name:     "timezone_utc",
			Args:     []string{"-u", "-d", "@0", "+%Z"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.4: %P lowercase am/pm
		{
			Name:     "lowercase_ampm",
			Args:     []string{"-d", "@0", "+%P"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.4: %-d no padding modifier
		{
			Name:     "no_pad_day",
			Args:     []string{"-d", "@0", "+%-d"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.4: %_m space padding modifier
		{
			Name:     "space_pad_month",
			Args:     []string{"-d", "@0", "+%_m"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.4: %0e zero padding modifier (overrides space default)
		{
			Name:     "zero_pad_day_e",
			Args:     []string{"-d", "@0", "+%0e"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: combined format with multiple specs
		{
			Name:     "combined_format",
			Args:     []string{"-d", "@1234567890", "+%Y/%m/%d %H:%M:%S %A"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %I 12-hour clock and %p AM/PM
		{
			Name:     "twelve_hour_format",
			Args:     []string{"-d", "@1234567890", "+%I:%M:%S %p"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R2.1: --date=STRING long form displays specified date
		{
			Name:     "date_long_form",
			Args:     []string{"--date=@86400", "+%Y-%m-%d"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R2.2: @EPOCH with negative epoch
		{
			Name:     "epoch_negative",
			Args:     []string{"-d", "@-86400", "+%Y-%m-%d"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R2.2: @EPOCH with fractional seconds
		{
			Name:     "epoch_fractional",
			Args:     []string{"-d", "@1234567890.123", "+%s.%N"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R2.3: ISO 8601 date-time string
		{
			Name:     "iso_datetime",
			Args:     []string{"-d", "2024-01-15 10:30:00", "+%Y-%m-%d %H:%M:%S"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R2.3: ISO 8601 date-only string
		{
			Name:     "iso_date_only",
			Args:     []string{"-d", "2024-06-01", "+%Y-%m-%d"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R2.3: ISO 8601 with T separator
		{
			Name:     "iso_t_separator",
			Args:     []string{"-d", "2024-01-15T10:30:00", "+%Y-%m-%dT%H:%M:%S"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R2.4: invalid date string exits 1 (discard stderr, binary names differ)
		{
			Name:      "invalid_date_error",
			Args:      []string{"-d", "invalid"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.4: non-date text exits 1
		{
			Name:      "garbage_date_error",
			Args:      []string{"-d", "not-a-date-at-all"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.1: -u displays date in UTC
		{
			Name:     "utc_flag_short",
			Args:     []string{"-u", "-d", "@1234567890", "+%Y-%m-%d %H:%M:%S %Z"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: --utc long form
		{
			Name:     "utc_flag_long",
			Args:     []string{"--utc", "-d", "@1234567890", "+%Z"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: --universal synonym
		{
			Name:     "utc_universal_flag",
			Args:     []string{"--universal", "-d", "@1234567890", "+%Z"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.4: output goes to stdout only, no stdin reading
		{
			Name:     "stdout_only_no_stdin",
			Args:     []string{"-u", "-d", "@0", "+%Y"},
			Stdin:    []byte("ignored input\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R4.1: exit 0 on successful display with compound format
		{
			Name:     "exit_0_compound_format",
			Args:     []string{"-d", "@86400", "+%F %T"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R4.2: exit 1 on invalid epoch value
		{
			Name:      "exit_1_bad_epoch",
			Args:      []string{"-d", "@notanumber"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: deterministic output via fixed -d @EPOCH
		{
			Name:     "deterministic_epoch_date",
			Args:     []string{"-u", "-d", "@0", "+%Y-%m-%d %H:%M:%S"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R4.4: comprehensive coverage — padding modifiers with epoch
		{
			Name:     "coverage_padding_modifiers",
			Args:     []string{"-d", "@86400", "+%-m %_d %0H"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
	}

	// R3.2: -r FILE displays the last modification time of FILE.
	// R3.3: missing file exits 1.
	refFile := filepath.Join(t.TempDir(), "reffile")
	if err := os.WriteFile(refFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("creating reference file: %v", err)
	}
	// Set a known modification time so output is deterministic.
	knownTime := parseTestTime(t, "2024-03-15 12:00:00")
	if err := os.Chtimes(refFile, knownTime, knownTime); err != nil {
		t.Fatalf("setting reference file time: %v", err)
	}

	tests = append(tests,
		// R3.2: -r FILE shows file modification time
		testutils.DiffTest{
			Name:     "reference_file",
			Args:     []string{"-r", refFile, "+%Y-%m-%d %H:%M:%S"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R3.2: --reference=FILE long form
		testutils.DiffTest{
			Name:     "reference_file_long",
			Args:     []string{"--reference=" + refFile, "+%Y-%m-%d"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R3.2: -r with -u UTC mode
		testutils.DiffTest{
			Name:     "reference_file_utc",
			Args:     []string{"-u", "-r", refFile, "+%Y-%m-%d %H:%M:%S %Z"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.3: missing referenced file exits 1
		testutils.DiffTest{
			Name:      "reference_file_missing",
			Args:      []string{"-r", "/nonexistent_date_test_file", "+%Y"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

// parseTestTime parses a UTC time string for test setup.
func parseTestTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		t.Fatalf("parseTestTime: %v", err)
	}
	return parsed.UTC()
}
