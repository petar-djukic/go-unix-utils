// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies date output parity against gdate (GNU coreutils).
// Uses -d @EPOCH for deterministic output across both binaries.
// R1.1: default format. R1.2: +FORMAT. R1.3: strftime specs. R1.4: GNU extensions.
// R2.1: -d/--date string parsing. R2.2: epoch @timestamps. R2.3: ISO 8601. R2.4: invalid date errors.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdate")
	if err != nil {
		t.Skip("reference binary gdate not in PATH")
	}
	// stderrNorm replaces the reference binary name with "date" so error
	// messages match between gdate and our binary.
	stderrNorm := makeBinaryNameNormalizer(refBin)
	errNorms := []testutils.NormalizeFunc{stderrNorm}
	tests := []testutils.DiffTest{
		// R1.1: default format with fixed epoch.
		{
			Name: "default-format-epoch-0",
			Args: []string{"-d", "@0"},
		},
		{
			Name: "default-format-epoch-large",
			Args: []string{"-d", "@1700000000"},
		},
		// R1.2: custom +FORMAT.
		{
			Name: "format-ymd",
			Args: []string{"-d", "@0", "+%Y-%m-%d"},
		},
		{
			Name: "format-hms",
			Args: []string{"-d", "@0", "+%H:%M:%S"},
		},
		{
			Name: "format-combined-datetime",
			Args: []string{"-d", "@1700000000", "+%Y-%m-%d %H:%M:%S"},
		},
		{
			Name: "format-literal-percent",
			Args: []string{"-d", "@0", "+100%%"},
		},
		// R1.3: strftime conversion specifications.
		{
			Name: "format-year",
			Args: []string{"-d", "@1700000000", "+%Y"},
		},
		{
			Name: "format-month-number",
			Args: []string{"-d", "@1700000000", "+%m"},
		},
		{
			Name: "format-day",
			Args: []string{"-d", "@1700000000", "+%d"},
		},
		{
			Name: "format-hour",
			Args: []string{"-d", "@1700000000", "+%H"},
		},
		{
			Name: "format-minute",
			Args: []string{"-d", "@1700000000", "+%M"},
		},
		{
			Name: "format-second",
			Args: []string{"-d", "@1700000000", "+%S"},
		},
		{
			Name: "format-epoch-seconds",
			Args: []string{"-d", "@1700000000", "+%s"},
		},
		{
			Name: "format-nanoseconds",
			Args: []string{"-d", "@0", "+%N"},
		},
		{
			Name: "format-weekday-full",
			Args: []string{"-d", "@0", "+%A"},
		},
		{
			Name: "format-weekday-abbrev",
			Args: []string{"-d", "@0", "+%a"},
		},
		{
			Name: "format-month-full",
			Args: []string{"-d", "@0", "+%B"},
		},
		{
			Name: "format-month-abbrev",
			Args: []string{"-d", "@0", "+%b"},
		},
		{
			Name: "format-day-of-year",
			Args: []string{"-d", "@1700000000", "+%j"},
		},
		{
			Name: "format-day-of-week-u",
			Args: []string{"-d", "@0", "+%u"},
		},
		{
			Name: "format-day-of-week-w",
			Args: []string{"-d", "@0", "+%w"},
		},
		{
			Name: "format-timezone-name",
			Args: []string{"-d", "@0", "+%Z"},
		},
		{
			Name: "format-timezone-offset",
			Args: []string{"-d", "@0", "+%z"},
		},
		{
			Name: "format-ampm-upper",
			Args: []string{"-d", "@0", "+%p"},
		},
		{
			Name: "format-12hour",
			Args: []string{"-d", "@0", "+%I"},
		},
		{
			Name: "format-composite-T",
			Args: []string{"-d", "@1700000000", "+%T"},
		},
		{
			Name: "format-composite-F",
			Args: []string{"-d", "@1700000000", "+%F"},
		},
		{
			Name: "format-composite-D",
			Args: []string{"-d", "@1700000000", "+%D"},
		},
		{
			Name: "format-composite-R",
			Args: []string{"-d", "@1700000000", "+%R"},
		},
		{
			Name: "format-composite-r",
			Args: []string{"-d", "@1700000000", "+%r"},
		},
		{
			Name: "format-space-day",
			Args: []string{"-d", "@0", "+%e"},
		},
		{
			Name: "format-space-hour",
			Args: []string{"-d", "@0", "+%k"},
		},
		{
			Name: "format-century",
			Args: []string{"-d", "@1700000000", "+%C"},
		},
		{
			Name: "format-short-year",
			Args: []string{"-d", "@1700000000", "+%y"},
		},
		{
			Name: "format-all-specs",
			Args: []string{"-d", "@1700000000", "+%Y %m %d %H %M %S %s %A %B %j %u %w %N"},
		},
		// R1.4: GNU extensions — %P and padding modifiers.
		{
			Name: "format-ampm-lower",
			Args: []string{"-d", "@0", "+%P"},
		},
		{
			Name: "format-ampm-lower-pm",
			Args: []string{"-d", "@1700000000", "+%P"},
		},
		{
			Name: "pad-no-padding-day",
			Args: []string{"-d", "@86400", "+%-d"},
		},
		{
			Name: "pad-space-padding-day",
			Args: []string{"-d", "@86400", "+%_d"},
		},
		{
			Name: "pad-zero-padding-day",
			Args: []string{"-d", "@86400", "+%0d"},
		},
		{
			Name: "pad-no-padding-month",
			Args: []string{"-d", "@0", "+%-m"},
		},
		{
			Name: "pad-space-padding-hour",
			Args: []string{"-d", "@0", "+%_H"},
		},
		{
			Name: "pad-zero-padding-space-day",
			Args: []string{"-d", "@86400", "+%0e"},
		},
		{
			Name: "pad-no-padding-minute",
			Args: []string{"-d", "@300", "+%-M"},
		},
		// R2.1: -d STRING and --date=STRING display the described date.
		{
			Name: "date-d-epoch-format",
			Args: []string{"-d", "@1234567890", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "date-long-date-flag",
			Args: []string{"--date=@1234567890", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "date-long-date-epoch-0",
			Args: []string{"--date=@0", "+%s"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R2.2: epoch timestamps prefixed with @.
		{
			Name: "epoch-at-zero",
			Args: []string{"-d", "@0", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "epoch-at-large",
			Args: []string{"-d", "@1700000000", "+%s"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "epoch-at-negative",
			Args: []string{"-d", "@-1", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R2.3: ISO 8601 date strings.
		{
			Name: "iso-datetime-space",
			Args: []string{"-d", "2024-01-15 10:30:00", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "iso-datetime-t-separator",
			Args: []string{"-d", "2024-01-15T10:30:00", "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "iso-date-only",
			Args: []string{"-d", "2024-01-15", "+%Y-%m-%d"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R2.4: invalid date string produces error and exit 1.
		{
			Name:      "invalid-date-string",
			Args:      []string{"-d", "not-a-date"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: errNorms,
		},
		{
			Name:      "invalid-date-long-flag",
			Args:      []string{"--date=garbage"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: errNorms,
		},
		{
			Name:      "invalid-epoch-nonnumeric",
			Args:      []string{"-d", "@abc"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: errNorms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// makeBinaryNameNormalizer returns a NormalizeFunc that replaces the reference
// binary path with "date" so stderr error messages match between gdate and
// our binary.
func makeBinaryNameNormalizer(refBin string) testutils.NormalizeFunc {
	refBase := filepath.Base(refBin)
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(progName))
		b = bytes.ReplaceAll(b, []byte(refBase), []byte(progName))
		return b
	}
}
