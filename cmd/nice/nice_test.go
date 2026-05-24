// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd094-nice R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"context"
	"io"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?nice\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("nice"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnice")
	if err != nil {
		t.Skip("reference binary not found")
	}

	t.Run("no_command_prints_niceness", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "no_args",
				Args:     []string{},
				ExitCode: 0,
			},
		})
	})

	t.Run("default_adjustment", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "echo_with_default",
				Args:     []string{"echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("custom_adjustment_n5", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "n5_echo",
				Args:     []string{"-n", "5", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("custom_adjustment_n0", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "n0_echo",
				Args:     []string{"-n", "0", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("adjustment_long_flag", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "adjustment_eq_3",
				Args:     []string{"--adjustment=3", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("adjustment_long_separate", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "adjustment_sep_3",
				Args:     []string{"--adjustment", "3", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("combined_short", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "n7_combined",
				Args:     []string{"-n7", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("numeric_shorthand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "dash_5",
				Args:     []string{"-5", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("command_not_found", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "nonexistent",
				Args:      []string{"nonexistent_command_xyz"},
				ExitCode:  127,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("command_preserves_exit_code", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "false_exit",
				Args:     []string{"false"},
				ExitCode: 1,
			},
		})
	})

	t.Run("command_args_passed", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "echo_multiple",
				Args:     []string{"echo", "hello", "world", "foo"},
				ExitCode: 0,
			},
		})
	})

	t.Run("double_dash", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "double_dash_echo",
				Args:     []string{"--", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("large_positive_adjustment", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "n100",
				Args:     []string{"-n", "100", "echo", "hello"},
				ExitCode: 0,
			},
		})
	})

	t.Run("invalid_adjustment", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_adjust",
				Args:      []string{"-n", "abc", "echo", "hello"},
				ExitCode:  125,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "--help")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("nice timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

