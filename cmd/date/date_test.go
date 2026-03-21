// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd060-date R1.1–R1.4.
package main

import (
	"os/exec"
	"regexp"
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
	tests := []testutils.DiffTest{
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
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

// normalizeTimePortion replaces HH:MM:SS with a placeholder to avoid
// flakiness when the two binaries execute across a second boundary.
func normalizeTimePortion(data []byte) []byte {
	re := regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)
	return re.ReplaceAll(data, []byte("HH:MM:SS"))
}
