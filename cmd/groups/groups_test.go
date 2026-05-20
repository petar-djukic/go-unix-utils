// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd043-groups R3.1, R3.2, R3.3.
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
	refBin, err := exec.LookPath("ggroups")
	if err != nil {
		t.Skip("reference binary not found")
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}
	username := currentUser.Username

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?groups`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("groups"))
	})
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName}

	tests := []testutils.DiffTest{
		{
			Name: "no_args_current_user",
		},
		{
			Name: "single_named_user",
			Args: []string{username},
		},
		{
			Name: "single_named_user_root",
			Args: []string{"root"},
		},
		{
			Name: "multiple_named_users",
			Args: []string{"root", username},
		},
		{
			Name:      "nonexistent_user",
			Args:      []string{"nosuchuser_xyzzy"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "mixed_valid_invalid_users",
			Args:      []string{username, "nosuchuser_xyzzy", "root"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name: "user_colon_prefix_single",
			Args: []string{username},
		},
		{
			Name: "user_colon_prefix_multiple",
			Args: []string{"root", username},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
