// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd042-whoami R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"os/user"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gwhoami")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?whoami`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("whoami"))
	})
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName}

	tests := []testutils.DiffTest{
		{Name: "no_args", ExitCode: 0},
		{Name: "extra_operand", Args: []string{"extraarg"}, ExitCode: 1, Normalize: errNorm},
		{Name: "unknown_flag", Args: []string{"--unknown"}, ExitCode: 1, Normalize: errNorm},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestEffectiveUsername(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, err := user.Current()
	if err != nil {
		t.Fatalf("cannot get current user: %v", err)
	}

	got := string(out)
	want := u.Username + "\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
