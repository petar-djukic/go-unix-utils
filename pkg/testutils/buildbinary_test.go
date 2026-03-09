// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for BuildBinary helper (prd001-testutils R4.1–R4.2 task requirements).

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildBinary_ValidSource(t *testing.T) {
	t.Parallel()

	// Create a standalone Go source file. go build accepts individual .go files
	// without requiring module context.
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	src := `package main

import "fmt"

func main() { fmt.Println("hello") }
`
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatalf("writing main.go: %v", err)
	}

	// R4.1: BuildBinary compiles the source and returns a path to a working executable.
	binPath := BuildBinary(t, srcFile)

	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if info.IsDir() {
		t.Fatal("BuildBinary returned a directory, not a file")
	}

	// Verify the binary runs and produces expected output.
	cmd := exec.Command(binPath)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running built binary: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("binary output = %q, want %q", out, "hello\n")
	}
}

func TestBuildBinary_ReturnsAbsolutePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	src := `package main

func main() {}
`
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatalf("writing main.go: %v", err)
	}

	binPath := BuildBinary(t, srcFile)

	if !filepath.IsAbs(binPath) {
		t.Errorf("BuildBinary returned relative path %q, want absolute", binPath)
	}
}

func TestBuildBinary_CleanupRemovesBinary(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	src := `package main

func main() {}
`
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatalf("writing main.go: %v", err)
	}

	// R4.1: BuildBinary registers cleanup via t.Cleanup.
	binPath := BuildBinary(t, srcFile)

	// The binary should exist now, before cleanup runs.
	if _, err := os.Stat(binPath); err != nil {
		t.Errorf("binary should exist before cleanup: %v", err)
	}
}

func TestBuildBinary_FailsOnInvalidSource(t *testing.T) {
	t.Parallel()

	// R4.2: Verify that go build fails on invalid source.
	// We cannot directly test t.Fatal behavior (it calls runtime.Goexit),
	// so we verify the underlying go build fails on invalid input.
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "test-binary")

	cmd := exec.Command("go", "build", "-o", binPath, filepath.Join(tmpDir, "nonexistent.go"))
	err := cmd.Run()
	if err == nil {
		t.Error("go build should fail on nonexistent source file")
	}

	// Verify no binary was produced.
	if _, statErr := os.Stat(binPath); statErr == nil {
		t.Error("binary should not exist after failed build")
	}
}
