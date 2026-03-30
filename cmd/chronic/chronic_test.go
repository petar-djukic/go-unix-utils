// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("chronic")
	if err != nil {
		t.Skipf("reference binary chronic not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: suppress output on success.
		{
			Name:     "success_suppressed",
			Args:     []string{"echo", "hello"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: suppress even with stderr on success (without -e).
		{
			Name:     "success_stderr_suppressed",
			Args:     []string{"sh", "-c", "echo warn >&2"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1, R2.1: show output on failure, exit code matches command.
		{
			Name:     "failure_exit_code",
			Args:     []string{"false"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.1: stdout captured and replayed on failure.
		{
			Name:     "failure_with_stdout",
			Args:     []string{"sh", "-c", "echo output; exit 1"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.1: stderr captured and replayed on failure.
		{
			Name:     "failure_with_stderr",
			Args:     []string{"sh", "-c", "echo err >&2; exit 2"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 2,
		},
		// R1.2: -e triggers output and exits 2 when stderr non-empty on exit 0.
		{
			Name:     "stderr_flag_triggers_on_stderr",
			Args:     []string{"-e", "sh", "-c", "echo warn >&2"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 2,
		},
		// R1.2: -e does not trigger when stderr is empty and exit 0.
		{
			Name:     "stderr_flag_silent_when_no_stderr",
			Args:     []string{"-e", "true"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: -v adds headers around output on failure.
		{
			Name:     "verbose_on_failure",
			Args:     []string{"-v", "sh", "-c", "echo out; echo err >&2; exit 1"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.3: -v suppresses output on success (no headers shown).
		{
			Name:     "verbose_on_success",
			Args:     []string{"-v", "true"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2 + R1.3: combined -v -e flags.
		{
			Name:     "verbose_and_stderr_combined",
			Args:     []string{"-v", "-e", "sh", "-c", "echo warn >&2"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 2,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
