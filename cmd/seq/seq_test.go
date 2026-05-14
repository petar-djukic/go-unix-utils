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
		// R4.4: single argument (seq 5)
		{
			Name: "single-arg-5",
			Args: []string{"5"},
		},
		// R4.4: two arguments (seq 2 5)
		{
			Name: "two-args-2-5",
			Args: []string{"2", "5"},
		},
		// R4.4: three arguments (seq 1 2 10)
		{
			Name: "three-args-1-2-10",
			Args: []string{"1", "2", "10"},
		},
		// R4.4: descending sequence (seq 5 -1 1)
		{
			Name: "descending-5-neg1-1",
			Args: []string{"5", "-1", "1"},
		},
		// R4.4: floating-point sequence (seq 0.1 0.1 0.5)
		{
			Name: "float-0.1-0.1-0.5",
			Args: []string{"0.1", "0.1", "0.5"},
		},
		// R4.4: equal-width (seq -w 8 12)
		{
			Name: "equal-width-8-12",
			Args: []string{"-w", "8", "12"},
		},
		// R4.4: custom separator (seq -s ', ' 1 5)
		{
			Name: "separator-comma-1-5",
			Args: []string{"-s", ", ", "1", "5"},
		},
		// R4.4: format string (seq -f '%.2f' 1 3)
		{
			Name: "format-2f-1-3",
			Args: []string{"-f", "%.2f", "1", "3"},
		},
		// R4.4: invalid format error
		{
			Name:      "error-invalid-format",
			Args:      []string{"-f", "%d", "1", "3"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// R4.4: floating-point accumulation near endpoint
		{
			Name: "float-accum-0.8-0.1-1.2",
			Args: []string{"0.8", "0.1", "1.2"},
		},
		{
			Name: "float-accum-0.1-0.1-0.3",
			Args: []string{"0.1", "0.1", "0.3"},
		},
		// R4.4: descending float sequence
		{
			Name: "descending-float-1.0-neg0.1-0.0",
			Args: []string{"1.0", "-0.1", "0.0"},
		},
		// R4.4: equal-width with negative range
		{
			Name: "equal-width-neg5-5",
			Args: []string{"-w", "--", "-5", "5"},
		},
		// R4.4: single-element sequence (FIRST == LAST)
		{
			Name: "first-equals-last",
			Args: []string{"5", "5"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
