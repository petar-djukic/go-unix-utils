// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts covering srd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3, R5.1-R5.3.
package main

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// subsecondRe matches a dot followed by 1-6 digits after a TIMESTAMP placeholder
// or after a time-like pattern, normalizing microsecond differences from timing.
var subsecondRe = regexp.MustCompile(`\.\d{1,6}`)

// subsecondNormalizer strips subsecond suffixes (.USEC) that vary due to
// timing differences between the Go and reference binary executions.
var subsecondNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return subsecondRe.ReplaceAll(b, []byte(".SUBSEC"))
}

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
		// R2.3: %.s subsecond extension (epochRe in TimestampNormalizer covers epoch.usec)
		{
			Name:      "subsecond_dots",
			Args:      []string{"%.s"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.1, R3.2: incremental mode
		{
			Name:      "incremental_single_line",
			Args:      []string{"-i"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:      "incremental_multi_line",
			Args:      []string{"-i"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:  "incremental_empty_stdin",
			Args:  []string{"-i"},
			Stdin: nil,
		},
		// R3.3: -i with custom format overrides default
		{
			Name:      "incremental_custom_format",
			Args:      []string{"-i", "%H:%M:%S"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.4: -i and -s combined (reference treats as -s)
		{
			Name:      "incremental_elapsed_combined",
			Args:      []string{"-i", "-s"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1, R4.2: elapsed-since-start mode
		{
			Name:      "elapsed_single_line",
			Args:      []string{"-s"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:      "elapsed_multi_line",
			Args:      []string{"-s"},
			Stdin:     []byte("line1\nline2\nline3\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		{
			Name:  "elapsed_empty_stdin",
			Args:  []string{"-s"},
			Stdin: nil,
		},
		// R4.2: -s with custom format overrides default
		{
			Name:      "elapsed_custom_format",
			Args:      []string{"-s", "%H:%M:%S"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.3: -s with custom format (%.T subsecond extension)
		{
			Name:      "elapsed_custom_subsecond",
			Args:      []string{"-s", "%.T"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer, subsecondNormalizer},
		},
		// R5.1, R5.2: -m monotonic mode with default format
		{
			Name:      "monotonic_default",
			Args:      []string{"-m"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2: -m with -i (monotonic incremental)
		{
			Name:      "monotonic_incremental",
			Args:      []string{"-m", "-i"},
			Stdin:     []byte("line1\nline2\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2: -m with -s (monotonic elapsed)
		{
			Name:      "monotonic_elapsed",
			Args:      []string{"-m", "-s"},
			Stdin:     []byte("line1\nline2\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2: -m with custom format
		{
			Name:      "monotonic_custom_format",
			Args:      []string{"-m", "%Y-%m-%d"},
			Stdin:     []byte("hello\n"),
		},
		// R5.2: -m with subsecond extension
		{
			Name:      "monotonic_subsecond",
			Args:      []string{"-m", "%.s"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.3: -m multi-line (monotonic clock should not jump)
		{
			Name:      "monotonic_multi_line",
			Args:      []string{"-m"},
			Stdin:     []byte("a\nb\nc\nd\ne\n"),
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2: -m empty stdin
		{
			Name:  "monotonic_empty_stdin",
			Args:  []string{"-m"},
			Stdin: nil,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFormatStrftime verifies strftime formatting for R2.2 and R2.3.
func TestFormatStrftime(t *testing.T) {
	t.Parallel()
	ref := time.Date(2009, 2, 13, 23, 31, 30, 0, time.UTC)
	refAM := time.Date(2009, 1, 5, 3, 5, 9, 0, time.UTC)
	// R2.3: time with non-zero microseconds for subsecond tests.
	refUsec := time.Date(2009, 2, 13, 23, 31, 30, 123456000, time.UTC)
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
		// R2.3: subsecond extensions with zero microseconds
		{"subsecond_S_zero", "%.S", ref, "30.000000"},
		{"subsecond_s_zero", "%.s", ref, "1234567890.000000"},
		{"subsecond_T_zero", "%.T", ref, "23:31:30.000000"},
		// R2.3: subsecond extensions with non-zero microseconds
		{"subsecond_S_usec", "%.S", refUsec, "30.123456"},
		{"subsecond_s_usec", "%.s", refUsec, "1234567890.123456"},
		{"subsecond_T_usec", "%.T", refUsec, "23:31:30.123456"},
		// R2.3: unknown subsecond specifier passes through
		{"subsecond_unknown", "%.X", ref, "%.X"},
		// R2.3: subsecond mixed with regular specifiers
		{"subsecond_mixed", "%Y-%.T", refUsec, "2009-23:31:30.123456"},
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

// TestParseArgs verifies argument parsing for R2.1, R3.1-R3.4, R4.1-R4.3, R5.1-R5.3, R7.2.
func TestParseArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		args        []string
		wantFmt     string
		wantIncr    bool
		wantElapsed bool
		wantMono    bool
		wantErr     bool
		errSubstr   string
	}{
		{"no_args", nil, "%b %d %H:%M:%S", false, false, false, false, ""},
		{"custom_format", []string{"%Y-%m-%d"}, "%Y-%m-%d", false, false, false, false, ""},
		{"custom_format_epoch", []string{"%s"}, "%s", false, false, false, false, ""},
		{"unknown_flag", []string{"-x"}, "", false, false, false, true, "unrecognized option"},
		{"dash_only", []string{"-"}, "-", false, false, false, false, ""},
		// R3.1: -i flag
		{"incr_flag", []string{"-i"}, "%H:%M:%S", true, false, false, false, ""},
		// R3.3: -i with custom format
		{"incr_custom", []string{"-i", "%T"}, "%T", true, false, false, false, ""},
		// R3.4: -i and -s combined, -s takes precedence (matches reference)
		{"incr_elapsed_combined", []string{"-i", "-s"}, "%H:%M:%S", false, true, false, false, ""},
		{"elapsed_incr_combined", []string{"-s", "-i"}, "%H:%M:%S", false, true, false, false, ""},
		// R4.1: -s flag
		{"elapsed_flag", []string{"-s"}, "%H:%M:%S", false, true, false, false, ""},
		// R4.2, R4.3: -s with custom format overrides default
		{"elapsed_custom", []string{"-s", "%T"}, "%T", false, true, false, false, ""},
		// R4.3: -s with subsecond custom format
		{"elapsed_custom_subsec", []string{"-s", "%.T"}, "%.T", false, true, false, false, ""},
		// R5.1: -m flag alone
		{"mono_flag", []string{"-m"}, "%b %d %H:%M:%S", false, false, true, false, ""},
		// R5.2: -m with -i
		{"mono_incr", []string{"-m", "-i"}, "%H:%M:%S", true, false, true, false, ""},
		// R5.2: -m with -s
		{"mono_elapsed", []string{"-m", "-s"}, "%H:%M:%S", false, true, true, false, ""},
		// R5.2: -m with custom format
		{"mono_custom", []string{"-m", "%Y"}, "%Y", false, false, true, false, ""},
		// R5.2: -m with -s and custom format
		{"mono_elapsed_custom", []string{"-m", "-s", "%.T"}, "%.T", false, true, true, false, ""},
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
			if got.format != tc.wantFmt {
				t.Errorf("parseArgs(%v).format = %q, want %q",
					tc.args, got.format, tc.wantFmt)
			}
			if got.incremental != tc.wantIncr {
				t.Errorf("parseArgs(%v).incremental = %v, want %v",
					tc.args, got.incremental, tc.wantIncr)
			}
			if got.elapsed != tc.wantElapsed {
				t.Errorf("parseArgs(%v).elapsed = %v, want %v",
					tc.args, got.elapsed, tc.wantElapsed)
			}
			if got.monotonic != tc.wantMono {
				t.Errorf("parseArgs(%v).monotonic = %v, want %v",
					tc.args, got.monotonic, tc.wantMono)
			}
		})
	}
}
