// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd051-pwd R3.1-R3.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpwd")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?pwd`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("pwd"))
	})

	// R3.3: All differential tests set LC_ALL=C in the environment.
	lcEnv := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// R3.2: default invocation (physical mode)
		{
			Name: "default",
			Env:  lcEnv,
		},
		// R3.2: -P flag (physical mode)
		{
			Name: "physical-short",
			Args: []string{"-P"},
			Env:  lcEnv,
		},
		{
			Name: "physical-long",
			Args: []string{"--physical"},
			Env:  lcEnv,
		},
		// R3.2: -L flag (logical mode)
		{
			Name: "logical-short",
			Args: []string{"-L"},
			Env:  lcEnv,
		},
		{
			Name: "logical-long",
			Args: []string{"--logical"},
			Env:  lcEnv,
		},
		// R3.2: -L -P precedence (last wins)
		{
			Name: "logical-then-physical",
			Args: []string{"-L", "-P"},
			Env:  lcEnv,
		},
		{
			Name: "physical-then-logical",
			Args: []string{"-P", "-L"},
			Env:  lcEnv,
		},
		{
			Name: "combined-LP",
			Args: []string{"-LP"},
			Env:  lcEnv,
		},
		{
			Name: "combined-PL",
			Args: []string{"-PL"},
			Env:  lcEnv,
		},
		// R2.1: extra positional operand (gpwd warns but still prints and exits 0)
		{
			Name:      "extra-operand",
			Args:      []string{"foo"},
			Env:       lcEnv,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "extra-operand-after-dashdash",
			Args:      []string{"--", "foo"},
			Env:       lcEnv,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R2.2: unknown flags
		{
			Name:      "error-unknown-long-flag",
			Args:      []string{"--bogus"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "error-unknown-short-flag",
			Args:      []string{"-x"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// --help and --version (discard stdout since text differs)
		{
			Name:      "help",
			Args:      []string{"--help"},
			Env:       lcEnv,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name:      "version",
			Args:      []string{"--version"},
			Env:       lcEnv,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
