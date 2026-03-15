// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// writeTestPackage creates a minimal Go main package in dir with the given
// main.go body content and a go.mod file.
func writeTestPackage(t *testing.T, dir, mainBody string) {
	t.Helper()

	mainGo := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\n\n"+mainBody), 0o644); err != nil {
		t.Fatalf("writing main.go: %v", err)
	}

	goMod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module testpkg\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
}

// TestBuildBinaryValidPackage verifies that BuildBinary compiles a valid Go
// package and returns a path to an executable binary (AC1, AC3).
func TestBuildBinaryValidPackage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestPackage(t, dir, "func main() {}\n")

	binPath := BuildBinary(t, dir)

	// Verify the binary exists and is executable.
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("binary not found at %s: %v", binPath, err)
	}
	if info.IsDir() {
		t.Fatalf("expected a file, got a directory: %s", binPath)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("binary is not executable: %s (mode %v)", binPath, info.Mode())
	}
}

// TestBuildBinaryDerivesBinaryName verifies that the binary name is derived
// from the package directory name.
func TestBuildBinaryDerivesBinaryName(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	dir := filepath.Join(parent, "mytool")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("creating directory: %v", err)
	}
	writeTestPackage(t, dir, "func main() {}\n")

	binPath := BuildBinary(t, dir)

	base := filepath.Base(binPath)
	if base != "mytool" {
		t.Errorf("expected binary name %q, got %q", "mytool", base)
	}
}

// TestBuildBinaryProducesRunnable verifies that the compiled binary can
// actually be executed (AC1).
func TestBuildBinaryProducesRunnable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestPackage(t, dir, `import "fmt"

func main() { fmt.Println("hello") }
`)

	binPath := BuildBinary(t, dir)

	// Run the compiled binary and verify it produces expected output.
	out, err := exec.Command(binPath).Output()
	if err != nil {
		t.Fatalf("running compiled binary: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("expected %q, got %q", "hello\n", string(out))
	}
}
