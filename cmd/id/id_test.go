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

var normNoSuchUser testutils.NormalizeFunc = func(b []byte) []byte {
	marker := []byte("no such user")
	idx := bytes.Index(b, marker)
	if idx < 0 {
		return b
	}
	end := idx + len(marker)
	rest := b[end:]
	nlIdx := bytes.IndexByte(rest, '\n')
	if nlIdx >= 0 {
		return append(b[:end], rest[nlIdx:]...)
	}
	return b[:end]
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
		{Name: "conflict_ug", Args: []string{"-ug"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normBinaryName}},
		{Name: "conflict_u_G", Args: []string{"-u", "-G"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normBinaryName}},
		{Name: "conflict_g_G", Args: []string{"-g", "-G"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normBinaryName}},

		// R3.1: -n/--name modifier flag
		{Name: "flag_un", Args: []string{"-un"}},
		{Name: "flag_gn", Args: []string{"-gn"}},
		{Name: "flag_Gn", Args: []string{"-Gn"}},
		{Name: "flag_long_name_user", Args: []string{"--name", "--user"}},
		{Name: "flag_long_name_group", Args: []string{"--name", "--group"}},
		{Name: "flag_long_name_groups", Args: []string{"--name", "--groups"}},
		{Name: "error_n_alone", Args: []string{"-n"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normBinaryName}},

		// R3.2: -r/--real modifier flag
		{Name: "flag_ur", Args: []string{"-ur"}},
		{Name: "flag_gr", Args: []string{"-gr"}},
		{Name: "flag_rG", Args: []string{"-rG"}},
		{Name: "flag_unr", Args: []string{"-unr"}},
		{Name: "flag_gnr", Args: []string{"-gnr"}},
		{Name: "flag_long_real_user", Args: []string{"--real", "--user"}},
		{Name: "flag_long_real_group", Args: []string{"--real", "--group"}},
		{Name: "error_r_alone", Args: []string{"-r"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normBinaryName}},

		// R3.3: USER operand
		{Name: "user_root", Args: []string{"root"}},
		{Name: "user_root_u", Args: []string{"-u", "root"}},
		{Name: "user_root_un", Args: []string{"-un", "root"}},
		{Name: "user_root_g", Args: []string{"-g", "root"}},
		{Name: "user_root_gn", Args: []string{"-gn", "root"}},
		{Name: "user_root_G", Args: []string{"-G", "root"}},
		{Name: "user_root_Gn", Args: []string{"-Gn", "root"}},
		{Name: "error_nonexistent_user", Args: []string{"nonexistentuser12345"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normBinaryName, normNoSuchUser}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
