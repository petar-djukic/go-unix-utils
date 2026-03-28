// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R5.1–R5.2 (BuildBinary with caching).

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// binaryCache stores compiled binary paths keyed by absolute package path.
// R5.1: repeated calls with the same package path reuse the binary.
var binaryCache sync.Map

// BuildBinary compiles the Go package at dir into a temporary binary and
// returns the path to the compiled binary. Repeated calls with the same
// package path within a test run return the cached binary without
// recompilation.
//
// R5.1: cache compiled binaries keyed by absolute package path.
// R5.2: output binary is placed in a temp directory for automatic cleanup.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()

	absDir := resolveAbsDir(t, dir)

	if cached, ok := binaryCache.Load(absDir); ok {
		return cached.(string)
	}

	outPath := compileBinary(t, dir, absDir)
	return outPath
}

// resolveAbsDir converts dir to an absolute path for use as cache key.
func resolveAbsDir(t *testing.T, dir string) string {
	t.Helper()
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("BuildBinary: filepath.Abs(%s) failed: %v", dir, err)
	}
	return absDir
}

// compileBinary runs go build and stores the result in the cache.
func compileBinary(t *testing.T, dir, absDir string) string {
	t.Helper()

	binName := binaryName()
	outDir, err := os.MkdirTemp("", "testutils-build-*")
	if err != nil {
		t.Fatalf("BuildBinary: MkdirTemp failed: %v", err)
	}
	outPath := filepath.Join(outDir, binName)

	cmd := exec.Command("go", "build", "-o", outPath, dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(outDir) // best-effort cleanup on build failure
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", dir, err, output)
	}

	// Store-or-load: if another goroutine compiled first, use theirs.
	actual, loaded := binaryCache.LoadOrStore(absDir, outPath)
	if loaded {
		os.RemoveAll(outDir) // best-effort cleanup of duplicate build
		return actual.(string)
	}
	return outPath
}

// binaryName returns the platform-appropriate binary filename.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "binary.exe"
	}
	return "binary"
}
