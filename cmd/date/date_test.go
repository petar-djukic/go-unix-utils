// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/date — differential tests against gdate.
// Covers prd060-date R1.1-R1.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdate")
	if err != nil {
		t.Skip("reference binary gdate not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: default format with fixed epoch
		{
			Name:     "default_format_epoch",
			Args:     []string{"-d", "@0"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.2: custom +FORMAT
		{
			Name:     "custom_format_ymd_hms",
			Args:     []string{"-d", "@0", "+%Y-%m-%d %H:%M:%S"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %s seconds since epoch
		{
			Name:     "epoch_seconds",
			Args:     []string{"-u", "-d", "@1700000000", "+%s"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: %A full weekday name
		{
			Name:     "weekday_name",
			Args:     []string{"-d", "@0", "+%A"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %B full month name
		{
			Name:     "month_name",
			Args:     []string{"-d", "@0", "+%B"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %j day of year
		{
			Name:     "day_of_year",
			Args:     []string{"-d", "@0", "+%j"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %u day of week (1-7, Monday=1)
		{
			Name:     "day_of_week_u",
			Args:     []string{"-d", "@0", "+%u"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %w day of week (0-6, Sunday=0)
		{
			Name:     "day_of_week_w",
			Args:     []string{"-d", "@0", "+%w"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %N nanoseconds
		{
			Name:     "nanoseconds",
			Args:     []string{"-d", "@0", "+%N"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %Z timezone name
		{
			Name:     "timezone_utc",
			Args:     []string{"-u", "-d", "@0", "+%Z"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.4: %P lowercase am/pm
		{
			Name:     "lowercase_ampm",
			Args:     []string{"-d", "@0", "+%P"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.4: %-d no padding modifier
		{
			Name:     "no_pad_day",
			Args:     []string{"-d", "@0", "+%-d"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.4: %_m space padding modifier
		{
			Name:     "space_pad_month",
			Args:     []string{"-d", "@0", "+%_m"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.4: %0e zero padding modifier (overrides space default)
		{
			Name:     "zero_pad_day_e",
			Args:     []string{"-d", "@0", "+%0e"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: combined format with multiple specs
		{
			Name:     "combined_format",
			Args:     []string{"-d", "@1234567890", "+%Y/%m/%d %H:%M:%S %A"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
		// R1.3: %I 12-hour clock and %p AM/PM
		{
			Name:     "twelve_hour_format",
			Args:     []string{"-d", "@1234567890", "+%I:%M:%S %p"},
			Env:      []string{"LC_ALL=C", "TZ=UTC"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
