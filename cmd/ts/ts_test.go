// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ts against ts (moreutils).
// Implements prd004-ts R1.1-R1.6, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3, R5.1-R5.3, R6.1-R6.5, R7.1-R7.3, R9.1-R9.2 test coverage.
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// epochSubsecNormalizer replaces Unix epoch timestamps with microsecond suffix
// (e.g., "1708358732.001234") with a fixed placeholder. Used for %.s tests.
var epochSubsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{9,}\.\d{6}`)
	return re.ReplaceAll(b, []byte("<EPOCH_USEC>"))
}

// deltaSubsecNormalizer replaces any number with microsecond suffix (e.g.,
// "0.000005") with a fixed placeholder. Used for %.s tests in -i/-s modes
// where the epoch value is a small delta rather than a wall-clock epoch.
var deltaSubsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d+\.\d{6}`)
	return re.ReplaceAll(b, []byte("<DELTA_USEC>"))
}

// secSubsecNormalizer replaces seconds with microsecond suffix (e.g.,
// "32.001234") with a fixed placeholder. Used for %.S tests.
var secSubsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{2}\.\d{6}`)
	return re.ReplaceAll(b, []byte("<SEC_USEC>"))
}

// relativeAgeNormalizer replaces relative age strings (e.g., "5d3h2m1s ago")
// with a fixed placeholder. Used for -r mode tests where the exact age depends
// on when the test runs.
var relativeAgeNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d+[dhms](?:\d+[dhms])* (?:ago|from now)`)
	return re.ReplaceAll(b, []byte("<RELATIVE_AGE>"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: single line with default format.
		{
			Name:      "R1.1_single_line_default_format",
			Stdin:     []byte("hello world\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1: multi-line stdin.
		{
			Name:      "R1.1_multi_line",
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.2: default format produces "Mon DD HH:MM:SS" pattern.
		{
			Name:      "R1.2_default_format",
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.3/R2.1: custom strftime format.
		{
			Name:      "R2.1_custom_format_iso",
			Args:      []string{"%Y-%m-%d %H:%M:%S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: empty stdin produces no output and exits 0.
		{
			Name:      "R9.2_empty_stdin",
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: 10-line input for differential test.
		{
			Name:      "R9.2_ten_lines",
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1: line with spaces and special characters.
		{
			Name:      "R1.1_special_chars",
			Stdin:     []byte("hello\tworld\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom format with date only.
		{
			Name:      "R2.1_custom_format_date_only",
			Args:      []string{"%Y-%m-%d"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.1: incremental mode single line.
		{
			Name:      "R3.1_incremental_single_line",
			Args:      []string{"-i"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.1: incremental mode multi-line.
		{
			Name:      "R3.1_incremental_multi_line",
			Args:      []string{"-i"},
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.3: incremental mode with custom format.
		{
			Name:      "R3.3_incremental_custom_format",
			Args:      []string{"-i", "%H:%M:%S"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1: elapsed mode single line.
		{
			Name:      "R4.1_elapsed_single_line",
			Args:      []string{"-s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1: elapsed mode multi-line.
		{
			Name:      "R4.1_elapsed_multi_line",
			Args:      []string{"-s"},
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.3: elapsed mode with custom format.
		{
			Name:      "R4.3_elapsed_custom_format",
			Args:      []string{"-s", "%H:%M:%S"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: incremental mode empty stdin.
		{
			Name:      "R9.2_incremental_empty_stdin",
			Args:      []string{"-i"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.2: elapsed mode empty stdin.
		{
			Name:      "R9.2_elapsed_empty_stdin",
			Args:      []string{"-s"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom format with time-only format.
		{
			Name:      "R2.1_custom_format_time_only",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: custom format with full datetime.
		{
			Name:      "R2.2_custom_format_full_datetime",
			Args:      []string{"%a %b %e %T %Z %Y"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.3: subsecond extension %.S (seconds with microsecond suffix).
		{
			Name:      "R2.3_subsecond_dotS",
			Args:      []string{"%.S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{secSubsecNormalizer},
		},
		// R2.3: subsecond extension %.s (Unix epoch with microsecond suffix).
		{
			Name:      "R2.3_subsecond_dots",
			Args:      []string{"%.s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{epochSubsecNormalizer},
		},
		// R2.3: subsecond extension %.T (HH:MM:SS with microsecond suffix).
		{
			Name:      "R2.3_subsecond_dotT",
			Args:      []string{"%.T"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.3: subsecond extension %.T with multi-line input.
		{
			Name:      "R2.3_subsecond_dotT_multi_line",
			Args:      []string{"%.T"},
			Stdin:     []byte("line one\nline two\nline three\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.3: subsecond extension %.S with -s mode.
		{
			Name:      "R2.3_subsecond_dotS_elapsed",
			Args:      []string{"-s", "%.S"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{secSubsecNormalizer},
		},
		// R2.3: subsecond extension %.s with -i mode. Uses deltaSubsecNormalizer
		// because -i mode produces small delta epoch values (e.g., "0.000005").
		{
			Name:      "R2.3_subsecond_dots_incremental",
			Args:      []string{"-i", "%.s"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{deltaSubsecNormalizer},
		},
		// R2.3: mixed format with subsecond extensions.
		{
			Name:      "R2.3_mixed_format_with_subsecond",
			Args:      []string{"%Y-%m-%d %.T"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.1: piped multi-line output with default format confirms line-buffered flush.
		{
			Name:      "R5.1_piped_multiline_default",
			Stdin:     []byte("alpha\nbeta\ngamma\ndelta\nepsilon\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.3: piped output with custom strftime format.
		{
			Name:      "R5.3_piped_custom_format",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("one\ntwo\nthree\nfour\nfive\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.3: piped output with incremental mode.
		{
			Name:      "R5.3_piped_incremental_multiline",
			Args:      []string{"-i"},
			Stdin:     []byte("a\nb\nc\nd\ne\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.3: piped output with elapsed mode.
		{
			Name:      "R5.3_piped_elapsed_multiline",
			Args:      []string{"-s"},
			Stdin:     []byte("a\nb\nc\nd\ne\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.3: piped output with subsecond extension %.T across multiple lines.
		{
			Name:      "R5.3_piped_subsecond_dotT",
			Args:      []string{"%.T"},
			Stdin:     []byte("x\ny\nz\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R6.1: -r with syslog timestamp produces relative age string.
		{
			Name:      "R6.1_relative_syslog",
			Args:      []string{"-r"},
			Stdin:     []byte("Jan  5 14:30:00 some log message\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{relativeAgeNormalizer},
		},
		// R6.2: -r with RFC 2822 timestamp (matched via syslog-style regex by reference).
		{
			Name:      "R6.2_relative_rfc2822",
			Args:      []string{"-r"},
			Stdin:     []byte("16 Jun 94 07:29:35 GMT\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{relativeAgeNormalizer},
		},
		// R6.3: -r with custom format reformats syslog timestamp.
		{
			Name:      "R6.3_relative_with_format",
			Args:      []string{"-r", "%Y-%m-%d %H:%M:%S"},
			Stdin:     []byte("Jan  5 14:30:00 logged event\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R6.4: -r with no recognizable timestamp passes through unchanged.
		{
			Name:      "R6.4_relative_no_timestamp",
			Args:      []string{"-r"},
			Stdin:     []byte("no timestamp here\n"),
			Env:       []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestR3_4_MutualExclusion verifies that passing both -i and -s prints a usage
// error to stderr and exits non-zero. R3.4: -i and -s are mutually exclusive.
// This cannot be a differential test because the reference ts binary accepts
// both flags without error; the PRD mandates stricter behavior.
func TestR3_4_MutualExclusion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-i", "-s")
	cmd.Stdin = bytes.NewReader([]byte("test\n"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code when -i and -s are both given")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Fatal("expected non-zero exit code when -i and -s are both given")
	}

	if !bytes.Contains(stderr.Bytes(), []byte("mutually exclusive")) {
		t.Errorf("expected stderr to mention 'mutually exclusive', got: %q", stderr.String())
	}
}

// TestR6_5_MutualExclusion verifies that passing -r with -i or -s prints a
// usage error to stderr and exits non-zero. R6.5: -r is mutually exclusive
// with -i and -s.
func TestR6_5_MutualExclusion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		args []string
	}{
		{"r_and_i", []string{"-r", "-i"}},
		{"r_and_s", []string{"-r", "-s"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(goBin, tc.args...)
			cmd.Stdin = bytes.NewReader([]byte("test\n"))
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Fatal("expected non-zero exit code when -r is combined with -i or -s")
			}

			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
			}
			if exitErr.ExitCode() == 0 {
				t.Fatal("expected non-zero exit code when -r is combined with -i or -s")
			}

			if !bytes.Contains(stderr.Bytes(), []byte("mutually exclusive")) {
				t.Errorf("expected stderr to mention 'mutually exclusive', got: %q", stderr.String())
			}
		})
	}
}

// TestR7_1_ExitZeroOnEOF verifies that ts exits 0 on clean EOF from stdin.
// R7.1: must exit 0 on clean EOF.
func TestR7_1_ExitZeroOnEOF(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name  string
		args  []string
		stdin []byte
	}{
		{"empty_stdin", nil, []byte("")},
		{"single_line", nil, []byte("hello\n")},
		{"with_flag_s", []string{"-s"}, []byte("hello\n")},
		{"with_flag_i", []string{"-i"}, []byte("hello\n")},
		{"with_flag_r", []string{"-r"}, []byte("no ts here\n")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(goBin, tc.args...)
			cmd.Stdin = bytes.NewReader(tc.stdin)
			cmd.Env = append(cmd.Environ(), "LC_ALL=C")

			err := cmd.Run()
			if err != nil {
				t.Fatalf("expected exit 0, got error: %v", err)
			}
		})
	}
}

// TestR7_2_UnrecognizedFlag verifies that ts exits non-zero and prints a usage
// message to stderr when an unrecognized flag is given.
// R7.2: must exit non-zero with usage message on unrecognized flags.
func TestR7_2_UnrecognizedFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		args []string
	}{
		{"unknown_x", []string{"-x"}},
		{"unknown_long", []string{"--unknown"}},
		{"unknown_z", []string{"-z"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(goBin, tc.args...)
			cmd.Stdin = bytes.NewReader([]byte("test\n"))
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Fatal("expected non-zero exit code for unrecognized flag")
			}

			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
			}
			if exitErr.ExitCode() == 0 {
				t.Fatal("expected non-zero exit code for unrecognized flag")
			}

			if !bytes.Contains(stderr.Bytes(), []byte("unrecognized option")) {
				t.Errorf("expected stderr to mention 'unrecognized option', got: %q", stderr.String())
			}
		})
	}
}

// TestR6_RelativeMode verifies -r mode behavior directly without requiring the
// reference binary. R6.1-R6.4.
func TestR6_RelativeMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("R6.1_syslog_relative_age", func(t *testing.T) {
		t.Parallel()

		// Use a timestamp from "now" to get a predictable small relative age.
		now := time.Now()
		syslogTs := now.Add(-5 * time.Minute).Format("Jan  2 15:04:05")
		input := syslogTs + " syslog message\n"

		cmd := exec.Command(goBin, "-r")
		cmd.Stdin = strings.NewReader(input)
		cmd.Env = append(cmd.Environ(), "LC_ALL=C")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(string(out))
		if !strings.Contains(output, "ago") {
			t.Errorf("expected relative age with 'ago', got: %q", output)
		}
		if !strings.Contains(output, "syslog message") {
			t.Errorf("expected original message preserved, got: %q", output)
		}
	})

	t.Run("R6.2_iso8601_recognized", func(t *testing.T) {
		t.Parallel()

		cmd := exec.Command(goBin, "-r")
		cmd.Stdin = strings.NewReader("2024-01-05T14:30:00Z event happened\n")
		cmd.Env = append(cmd.Environ(), "LC_ALL=C")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(string(out))
		if !strings.Contains(output, "ago") {
			t.Errorf("expected relative age with 'ago', got: %q", output)
		}
		if !strings.Contains(output, "event happened") {
			t.Errorf("expected original message preserved, got: %q", output)
		}
	})

	t.Run("R6.3_format_reformats_timestamp", func(t *testing.T) {
		t.Parallel()

		cmd := exec.Command(goBin, "-r", "%Y-%m-%d %H:%M:%S")
		cmd.Stdin = strings.NewReader("2024-01-05T14:30:00Z event\n")
		cmd.Env = append(cmd.Environ(), "LC_ALL=C")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(string(out))
		if !strings.Contains(output, "2024-01-05 14:30:00") {
			t.Errorf("expected reformatted timestamp '2024-01-05 14:30:00', got: %q", output)
		}
		if strings.Contains(output, "ago") {
			t.Errorf("with custom format, should not produce relative age, got: %q", output)
		}
	})

	t.Run("R6.4_no_timestamp_passthrough", func(t *testing.T) {
		t.Parallel()

		cmd := exec.Command(goBin, "-r")
		cmd.Stdin = strings.NewReader("no timestamp here\n")
		cmd.Env = append(cmd.Environ(), "LC_ALL=C")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := strings.TrimSpace(string(out))
		if output != "no timestamp here" {
			t.Errorf("expected passthrough 'no timestamp here', got: %q", output)
		}
	})
}
