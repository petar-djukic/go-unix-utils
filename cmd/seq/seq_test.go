// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?seq`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("seq"))
	})

	tests := []testutils.DiffTest{
		// R4.2: non-numeric argument
		{
			Name:      "error-non-numeric",
			Args:      []string{"abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R4.2: missing operand
		{
			Name:      "error-no-args",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R4.2: extra operand
		{
			Name:      "error-extra-operand",
			Args:      []string{"1", "2", "3", "4"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R4.2: zero step
		{
			Name:      "error-zero-step",
			Args:      []string{"1", "0", "10"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R4.2: NaN argument
		{
			Name:      "error-nan",
			Args:      []string{"nan"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R4.1: successful sequence exits 0
		{
			Name: "success-single-arg",
			Args: []string{"3"},
		},
		// R4.1: empty sequence exits 0
		{
			Name: "success-empty-sequence",
			Args: []string{"5", "1", "3"},
		},
		// R3.4: -f and -w are mutually exclusive
		{
			Name:      "error-format-with-equal-width",
			Args:      []string{"-f", "%.2f", "-w", "1", "3"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// --help and --version (discard stdout since text differs)
		{
			Name:      "help",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name:      "version",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
