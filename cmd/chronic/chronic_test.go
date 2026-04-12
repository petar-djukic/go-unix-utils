// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("chronic")
	if err != nil {
		t.Skipf("reference binary chronic not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			// R1.1: suppress all output when command exits 0.
			Name: "success_suppressed",
			Args: []string{"echo", "hello"},
		},
		{
			// R1.1: exit code propagated on failure; false produces no output.
			Name:     "failure_exit_code",
			Args:     []string{"false"},
			ExitCode: 1,
		},
		{
			// R1.1: replay stdout and stderr on non-zero exit.
			Name:     "failure_output_replayed",
			Args:     []string{"sh", "-c", "echo out; echo err >&2; exit 1"},
			ExitCode: 1,
		},
		{
			// R1.2: -e triggers output when stderr has content, exits 2.
			Name:     "stderr_flag_triggers_on_stderr",
			Args:     []string{"-e", "sh", "-c", "echo err >&2"},
			ExitCode: 2,
		},
		{
			// R1.2: -e does not trigger when stderr is empty and exit 0.
			Name: "stderr_flag_silent_on_no_stderr",
			Args: []string{"-e", "echo", "hello"},
		},
		{
			// R1.3: -v uses labeled format on failure.
			Name:     "verbose_with_failure",
			Args:     []string{"-v", "false"},
			ExitCode: 1,
		},
		{
			// R1.3: -v still suppresses on success.
			Name: "verbose_suppressed_on_success",
			Args: []string{"-v", "echo", "hello"},
		},
		{
			// R2.1: exit code propagation with specific non-1 exit code.
			Name:     "exit_code_propagation_42",
			Args:     []string{"sh", "-c", "exit 42"},
			ExitCode: 42,
		},
		{
			// R2.1: exit code propagation preserves exit 0 on success.
			Name: "exit_code_zero_on_success",
			Args: []string{"true"},
		},
		{
			// R1.3 + R1.2: -v -e combined, stderr triggers labeled output.
			Name:     "verbose_stderr_combined",
			Args:     []string{"-v", "-e", "sh", "-c", "echo err >&2"},
			ExitCode: 2,
		},
		{
			// R1.3 + R2.1: -v with specific exit code, labeled format.
			Name:     "verbose_exit_code_propagation",
			Args:     []string{"-v", "sh", "-c", "echo out; echo err >&2; exit 3"},
			ExitCode: 3,
		},
		{
			// R1.2 + R2.1: -e with non-zero exit, stderr present.
			Name:     "stderr_flag_with_nonzero_exit",
			Args:     []string{"-e", "sh", "-c", "echo err >&2; exit 1"},
			ExitCode: 1,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
