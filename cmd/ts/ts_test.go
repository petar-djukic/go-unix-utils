// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts covering srd004-ts R1.1-R1.4.
package main

import (
	"os/exec"
	"strings"
	"testing"

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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestStrftimeToGo verifies the strftime-to-Go format conversion for the
// default format used by R1.2.
func TestStrftimeToGo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"default_format", "%b %d %H:%M:%S", "Jan 02 15:04:05"},
		{"iso_date", "%Y-%m-%d", "2006-01-02"},
		{"time_only", "%T", "15:04:05"},
		{"literal_percent", "%%", "%"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := strftimeToGo(tc.input)
			if got != tc.expect {
				t.Errorf("strftimeToGo(%q) = %q, want %q",
					tc.input, got, tc.expect)
			}
		})
	}
}

// TestParseArgs verifies argument parsing for R1.1-R1.4 scope.
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
