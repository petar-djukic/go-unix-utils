// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts covering srd004-ts R1.1-R1.6, R2.1-R2.2.
package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:      "default_format_single_line",
			Stdin:     []byte("hello world\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:      "default_format_multi_line",
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:  "empty_stdin",
			Stdin: nil,
		},
		{
			Name:      "single_word",
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:      "line_with_spaces",
			Stdin:     []byte("  leading and trailing  \n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:      "multiple_blank_lines",
			Stdin:     []byte("\n\n\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.5: partial last line (no trailing newline at EOF)
		{
			Name:      "partial_last_line",
			Stdin:     []byte("no newline"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.5: mixed complete and partial lines
		{
			Name:      "partial_after_complete",
			Stdin:     []byte("complete\npartial"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom format string (date-only, stable within a day)
		{
			Name:  "custom_format_date",
			Args:  []string{"%Y-%m-%d"},
			Stdin: []byte("hello\n"),
		},
		// R2.2: %F shorthand for ISO date
		{
			Name:  "custom_format_F",
			Args:  []string{"%F"},
			Stdin: []byte("test\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFormatStrftime verifies strftime formatting for R2.2.
func TestFormatStrftime(t *testing.T) {
	t.Parallel()
	ref := time.Date(2009, 2, 13, 23, 31, 30, 0, time.UTC)
	refAM := time.Date(2009, 1, 5, 3, 5, 9, 0, time.UTC)
	tests := []struct {
		name   string
		format string
		t      time.Time
		want   string
	}{
		{"default_format", "%b %d %H:%M:%S", ref, "Feb 13 23:31:30"},
		{"iso_date", "%Y-%m-%d", ref, "2009-02-13"},
		{"time_only", "%T", ref, "23:31:30"},
		{"literal_percent", "%%", ref, "%"},
		{"empty", "", ref, ""},
		{"epoch", "%s", ref, "1234567890"},
		{"century", "%C", ref, "20"},
		{"hour_space_24", "%k", ref, "23"},
		{"hour_space_24_single", "%k", refAM, " 3"},
		{"hour_space_12", "%l", ref, "11"},
		{"hour_space_12_single", "%l", refAM, " 3"},
		{"lower_meridiem_pm", "%P", ref, "pm"},
		{"lower_meridiem_am", "%P", refAM, "am"},
		{"weekday_iso", "%u", ref, "5"},
		{"weekday_sunday", "%w", ref, "5"},
		{"week_number_sunday", "%U", ref, "06"},
		{"week_number_monday", "%W", ref, "06"},
		{"iso_week", "%V", ref, "07"},
		{"iso_week_year", "%G", ref, "2009"},
		{"iso_week_year_short", "%g", ref, "09"},
		{"full_weekday", "%A", ref, "Friday"},
		{"month_name", "%B", ref, "February"},
		{"day_space", "%e", ref, "13"},
		{"julian_day", "%j", ref, "044"},
		{"date_F", "%F", ref, "2009-02-13"},
		{"date_D", "%D", ref, "02/13/09"},
		{"time_R", "%R", ref, "23:31"},
		{"locale_datetime", "%c", ref, "Fri Feb 13 23:31:30 2009"},
		{"locale_date", "%x", ref, "02/13/09"},
		{"locale_time", "%X", ref, "23:31:30"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatStrftime(tc.format, tc.t)
			if got != tc.want {
				t.Errorf("formatStrftime(%q) = %q, want %q",
					tc.format, got, tc.want)
			}
		})
	}
}

// TestParseArgs verifies argument parsing for R2.1 and R7.2.
func TestParseArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		wantFmt   string
		wantErr   bool
		errSubstr string
	}{
		{"no_args", nil, "%b %d %H:%M:%S", false, ""},
		{"custom_format", []string{"%Y-%m-%d"}, "%Y-%m-%d", false, ""},
		{"custom_format_epoch", []string{"%s"}, "%s", false, ""},
		{"unknown_flag", []string{"-x"}, "", true, "unrecognized option"},
		{"dash_only", []string{"-"}, "-", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error %q missing substring %q",
						err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantFmt {
				t.Errorf("parseArgs(%v) = %q, want %q",
					tc.args, got, tc.wantFmt)
			}
		})
	}
}
