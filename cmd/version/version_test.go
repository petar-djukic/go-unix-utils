// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/version binary (prd059-version R1.1-R1.4).

package main_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	t.Parallel()

	bin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantExit   int
		checkErr   bool // when true, check stderr contains something
	}{
		{
			// R1.1: no arguments prints version string and exits 0.
			name:       "no args prints version",
			args:       nil,
			wantStdout: "dev\n",
			wantExit:   0,
		},
		{
			// R1.4: --version prints version string and exits 0.
			name:       "dash dash version flag",
			args:       []string{"--version"},
			wantStdout: "dev\n",
			wantExit:   0,
		},
		{
			// R1.3 (task): --help prints usage to stdout and exits 0.
			name:     "dash dash help flag",
			args:     []string{"--help"},
			wantExit: 0,
		},
		{
			// R1.4: unknown flag prints error to stderr and exits 1.
			name:     "unknown flag exits 1",
			args:     []string{"--bogus"},
			wantExit: 1,
			checkErr: true,
		},
		{
			// R1.4: unknown short flag prints error to stderr and exits 1.
			name:     "unknown short flag exits 1",
			args:     []string{"-z"},
			wantExit: 1,
			checkErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(bin, tc.args...)
			stdout, err := cmd.Output()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("unexpected error running binary: %v", err)
				}
			}

			require.Equal(t, tc.wantExit, exitCode, "exit code mismatch")

			if tc.wantStdout != "" {
				require.Equal(t, tc.wantStdout, string(stdout), "stdout mismatch")
			}

			if tc.checkErr {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				stderr := string(exitErr.Stderr)
				require.True(t, len(strings.TrimSpace(stderr)) > 0, "expected non-empty stderr")
			}
		})
	}
}

func TestVersionHelpContainsUsage(t *testing.T) {
	t.Parallel()

	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--help")
	stdout, err := cmd.Output()
	require.NoError(t, err, "expected exit 0 for --help")
	require.Contains(t, string(stdout), "Usage:", "help output should contain Usage:")
}
