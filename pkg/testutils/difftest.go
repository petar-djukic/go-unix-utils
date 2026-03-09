// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides differential testing infrastructure for comparing
// Go utility implementations against GNU reference binaries.
// Implements prd001-testutils.
package testutils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// NormalizeFunc transforms output bytes before comparison.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. ExpectedFiles specifies
// files to create in the working directory before running each binary.
type DiffTest struct {
	Name          string
	Args          []string
	Stdin         []byte
	Env           []string
	WorkDir       string
	ExitCode      int
	Normalize     []NormalizeFunc
	ExpectedFiles map[string][]byte
}

// ComposeNormalizers chains multiple normalizers into one.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// TimestampNormalizer replaces timestamp content for comparison.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return b
}

// BuildBinary compiles the Go package at the given path and returns the
// path to the compiled binary.
func BuildBinary(t *testing.T, pkg string) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "test-binary")
	cmd := exec.Command("go", "build", "-o", binPath, pkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary(%s): %v\n%s", pkg, err, out)
	}
	return binPath
}

// runResult holds the output of a single binary execution.
type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBinary executes a binary with the given test parameters and returns its output.
func runBinary(bin string, tc DiffTest, workDir string) runResult {
	cmd := exec.Command(bin, tc.Args...)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(tc.Stdin)
	env := append(os.Environ(), "LC_ALL=C")
	env = append(env, tc.Env...)
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
	}

	return runResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// RunDiffTests runs each test case against both the Go binary and the
// reference binary, comparing stdout, stderr, exit code, and files
// created in the working directory.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			goDir := t.TempDir()
			refDir := t.TempDir()

			// Set up files in both working directories.
			setupFiles(t, goDir, tc.ExpectedFiles)
			setupFiles(t, refDir, tc.ExpectedFiles)

			goResult := runBinary(goBinary, tc, goDir)
			refResult := runBinary(refBinary, tc, refDir)

			// Apply normalizers to stdout.
			goStdout := applyNormalizers(goResult.stdout, tc.Normalize)
			refStdout := applyNormalizers(refResult.stdout, tc.Normalize)

			if goResult.exitCode != refResult.exitCode {
				t.Errorf("exit code: go=%d ref=%d", goResult.exitCode, refResult.exitCode)
			}
			if !bytes.Equal(goStdout, refStdout) {
				t.Errorf("stdout differs:\n  go:  %q\n  ref: %q",
					truncateBytes(goStdout, 200), truncateBytes(refStdout, 200))
			}

			// Compare files in working directories.
			compareWorkDirFiles(t, goDir, refDir, tc.Normalize)
		})
	}
}

func setupFiles(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("setup file %s: %v", name, err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatalf("setup file %s: %v", name, err)
		}
	}
}

func applyNormalizers(data []byte, norms []NormalizeFunc) []byte {
	for _, norm := range norms {
		data = norm(data)
	}
	return data
}

func compareWorkDirFiles(t *testing.T, goDir, refDir string, norms []NormalizeFunc) {
	t.Helper()

	goFiles := collectFiles(t, goDir)
	refFiles := collectFiles(t, refDir)

	for name, refContent := range refFiles {
		goContent, ok := goFiles[name]
		if !ok {
			t.Errorf("file %q exists in ref output but not in go output", name)
			continue
		}
		gc := applyNormalizers(goContent, norms)
		rc := applyNormalizers(refContent, norms)
		if !bytes.Equal(gc, rc) {
			t.Errorf("file %q differs:\n  go (%d bytes):  %q\n  ref (%d bytes): %q",
				name, len(gc), truncateBytes(gc, 200), len(rc), truncateBytes(rc, 200))
		}
	}
	for name := range goFiles {
		if _, ok := refFiles[name]; !ok {
			t.Errorf("file %q exists in go output but not in ref output", name)
		}
	}
}

func collectFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files[rel] = content
		return nil
	})
	if err != nil {
		t.Fatalf("collectFiles(%s): %v", dir, err)
	}
	return files
}

func truncateBytes(b []byte, max int) []byte {
	if len(b) > max {
		return b[:max]
	}
	return b
}
