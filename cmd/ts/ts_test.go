// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// subsecNormalizer replaces bare subsecond timestamps (e.g., "32.001234")
// with a fixed placeholder, for %.S format where TimestampNormalizer does
// not match the pattern.
var subsecNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d{1,2}\.\d{6}`)
	return re.ReplaceAll(b, []byte("SS.USEC"))
}

// relativeAgeNormalizer replaces relative age strings (e.g., "5m30s ago")
// with a fixed placeholder so timing differences between binary invocations
// do not cause false divergences.
var relativeAgeNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`\d+[ydhms](?:\d+[ydhms])? (?:ago|from now)|right now`)
	return re.ReplaceAll(b, []byte("REL_AGE"))
}

// TestDiff verifies cmd/ts against the moreutils reference binary ts.
// Implements prd004-ts R9.1-R9.2.
// R7.3: parsing dependency always available in Go (invariant).
// R8.1: TZ environment respected for wall-clock timestamps.
// R8.2: -i/-s use TZ=GMT internally regardless of user TZ.
// R9.1: uses TimestampNormalizer for wall-clock timestamp comparison.
// R9.2: covers default format, custom format, subsecond extensions (R2.3, R2.4),
// -i incremental mode (R3.1-R3.4), -s elapsed mode (R4.1-R4.3),
// -m monotonic mode (R5.1-R5.3), -r relative mode (R6.1, R6.2),
// empty stdin, partial last line, additional strftime specifiers (R2.2),
// R7.1 exit codes, R10.2 no-timestamp passthrough.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skipf("reference binary ts not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2, R1.4: default format with three lines.
		{
			Name:      "default_format_three_lines",
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.1, R1.6, R7.1: empty stdin produces no output and exits 0.
		{
			Name:     "default_format_empty_stdin",
			Stdin:    []byte(""),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.5: partial last line (no trailing newline) is timestamped.
		{
			Name:      "default_format_partial_last_line",
			Stdin:     []byte("partial"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom strftime format string.
		{
			Name:      "custom_strftime_format",
			Args:      []string{"%Y-%m-%dT%H:%M:%S"},
			Stdin:     []byte("hello\nworld\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R8.1: TZ=UTC causes timestamps to be in UTC.
		{
			Name:      "tz_environment_respected",
			Args:      []string{"%H:%M:%S"},
			Stdin:     []byte("event\n"),
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: ISO week-based year and week number (%G, %V).
		{
			Name:      "strftime_iso_week",
			Args:      []string{"%G-W%V"},
			Stdin:     []byte("isoweek\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: week number with Sunday start (%U) and Monday start (%W).
		{
			Name:      "strftime_week_numbers",
			Args:      []string{"%U %W"},
			Stdin:     []byte("weeks\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: day of year (%j), century (%C), ISO weekday (%u).
		{
			Name:      "strftime_day_of_year_and_century",
			Args:      []string{"%j %C %u"},
			Stdin:     []byte("misc\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: short ISO week year (%g), weekday number (%w).
		{
			Name:      "strftime_short_iso_year_weekday",
			Args:      []string{"%g %w"},
			Stdin:     []byte("data\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R1.5: multiple lines with partial last line (no trailing newline).
		{
			Name:      "multiline_partial_last",
			Stdin:     []byte("first\nsecond"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.3, R2.4: subsecond extension %.S (seconds with microsecond suffix).
		{
			Name:      "subsecond_format_dotS",
			Args:      []string{"%.S"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{subsecNormalizer},
		},
		// R2.3, R2.4: subsecond extension %.T (HH:MM:SS with microsecond suffix).
		{
			Name:      "subsecond_format_dotT",
			Args:      []string{"%.T"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.1, R3.2: incremental mode shows elapsed time since previous line.
		{
			Name:      "incremental_mode",
			Args:      []string{"-i"},
			Stdin:     []byte("first\nsecond\nthird\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.3: incremental mode with custom format overrides default.
		{
			Name:      "incremental_mode_custom_format",
			Args:      []string{"-i", "%M:%S"},
			Stdin:     []byte("alpha\nbeta\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.4: -i and -s together — last flag wins (matches reference).
		{
			Name:      "incremental_and_elapsed_last_wins",
			Args:      []string{"-i", "-s"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1, R4.2: elapsed-since-start mode with default format.
		{
			Name:      "elapsed_mode",
			Args:      []string{"-s"},
			Stdin:     []byte("first\nsecond\nthird\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.3: elapsed-since-start mode with custom format overrides default.
		{
			Name:      "elapsed_mode_custom_format",
			Args:      []string{"-s", "%M:%S"},
			Stdin:     []byte("alpha\nbeta\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R3.4: -s and -i together (reverse order) — last flag wins.
		{
			Name:      "elapsed_and_incremental_last_wins",
			Args:      []string{"-s", "-i"},
			Stdin:     []byte("test\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R4.1, R4.2, R7.1: elapsed mode with empty stdin exits 0.
		{
			Name:     "elapsed_mode_empty_stdin",
			Args:     []string{"-s"},
			Stdin:    []byte(""),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.1, R5.2: monotonic mode with default format.
		{
			Name:      "monotonic_default_format",
			Args:      []string{"-m"},
			Stdin:     []byte("mono\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2: monotonic mode with custom format string.
		{
			Name:      "monotonic_custom_format",
			Args:      []string{"-m", "%H:%M:%S"},
			Stdin:     []byte("mono\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2: monotonic mode combined with -s (elapsed since start).
		{
			Name:      "monotonic_elapsed_mode",
			Args:      []string{"-m", "-s"},
			Stdin:     []byte("first\nsecond\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2: monotonic mode combined with -i (incremental).
		{
			Name:      "monotonic_incremental_mode",
			Args:      []string{"-m", "-i"},
			Stdin:     []byte("first\nsecond\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R5.2, R5.3: monotonic mode with subsecond extension.
		{
			Name:      "monotonic_subsecond_format",
			Args:      []string{"-m", "%.T"},
			Stdin:     []byte("precise\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R6.1: -r mode replaces syslog timestamps with relative age.
		{
			Name:      "relative_mode_syslog",
			Args:      []string{"-r"},
			Stdin:     []byte("Jan  1 00:00:00 system started\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{relativeAgeNormalizer},
		},
		// R6.2: -r mode recognizes RFC 2822 timestamps.
		{
			Name:      "relative_mode_rfc2822",
			Args:      []string{"-r"},
			Stdin:     []byte("16 Jun 2024 07:29:35 GMT event\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{relativeAgeNormalizer},
		},
		// R10.2: -r mode passes through lines without timestamps unchanged.
		{
			Name:     "relative_mode_no_timestamp",
			Args:     []string{"-r"},
			Stdin:    []byte("no timestamp here\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R6.1, R7.1: -r mode with empty stdin exits 0.
		{
			Name:     "relative_mode_empty_stdin",
			Args:     []string{"-r"},
			Stdin:    []byte(""),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R6.1: -r mode with multiple lines, some with timestamps.
		{
			Name:      "relative_mode_mixed_lines",
			Args:      []string{"-r"},
			Stdin:     []byte("Jan  1 00:00:00 boot\nno ts here\nJan  1 00:00:01 init\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{relativeAgeNormalizer},
		},
		// R7.3: -r mode works (parsing dependency always available in Go).
		{
			Name:      "relative_mode_parsing_available",
			Args:      []string{"-r"},
			Stdin:     []byte("Jan  5 14:30:00 event\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{relativeAgeNormalizer},
		},
		// R8.1: TZ=UTC causes wall-clock timestamps to be in UTC.
		{
			Name:      "tz_utc_wall_clock",
			Args:      []string{"%Z %H:%M:%S"},
			Stdin:     []byte("tztest\n"),
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R8.2: -s mode uses TZ=GMT internally despite user TZ setting.
		{
			Name:      "elapsed_mode_ignores_user_tz",
			Args:      []string{"-s"},
			Stdin:     []byte("line1\nline2\n"),
			Env:       []string{"LC_ALL=C", "TZ=US/Eastern"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R8.2: -i mode uses TZ=GMT internally despite user TZ setting.
		{
			Name:      "incremental_mode_ignores_user_tz",
			Args:      []string{"-i"},
			Stdin:     []byte("line1\nline2\n"),
			Env:       []string{"LC_ALL=C", "TZ=US/Eastern"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R9.1, R9.2: multi-line default format with TimestampNormalizer.
		{
			Name:      "normalizer_multi_line_default",
			Stdin:     []byte("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestRelativeISO8601 verifies R6.2: -r mode recognizes ISO-8601 timestamps.
// Tested standalone because Date::Parse availability varies across systems,
// and the reference binary may not handle ISO-8601.
func TestRelativeISO8601(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-r")
	cmd.Stdin = strings.NewReader("event at 2020-01-01T00:00:00Z done\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, "2020-01-01") {
		t.Errorf("ISO-8601 timestamp not replaced: %q", out)
	}
	if !strings.Contains(out, "ago") {
		t.Errorf("expected relative age with 'ago', got: %q", out)
	}
}

// TestRelativeLastlog verifies R6.2: -r mode recognizes lastlog timestamps.
// Tested standalone because Date::Parse availability varies across systems.
func TestRelativeLastlog(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-r")
	cmd.Stdin = strings.NewReader("Mon Jan  6 14:30 login\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, "14:30") {
		t.Errorf("lastlog timestamp not replaced: %q", out)
	}
	if !strings.Contains(out, "ago") {
		t.Errorf("expected relative age with 'ago', got: %q", out)
	}
}

// TestUnrecognizedFlag verifies R7.2: unrecognized flags cause non-zero
// exit with a usage message on stderr.
func TestUnrecognizedFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "-x")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code for unrecognized flag -x")
	}
	if stderr.Len() == 0 {
		t.Error("expected usage message on stderr for unrecognized flag")
	}
}

// TestRelativeWithFormat verifies R10.1: -r with a format string converts
// matched timestamps to strftime format instead of relative age.
// Tested standalone because the reference binary behavior may vary.
func TestRelativeWithFormat(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-r", "%Y-%m-%d")
	cmd.Stdin = strings.NewReader("event at 2020-06-15T12:30:00Z done\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	// R10.1: timestamp should be converted to strftime format, not relative age.
	if strings.Contains(out, "ago") {
		t.Errorf("expected strftime output, got relative age: %q", out)
	}
	if !strings.Contains(out, "2020-06-15") {
		t.Errorf("expected strftime-formatted date 2020-06-15, got: %q", out)
	}
	if !strings.Contains(out, "done") {
		t.Errorf("expected rest of line preserved, got: %q", out)
	}
}

// TestRelativeWithFormatNoTimestamp verifies R10.2: -r with format passes
// through lines without recognizable timestamps unchanged.
func TestRelativeWithFormatNoTimestamp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-r", "%Y-%m-%d")
	cmd.Stdin = strings.NewReader("no timestamp here\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if out != "no timestamp here\n" {
		t.Errorf("expected unchanged line, got: %q", out)
	}
}

// TestRelativeWithSyslogFormat verifies R10.1: -r with a format string
// converts syslog timestamps to the specified strftime format.
func TestRelativeWithSyslogFormat(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-r", "%H:%M:%S")
	cmd.Stdin = strings.NewReader("Jan  5 14:30:00 event happened\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if strings.Contains(out, "ago") {
		t.Errorf("expected strftime output, got relative age: %q", out)
	}
	// The syslog timestamp should be replaced with formatted time.
	if !strings.Contains(out, "14:30:00") {
		t.Errorf("expected formatted time 14:30:00, got: %q", out)
	}
}

// TestRelativeMutualExclusionWithIncremental verifies R10.3: -r and -i
// together must print a usage error to stderr and exit non-zero.
func TestRelativeMutualExclusionWithIncremental(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-r", "-i")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code for -r -i combination")
	}
	if stderr.Len() == 0 {
		t.Error("expected usage message on stderr for -r -i combination")
	}
}

// TestRelativeMutualExclusionWithElapsed verifies R10.3: -r and -s
// together must print a usage error to stderr and exit non-zero.
func TestRelativeMutualExclusionWithElapsed(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "-r", "-s")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code for -r -s combination")
	}
	if stderr.Len() == 0 {
		t.Error("expected usage message on stderr for -r -s combination")
	}
}
