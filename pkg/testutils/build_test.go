// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// minimalMain is the source for a trivial Go command used in BuildBinary tests.
const minimalMain = `package main

func main() {}
`

// minimalGoMod is a self-contained go.mod for the test command.
const minimalGoMod = `module testcmd

go 1.21
`

// writeSrcDir creates a temporary directory containing a minimal Go command
// package and returns its path. The caller is responsible for cleanup via
// t.TempDir (the returned dir is already rooted in a TempDir).
func writeSrcDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(minimalGoMod), 0o644); err != nil {
		t.Fatalf("writeSrcDir: write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(minimalMain), 0o644); err != nil {
		t.Fatalf("writeSrcDir: write main.go: %v", err)
	}
	return dir
}

// TestBuildBinary_HappyPath verifies that BuildBinary compiles a valid Go
// package, returns a non-empty path, and the binary is executable.
func TestBuildBinary_HappyPath(t *testing.T) {
	t.Parallel()

	srcDir := writeSrcDir(t)
	binPath := testutils.BuildBinary(t, srcDir)

	if binPath == "" {
		t.Fatal("BuildBinary returned empty path")
	}

	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("stat binary %q: %v", binPath, err)
	}
	// AC1: binary must be executable.
	if info.Mode()&0o111 == 0 {
		t.Errorf("binary %q is not executable (mode %v)", binPath, info.Mode())
	}
}

// TestBuildBinary_BinaryRuns verifies that the binary produced by BuildBinary
// can actually be executed and exits with code 0.
func TestBuildBinary_BinaryRuns(t *testing.T) {
	t.Parallel()

	srcDir := writeSrcDir(t)
	binPath := testutils.BuildBinary(t, srcDir)

	cmd := exec.Command(binPath)
	if err := cmd.Run(); err != nil {
		t.Errorf("running binary %q: %v", binPath, err)
	}
}

// TestBuildBinary_CleanupRemovesBinary verifies that the binary is removed
// after the test completes. We do this by running a sub-test that records the
// binary path, then checking the path is gone after Cleanup runs.
func TestBuildBinary_CleanupRemovesBinary(t *testing.T) {
	t.Parallel()

	var binPath string
	t.Run("inner", func(t *testing.T) {
		srcDir := writeSrcDir(t)
		binPath = testutils.BuildBinary(t, srcDir)
		if _, err := os.Stat(binPath); err != nil {
			t.Fatalf("binary %q should exist during test: %v", binPath, err)
		}
		// When this sub-test returns, t.TempDir cleanup removes the binary.
	})

	// After the inner sub-test completes, its t.TempDir has been cleaned up.
	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Errorf("binary %q should have been removed after test cleanup; stat err: %v", binPath, err)
	}
}
