// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at dir and returns the path to the
// compiled binary. The binary is placed in a temporary directory that is
// cleaned up when the test completes.
// R4: compile within the Go module, return binary path, fatal on failure.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	name := resolvePackageName(dir)
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, name)
	cmd := exec.Command("go", "build", "-o", binPath, dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", dir, err, output)
	}
	return binPath
}

func resolvePackageName(dir string) string {
	if dir == "." {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "testbin"
		}
		return filepath.Base(abs)
	}
	return filepath.Base(dir)
}
