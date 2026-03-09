// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at pkgPath to a temporary binary and returns
// the path. The binary is removed via t.Cleanup after the test completes. R1.4 (task).
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
