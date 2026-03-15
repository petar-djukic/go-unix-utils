// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd059-version R1.1-R1.5.
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name     string
		args     []string
		exitCode int
		stdout   string
		stderr   string
	}{
		{
			// R1.1: no arguments prints version string followed by newline.
			// R1.2: development build (no ldflags) prints "dev".
			name:     "no_args_prints_dev",
			args:     nil,
			exitCode: 0,
			stdout:   "dev\n",
		},
		{
			// R1.4: --version prints same output as no-argument invocation.
			name:     "dash_dash_version",
			args:     []string{"--version"},
			exitCode: 0,
			stdout:   "dev\n",
		},
		{
			// R1.4: -v prints same output as no-argument invocation.
			name:     "dash_v",
			args:     []string{"-v"},
			exitCode: 0,
			stdout:   "dev\n",
		},
		{
			// R1.4: unknown flag prints usage to stderr and exits 2.
			name:     "unknown_flag_exits_2",
			args:     []string{"--bogus"},
			exitCode: 2,
			stderr:   "Usage:",
		},
		{
			// R1.4: extra arguments print usage to stderr and exits 2.
			name:     "extra_args_exits_2",
			args:     []string{"--version", "extra"},
			exitCode: 2,
			stderr:   "Usage:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(goBin, tc.args...)
			out, err := cmd.CombinedOutput()

			actualExit := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					actualExit = exitErr.ExitCode()
				} else {
					t.Fatalf("unexpected error running binary: %v", err)
				}
			}

			if actualExit != tc.exitCode {
				t.Errorf("exit code: got %d, want %d", actualExit, tc.exitCode)
			}

			output := string(out)

			if tc.stdout != "" && !strings.Contains(output, tc.stdout) {
				t.Errorf("stdout: got %q, want to contain %q", output, tc.stdout)
			}

			if tc.stderr != "" && !strings.Contains(output, tc.stderr) {
				t.Errorf("stderr: got %q, want to contain %q", output, tc.stderr)
			}
		})
	}
}

// TestVersionExported verifies R1.5: the pkg/version.Version variable is
// importable from another package and returns the expected default value.
func TestVersionExported(t *testing.T) {
	t.Parallel()

	got := version.Version
	if got != "dev" {
		t.Errorf("version.Version = %q, want %q", got, "dev")
	}
}
