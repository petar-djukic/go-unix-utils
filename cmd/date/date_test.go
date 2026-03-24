// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/date against GNU gdate.
// Covers prd060-date R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gdate and Go date.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?date|gdate`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	// GNU treats bare operands as -d input; Go rejects them as extra operands.
	extraOperand := regexp.MustCompile(
		`(?:invalid date|extra operand) '[^']*'`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("date"))
		b = tryHelp.ReplaceAll(b, nil)
		b = extraOperand.ReplaceAll(b, []byte("rejected input"))
		return b
	}
}

// createRefFile creates a temporary file with a known modification time
// for -r/--reference tests.
func createRefFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "reffile")
	if err := os.WriteFile(p, []byte("ref"), 0o644); err != nil {
		t.Fatalf("create ref file: %v", err)
	}
	// R4.4: set modification time to a known epoch for deterministic output.
	refTime := time.Unix(1700000000, 0)
	if err := os.Chtimes(p, refTime, refTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return p
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdate")
	if err != nil {
		t.Skipf("reference binary gdate not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()
	refFile := createRefFile(t)
	utcEnv := []string{"TZ=UTC"}

	tests := []testutils.DiffTest{
		// ===== R4.1: Core format directives and basic date output =====

		// R4.1/R4.4: default output with fixed epoch (TZ=UTC for determinism).
		{
			Name: "default_format_epoch",
			Args: []string{"-d", "@0"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %Y-%m-%d %H:%M:%S full date-time format.
		{
			Name: "ymd_hms_epoch0",
			Args: []string{"-d", "@0", "+%Y-%m-%d %H:%M:%S"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %s seconds since epoch roundtrip.
		{
			Name: "epoch_seconds_roundtrip",
			Args: []string{"-d", "@1700000000", "+%s"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %A full weekday name.
		{
			Name: "weekday_name_full",
			Args: []string{"-d", "@0", "+%A"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %a abbreviated weekday name.
		{
			Name: "weekday_name_abbr",
			Args: []string{"-d", "@86400", "+%a"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %B full month name.
		{
			Name: "month_name_full",
			Args: []string{"-d", "@1700000000", "+%B"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %b abbreviated month name.
		{
			Name: "month_name_abbr",
			Args: []string{"-d", "@1700000000", "+%b"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %j day of year.
		{
			Name: "day_of_year",
			Args: []string{"-d", "@1700000000", "+%j"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %u ISO weekday (1=Mon, 7=Sun).
		{
			Name: "iso_weekday",
			Args: []string{"-d", "@0", "+%u"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %w weekday (0=Sun, 6=Sat).
		{
			Name: "weekday_number",
			Args: []string{"-d", "@0", "+%w"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %N nanoseconds.
		{
			Name: "nanoseconds",
			Args: []string{"-d", "@0", "+%N"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %Z timezone name.
		{
			Name: "timezone_name_utc",
			Args: []string{"-u", "-d", "@0", "+%Z"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %z numeric timezone offset.
		{
			Name: "timezone_offset_utc",
			Args: []string{"-u", "-d", "@0", "+%z"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %p AM/PM indicator.
		{
			Name: "ampm_upper",
			Args: []string{"-d", "@0", "+%p"},
			Env:  utcEnv,
		},
		// R4.1/R4.4: %I 12-hour format.
		{
			Name: "hour_12",
			Args: []string{"-d", "@0", "+%I"},
			Env:  utcEnv,
		},

		// ===== R4.2: Extended format directives and combinations =====

		// R4.2/R4.4: %F full date shortcut (ISO 8601).
		{
			Name: "format_F_iso_date",
			Args: []string{"-d", "@1700000000", "+%F"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %T time shortcut.
		{
			Name: "format_T_time",
			Args: []string{"-d", "@1700000000", "+%T"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %D US date format.
		{
			Name: "format_D_us_date",
			Args: []string{"-d", "@1700000000", "+%D"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %R 24-hour time shortcut.
		{
			Name: "format_R_24h",
			Args: []string{"-d", "@1700000000", "+%R"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %r 12-hour time with AM/PM.
		{
			Name: "format_r_12h",
			Args: []string{"-d", "@1700000000", "+%r"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %c locale date and time.
		{
			Name: "format_c_locale_datetime",
			Args: []string{"-d", "@1700000000", "+%c"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %x locale date.
		{
			Name: "format_x_locale_date",
			Args: []string{"-d", "@1700000000", "+%x"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %X locale time.
		{
			Name: "format_X_locale_time",
			Args: []string{"-d", "@1700000000", "+%X"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %P lowercase am/pm (GNU extension).
		{
			Name: "ampm_lower_P",
			Args: []string{"-d", "@0", "+%P"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %P in afternoon.
		{
			Name: "ampm_lower_P_pm",
			Args: []string{"-d", "@43200", "+%P"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: padding modifier %-d (no padding).
		{
			Name: "pad_none_day",
			Args: []string{"-d", "@1700000000", "+%-d"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: padding modifier %_d (space padding).
		{
			Name: "pad_space_day",
			Args: []string{"-d", "@1700000000", "+%_d"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: padding modifier %0d (zero padding).
		{
			Name: "pad_zero_day",
			Args: []string{"-d", "@1700000000", "+%0d"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: padding modifier %-m (no padding for month).
		{
			Name: "pad_none_month",
			Args: []string{"-d", "@0", "+%-m"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: padding modifier %_H (space-padded hour).
		{
			Name: "pad_space_hour",
			Args: []string{"-d", "@3600", "+%_H"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: combined format with multiple directives.
		{
			Name: "combined_format",
			Args: []string{"-d", "@1700000000", "+%Y/%m/%d %H:%M:%S %Z"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: literal %% in format.
		{
			Name: "literal_percent",
			Args: []string{"-d", "@0", "+100%%"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %n newline in format.
		{
			Name: "format_newline",
			Args: []string{"-d", "@0", "+line1%nline2"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %t tab in format.
		{
			Name: "format_tab",
			Args: []string{"-d", "@0", "+col1%tcol2"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %y two-digit year.
		{
			Name: "two_digit_year",
			Args: []string{"-d", "@1700000000", "+%y"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %C century.
		{
			Name: "century",
			Args: []string{"-d", "@1700000000", "+%C"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %e space-padded day.
		{
			Name: "space_padded_day",
			Args: []string{"-d", "@0", "+%e"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %k space-padded hour.
		{
			Name: "space_padded_hour",
			Args: []string{"-d", "@3600", "+%k"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %l space-padded 12-hour.
		{
			Name: "space_padded_12h",
			Args: []string{"-d", "@3600", "+%l"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %h abbreviated month (alias for %b).
		{
			Name: "month_abbr_h",
			Args: []string{"-d", "@1700000000", "+%h"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %U week number (Sunday start).
		{
			Name: "week_number_sunday",
			Args: []string{"-d", "@1700000000", "+%U"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %W week number (Monday start).
		{
			Name: "week_number_monday",
			Args: []string{"-d", "@1700000000", "+%W"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %V ISO week number.
		{
			Name: "iso_week_number",
			Args: []string{"-d", "@1700000000", "+%V"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %G ISO week-based year.
		{
			Name: "iso_week_year",
			Args: []string{"-d", "@1700000000", "+%G"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: %g ISO week-based 2-digit year.
		{
			Name: "iso_week_year_short",
			Args: []string{"-d", "@1700000000", "+%g"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: -u flag with -d for UTC output.
		{
			Name: "utc_flag_with_date",
			Args: []string{"-u", "-d", "@1700000000", "+%Y-%m-%d %H:%M:%S %Z"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: -d with ISO 8601 date string.
		{
			Name: "iso_date_string",
			Args: []string{"-d", "2024-01-15 10:30:00", "+%Y-%m-%d %H:%M:%S"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: -d with ISO 8601 date-only string.
		{
			Name: "iso_date_only",
			Args: []string{"-d", "2024-01-15", "+%Y-%m-%d"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: -r reference file with fixed mtime.
		{
			Name: "reference_file",
			Args: []string{"-r", refFile, "+%s"},
			Env:  utcEnv,
		},
		// R4.2/R4.4: -r reference file with format.
		{
			Name: "reference_file_format",
			Args: []string{"-r", refFile, "+%Y-%m-%d"},
			Env:  utcEnv,
		},

		// ===== R4.3: Error conditions =====

		// R4.3/R4.4: invalid date string.
		{
			Name:      "error_invalid_date",
			Args:      []string{"-d", "not-a-date"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R4.4: missing reference file.
		{
			Name:      "error_missing_ref_file",
			Args:      []string{"-r", "/nonexistent/file/path"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R4.4: invalid option.
		{
			Name:      "error_invalid_option",
			Args:      []string{"-Q"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R4.4: extra operand (GNU treats as date string, Go as operand).
		{
			Name:      "error_extra_operand",
			Args:      []string{"extra"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// ===== R4.4: Edge cases =====

		// R4.4: epoch 0 (Unix epoch boundary).
		{
			Name: "epoch_zero_full",
			Args: []string{"-d", "@0", "+%Y-%m-%d %H:%M:%S"},
			Env:  utcEnv,
		},
		// R4.4: negative epoch (before Unix epoch).
		{
			Name: "negative_epoch",
			Args: []string{"-d", "@-86400", "+%Y-%m-%d"},
			Env:  utcEnv,
		},
		// R4.4: large epoch value.
		{
			Name: "large_epoch",
			Args: []string{"-d", "@2000000000", "+%Y-%m-%d %H:%M:%S"},
			Env:  utcEnv,
		},
		// R4.4: epoch at midnight boundary.
		{
			Name: "epoch_midnight_boundary",
			Args: []string{"-d", "@86399", "+%H:%M:%S"},
			Env:  utcEnv,
		},
		// R4.4: epoch just past midnight.
		{
			Name: "epoch_just_past_midnight",
			Args: []string{"-d", "@86400", "+%Y-%m-%d %H:%M:%S"},
			Env:  utcEnv,
		},
		// R4.4: leap year date (Feb 29).
		{
			Name: "leap_year_feb29",
			Args: []string{"-d", "2024-02-29", "+%Y-%m-%d %j"},
			Env:  utcEnv,
		},
		// R4.4: non-leap year end of year.
		{
			Name: "end_of_year",
			Args: []string{"-d", "2023-12-31", "+%j %u %w"},
			Env:  utcEnv,
		},
		// R4.4: year 2000 epoch.
		{
			Name: "y2k_epoch",
			Args: []string{"-d", "@946684800", "+%Y-%m-%d %H:%M:%S"},
			Env:  utcEnv,
		},
		// R4.4: --date= form.
		{
			Name: "long_date_equals",
			Args: []string{"--date=@1700000000", "+%s"},
			Env:  utcEnv,
		},
		// R4.4: --utc long option.
		{
			Name: "long_utc_option",
			Args: []string{"--utc", "-d", "@1700000000", "+%Z"},
			Env:  utcEnv,
		},
		// R4.4: --universal long option.
		{
			Name: "long_universal_option",
			Args: []string{"--universal", "-d", "@1700000000", "+%Z"},
			Env:  utcEnv,
		},
		// R4.4: --reference= long option.
		{
			Name: "long_reference_option",
			Args: []string{"--reference=" + refFile, "+%s"},
			Env:  utcEnv,
		},
		// R4.4: format with only literal text (no directives).
		{
			Name: "literal_only_format",
			Args: []string{"-d", "@0", "+hello world"},
			Env:  utcEnv,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
