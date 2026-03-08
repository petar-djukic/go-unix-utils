// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at pkgPath to a temporary binary and
// returns its path. The binary is cleaned up via t.Cleanup.
// R4: used by cmd/ test files to build their utility before running differential tests.
func BuildBinary(t *testing.T, pkgPath string) string {
	t.Helper()

	tmpDir := t.TempDir()
	binName := filepath.Base(pkgPath)
	if binName == "." {
		// When pkgPath is ".", use the current directory name.
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("BuildBinary: get working directory: %v", err)
		}
		binName = filepath.Base(wd)
	}

	binPath := filepath.Join(tmpDir, binName)

	cmd := exec.Command("go", "build", "-o", binPath, pkgPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", pkgPath, err, output)
	}

	return binPath
}
