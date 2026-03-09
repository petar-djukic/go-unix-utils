// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// BuildBinary compiles a Go cmd/ package to a temporary binary for testing.
// Implements prd001-testutils R2.1.
package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at pkgPath to a temporary binary and
// returns the path. Calls t.Fatal on build failure. Registers t.Cleanup to
// remove the binary. (prd001-testutils R2.1)
func BuildBinary(t *testing.T, pkgPath string) string {
	t.Helper()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "test-binary")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = pkgPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary(%q) failed: %v\n%s", pkgPath, err, out)
	}

	t.Cleanup(func() {
		os.Remove(binPath) // best-effort cleanup; TempDir removal handles it too
	})

	return binPath
}
