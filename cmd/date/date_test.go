// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies date output parity against gdate (GNU coreutils).
// Uses -d @EPOCH for deterministic output across both binaries.
// R1.1: default format. R1.2: +FORMAT. R1.3: strftime specs. R1.4: GNU extensions.
// R2.1: -d/--date string parsing. R2.2: epoch @timestamps. R2.3: ISO 8601. R2.4: invalid date errors.
// R3.1: -u/--utc/--universal UTC mode. R3.2: -r/--reference file. R3.3: missing file error. R3.4: stdout only.
// R4.1: exit 0 on success. R4.2: exit 1 on error. R4.3: differential tests. R4.4: comprehensive coverage.
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

	// Create a temp file with known modification time for -r tests.
	refFile := createRefFile(t)

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
		// R3.1: -u/--utc/--universal display in UTC.
		{
			Name: "utc-short-flag",
			Args: []string{"-u", "-d", "@1700000000", "+%Z"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "utc-long-flag",
			Args: []string{"--utc", "-d", "@1700000000", "+%Z"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "utc-universal-flag",
			Args: []string{"--universal", "-d", "@1700000000", "+%Z"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "utc-epoch-seconds",
			Args: []string{"-u", "-d", "@1700000000", "+%s"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "utc-full-datetime",
			Args: []string{"-u", "-d", "@1700000000", "+%Y-%m-%d %H:%M:%S %Z"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: -r FILE / --reference=FILE display modification time.
		{
			Name: "ref-file-short-flag",
			Args: []string{"-r", refFile, "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "ref-file-long-flag",
			Args: []string{"--reference=" + refFile, "+%Y-%m-%d %H:%M:%S"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "ref-file-with-utc",
			Args: []string{"-u", "-r", refFile, "+%Y-%m-%d %H:%M:%S %Z"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "ref-file-epoch-format",
			Args: []string{"-r", refFile, "+%s"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R3.3: missing reference file produces error and exit 1.
		{
			Name:      "ref-file-missing",
			Args:      []string{"-r", "/nonexistent/file/path"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: errNorms,
		},
		{
			Name:      "ref-file-missing-long-flag",
			Args:      []string{"--reference=/nonexistent/file/path"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: errNorms,
		},
		// R4.1: exit 0 on successful display. R4.3: deterministic via -d @EPOCH.
		{
			Name: "exit-0-default-format",
			Args: []string{"-d", "@0"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "exit-0-custom-format",
			Args: []string{"-d", "@0", "+%Y"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R4.2: exit 1 on invalid date or missing file.
		{
			Name:      "exit-1-invalid-date",
			Args:      []string{"-d", "xyz123"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: errNorms,
		},
		{
			Name:      "exit-1-missing-ref-file",
			Args:      []string{"-r", "/no/such/file"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C", "TZ=UTC"},
			Normalize: errNorms,
		},
		// R4.4: comprehensive format coverage — week specifiers.
		{
			Name: "format-week-sunday-U",
			Args: []string{"-d", "@1700000000", "+%U"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "format-week-monday-W",
			Args: []string{"-d", "@1700000000", "+%W"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "format-iso-week-V",
			Args: []string{"-d", "@1700000000", "+%V"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "format-iso-year-G",
			Args: []string{"-d", "@1700000000", "+%G"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "format-iso-short-year-g",
			Args: []string{"-d", "@1700000000", "+%g"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R4.4: composite format specifiers.
		{
			Name: "format-composite-c",
			Args: []string{"-d", "@1700000000", "+%c"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "format-composite-x",
			Args: []string{"-d", "@1700000000", "+%x"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "format-composite-X",
			Args: []string{"-d", "@1700000000", "+%X"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R4.4: month abbreviation alias %h.
		{
			Name: "format-month-abbrev-h",
			Args: []string{"-d", "@1700000000", "+%h"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R4.4: space-padded 12-hour %l.
		{
			Name: "format-12hour-space-padded",
			Args: []string{"-d", "@0", "+%l"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R4.4: escape sequences %n (newline) and %t (tab).
		{
			Name: "format-tab-escape",
			Args: []string{"-d", "@0", "+%H%t%M"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "format-newline-escape",
			Args: []string{"-d", "@0", "+%H%n%M"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		// R4.4: combined -u with -d and format.
		{
			Name: "combined-utc-date-format",
			Args: []string{"-u", "-d", "@1700000000", "+%Y-%m-%d %H:%M:%S %Z"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.4: padding modifiers on additional specifiers.
		{
			Name: "pad-no-padding-hour",
			Args: []string{"-d", "@3600", "+%-H"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "pad-space-padding-month",
			Args: []string{"-d", "@0", "+%_m"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
		{
			Name: "pad-zero-padding-minute",
			Args: []string{"-d", "@300", "+%0M"},
			Env:  []string{"LC_ALL=C", "TZ=UTC"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createRefFile creates a temporary file with a fixed modification time
// for -r/--reference differential tests.
func createRefFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "reffile")
	if err := os.WriteFile(p, []byte("test"), 0o644); err != nil {
		t.Fatalf("creating ref file: %v", err)
	}
	// Set a fixed mtime so both binaries produce the same output.
	mtime := time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatalf("setting ref file mtime: %v", err)
	}
	return p
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
