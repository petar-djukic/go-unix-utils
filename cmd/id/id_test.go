// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd041-id R4.1, R4.2, R4.3.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var normBinaryName testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gid:"), []byte("id:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gid")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{Name: "default_no_flags"},
		{Name: "flag_u", Args: []string{"-u"}},
		{Name: "flag_long_user", Args: []string{"--user"}},
		{Name: "flag_g", Args: []string{"-g"}},
		{Name: "flag_long_group", Args: []string{"--group"}},
		{Name: "flag_G", Args: []string{"-G"}},
		{Name: "flag_long_groups", Args: []string{"--groups"}},
		{Name: "conflict_ug", Args: []string{"-ug"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{normBinaryName}},
		{Name: "conflict_u_G", Args: []string{"-u", "-G"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{normBinaryName}},
		{Name: "conflict_g_G", Args: []string{"-g", "-G"}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{normBinaryName}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
