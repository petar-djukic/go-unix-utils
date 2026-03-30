// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// users_test.go implements differential tests for cmd/users against gusers.

package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gusers")
	if err != nil {
		t.Skip("reference binary gusers not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: prints logged-in usernames sorted and space-separated.
			Name: "users_list",
			Args: []string{},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
