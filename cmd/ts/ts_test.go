// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
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

// TestDiff verifies cmd/ts against the moreutils reference binary ts.
// Implements prd004-ts R9.1-R9.2.
// R9.1: uses TimestampNormalizer for wall-clock timestamp comparison.
// R9.2: covers default format, custom format, subsecond extensions (R2.3, R2.4),
// -i incremental mode (R3.1-R3.4), -s elapsed mode (R4.1-R4.3),
// -m monotonic mode (R5.1-R5.3),
// empty stdin, partial last line, additional strftime specifiers (R2.2).
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
		// R1.1, R1.6: empty stdin produces no output and exits 0.
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
		// R4.1, R4.2: elapsed mode with empty stdin.
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
