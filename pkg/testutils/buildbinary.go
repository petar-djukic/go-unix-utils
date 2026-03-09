// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-testutils R4.1–R4.2 (task requirements).

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at pkgPath to a temporary binary and returns
// the path. The binary is removed via t.Cleanup after the test completes. R4.1.
// If compilation fails, t.Fatal is called with the go build stderr output. R4.2.
func BuildBinary(t *testing.T, pkgPath string) string {
	t.Helper()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "test-binary")

	cmd := exec.Command("go", "build", "-o", binPath, pkgPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("BuildBinary: go build %s: %v", pkgPath, err)
	}

	t.Cleanup(func() {
		os.Remove(binPath) // best-effort cleanup, TempDir removal handles this too
	})

	return binPath
}
