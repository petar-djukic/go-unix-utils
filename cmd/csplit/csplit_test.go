// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/csplit via differential testing against gcsplit.
// Implements srd068-csplit R4.3, R4.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcsplit")
	if err != nil {
		t.Skipf("reference binary gcsplit not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
