// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd097-who R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gwho")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?who`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("who"))
	})
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName}

	tests := []testutils.DiffTest{
		{Name: "no_args", ExitCode: 0},
		{Name: "heading", Args: []string{"-H"}, ExitCode: 0},
		{Name: "heading_long", Args: []string{"--heading"}, ExitCode: 0},
		{Name: "users", Args: []string{"-u"}, ExitCode: 0},
		{Name: "users_long", Args: []string{"--users"}, ExitCode: 0},
		{Name: "boot", Args: []string{"-b"}, ExitCode: 0},
		{Name: "boot_long", Args: []string{"--boot"}, ExitCode: 0},
		{Name: "count", Args: []string{"-q"}, ExitCode: 0},
		{Name: "count_long", Args: []string{"--count"}, ExitCode: 0},
		{Name: "heading_and_boot", Args: []string{"-H", "-b"}, ExitCode: 0},
		{Name: "heading_and_users", Args: []string{"-H", "-u"}, ExitCode: 0},
		{Name: "heading_boot_combined", Args: []string{"-Hb"}, ExitCode: 0},
		{Name: "heading_users_combined", Args: []string{"-Hu"}, ExitCode: 0},
		{Name: "unrecognized_option", Args: []string{"--foo"}, ExitCode: 1, Normalize: errNorm},
		{Name: "invalid_short_option", Args: []string{"-x"}, ExitCode: 1, Normalize: errNorm},
		{Name: "extra_operand", Args: []string{"a", "b", "c"}, ExitCode: 1, Normalize: errNorm},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
