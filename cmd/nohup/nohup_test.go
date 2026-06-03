// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd095-nohup R2.1, R2.2, R2.3.
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

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?nohup\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("nohup"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnohup")
	if err != nil {
		t.Skip("reference binary not found")
	}

	norm := []testutils.NormalizeFunc{normalizeBinaryName}

	t.Run("missing_operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_args",
				Args:      []string{},
				ExitCode:  125,
				Normalize: norm,
			},
		})
	})

	t.Run("command_not_found", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "nonexistent",
				Args:      []string{"nonexistent_command_xyz"},
				ExitCode:  127,
				Normalize: norm,
			},
		})
	})

	t.Run("not_executable", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "dev_null",
				Args:      []string{"/dev/null"},
				ExitCode:  126,
				Normalize: norm,
			},
		})
	})

	t.Run("successful_command", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "true",
				Args:     []string{"true"},
				ExitCode: 0,
			},
		})
	})

	t.Run("exit_code_preservation", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "false_exit",
				Args:     []string{"false"},
				ExitCode: 1,
			},
		})
	})

	t.Run("custom_exit_code", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "exit_42",
				Args:     []string{"sh", "-c", "exit 42"},
				ExitCode: 42,
			},
		})
	})

	t.Run("args_passed", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "echo_multiple",
				Args:     []string{"echo", "hello", "world", "foo"},
				ExitCode: 0,
			},
		})
	})
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "echo", "hello")
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
		t.Fatal("nohup timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}
}
