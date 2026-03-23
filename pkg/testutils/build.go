// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R2.5 (BuildBinary).

package testutils

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// BuildBinary compiles the Go package at dir into a temporary binary and
// returns the path to the compiled binary. The binary is placed in a
// directory created by t.TempDir(), so cleanup is automatic.
//
// R2.5: compile cmd/ package, return binary path, auto-cleanup via t.TempDir.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()

	binName := "binary"
	if runtime.GOOS == "windows" {
		binName = "binary.exe"
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, binName)

	cmd := exec.Command("go", "build", "-o", outPath, dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", dir, err, output)
	}

	return outPath
}
