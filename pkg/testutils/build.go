// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at pkgDir (relative to the caller's
// working directory, typically ".") and returns the path to the compiled
// binary. The binary is placed in t.TempDir() and cleaned up automatically
// via t.Cleanup.
//
// BuildBinary must be called from within the Go module (use "." as pkgDir
// when the test file lives in the cmd/ package being tested). It must not
// create Go files outside the module boundary.
//
// Implements prd001-testutils R1.3 (via task requirements).
func BuildBinary(t *testing.T, pkgDir string) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "testbin")

	cmd := exec.Command("go", "build", "-o", binPath, pkgDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", pkgDir, err, output)
	}

	return binPath
}
