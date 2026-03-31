// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/false against gfalse (GNU coreutils).
//
// Covers prd014-false R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R4.1, R4.2, R4.3.
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardStdout blanks all stdout so that tests comparing --help or --version
// check only exit code and stderr. GNU false's output includes full binary paths,
// OSC hyperlinks, and boilerplate that cannot be reproduced exactly.
func discardStdout(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfalse")
	if err != nil {
		t.Skip("reference binary gfalse not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments — exit 1, no output
		{
			Name:     "R1.1_no_args",
			Args:     []string{},
			ExitCode: 1,
		},
		// R1.2, R4.2: arbitrary arguments ignored — exit 1, no output
		{
			Name:     "R1.2_arbitrary_args",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 1,
		},
		// R1.2: single unrecognized flag ignored — exit 1, no output
		{
			Name:     "R1.2_unrecognized_flag",
			Args:     []string{"--unknown"},
			ExitCode: 1,
		},
		// R2.1, R4.2: --help — tested differentially for exit code.
		// GNU gfalse --help exits 1 (unlike gtrue), stdout discarded.
		{
			Name:      "R2.1_help",
			Args:      []string{"--help"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		// R2.2, R4.2: --version — tested differentially for exit code.
		{
			Name:      "R2.2_version",
			Args:      []string{"--version"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		// R1.2: --version followed by other args — exit 1
		{
			Name:      "R2.2_version_with_extra_args",
			Args:      []string{"--version", "--extra"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		// R1.3: multiple flags ignored — exit 1
		{
			Name:     "R1.3_multiple_flags",
			Args:     []string{"-x", "-y", "-z"},
			ExitCode: 1,
		},
		// R4.3: --help as non-first arg is ignored — no output, exit 1
		{
			Name:     "R4.3_help_not_first",
			Args:     []string{"foo", "--help"},
			ExitCode: 1,
		},
		// R4.3: --version as non-first arg is ignored — no output, exit 1
		{
			Name:     "R4.3_version_not_first",
			Args:     []string{"foo", "--version"},
			ExitCode: 1,
		},
		// R4.3: stdin provided but not read — exit 1, no output
		{
			Name:     "R4.3_stdin_ignored",
			Args:     []string{},
			Stdin:    []byte("input that should be ignored\n"),
			ExitCode: 1,
		},
		// R4.1: single dash argument ignored — exit 1
		{
			Name:     "R4.1_single_dash",
			Args:     []string{"-"},
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies --help prints usage to stdout (R2.1).
// GNU gfalse --help exits 1 (unlike gtrue), so we check output content only.
func TestHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--help")
	out, _ := cmd.CombinedOutput() // exit code 1 is expected

	stdout := string(out)
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("--help output missing Usage header: %q", stdout)
	}
	if !strings.Contains(stdout, "false") {
		t.Errorf("--help output missing 'false': %q", stdout)
	}
}

// TestVersion verifies --version prints version info to stdout (R2.2).
// GNU gfalse --version exits 1, so we check output content only.
func TestVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--version")
	out, _ := cmd.CombinedOutput() // exit code 1 is expected

	stdout := string(out)
	if !strings.Contains(stdout, "false") {
		t.Errorf("--version output missing 'false': %q", stdout)
	}
	if !strings.Contains(stdout, "go-unix-utils") {
		t.Errorf("--version output missing 'go-unix-utils': %q", stdout)
	}
}
