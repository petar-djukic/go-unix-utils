// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gptx")
	if err != nil {
		t.Skipf("reference binary gptx not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
