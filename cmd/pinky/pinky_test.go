// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd098-pinky R3.1, R3.2, R3.3.
package main

import (
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gpinky")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?pinky`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("pinky"))
	})
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName}

	user := os.Getenv("USER")
	if user == "" {
		t.Skip("USER environment variable not set")
	}

	tests := []testutils.DiffTest{
		{Name: "no_args", ExitCode: 0},
		{Name: "short_no_header", Args: []string{"-f"}, ExitCode: 0},
		{Name: "explicit_short", Args: []string{"-s"}, ExitCode: 0},
		{Name: "short_with_user", Args: []string{user}, ExitCode: 0},
		{Name: "long_format", Args: []string{"-l", user}, ExitCode: 0},
		{Name: "long_no_home_shell", Args: []string{"-lb", user}, ExitCode: 0},
		{Name: "long_suppress_all", Args: []string{"-lbhp", user}, ExitCode: 0},
		{Name: "long_no_user", Args: []string{"-l"}, ExitCode: 1, Normalize: errNorm},
		{Name: "unrecognized_option", Args: []string{"--foo"}, ExitCode: 1, Normalize: errNorm},
		{Name: "invalid_short_option", Args: []string{"-x"}, ExitCode: 1, Normalize: errNorm},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
