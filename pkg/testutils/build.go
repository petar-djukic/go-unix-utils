// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles a Go package at pkgPath to a temporary binary and
// returns the path. Calls t.Fatal on build failure and t.Cleanup for removal.
func BuildBinary(t *testing.T, pkgPath string) string {
	t.Helper()

	tmpDir := t.TempDir()
	binName := filepath.Base(pkgPath)
	if binName == "." || binName == "" {
		binName = "testbin"
	}
	binPath := filepath.Join(tmpDir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, pkgPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build %s: %v\n%s", pkgPath, err, output)
	}

	t.Cleanup(func() {
		_ = os.Remove(binPath) // best-effort cleanup; t.TempDir handles directory removal
	})

	return binPath
}
