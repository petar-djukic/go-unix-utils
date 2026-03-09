// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for BuildBinary helper and comparison/reporting behavior
// (prd001-testutils R3.1-R3.4).
package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildBinaryProducesExecutable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testmod\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("writing source: %v", err)
	}

	binPath := BuildBinary(t, dir)
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("BuildBinary returned path that does not exist: %v", err)
	}

	if runtime.GOOS != "windows" {
		if info.Mode()&0o111 == 0 {
			t.Error("BuildBinary output is not executable")
		}
	}
}

func TestBuildBinaryCleanup(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testmod\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("writing source: %v", err)
	}

	binPath := BuildBinary(t, dir)

	// Verify the binary exists before cleanup.
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("binary should exist before test cleanup: %v", err)
	}
}

// TestComparisonNormalizesBeforeCompare verifies R3.1: normalizers are applied
// to both stdout and stderr before comparison. Two binaries that produce
// different timestamps should pass when TimestampNormalizer is applied.
func TestComparisonNormalizesBeforeCompare(t *testing.T) {
	t.Parallel()
	goBin := buildMockBinary(t, "ts1", mockTimestampSource)
	refBin := buildMockBinary(t, "ts2", mockTimestampAltSource)

	tests := []DiffTest{
		{
			Name:      "normalize-before-compare",
			ExitCode:  0,
			Normalize: []NormalizeFunc{TimestampNormalizer},
		},
	}
	RunDiffTests(t, goBin, refBin, tests)
}

// TestComparisonStdoutByteForByte verifies R3.2: stdout is compared
// byte-for-byte. Identical binaries must pass.
func TestComparisonStdoutByteForByte(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "echo", mockEchoSource)
	RunDiffTests(t, bin, bin, []DiffTest{{Name: "stdout-match", ExitCode: 0}})
}

// TestComparisonStdoutDivergenceFails verifies R3.2: stdout mismatch causes
// test failure. Uses subprocess pattern to capture test failure.
func TestComparisonStdoutDivergenceFails(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_STDOUT_DIVERGE") == "1" {
		goBin := buildMockBinary(t, "go-bin", mockDivergentSource)
		refBin := buildMockBinary(t, "ref-bin", mockEchoSource)
		RunDiffTests(t, goBin, refBin, []DiffTest{
			{Name: "stdout-diverge", ExitCode: 0},
		})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestComparisonStdoutDivergenceFails$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_STDOUT_DIVERGE=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail on stdout divergence")
	}
	if !strings.Contains(string(out), "divergence detected") {
		t.Errorf("expected 'divergence detected' in output, got:\n%s", out)
	}
}

// TestComparisonStderrByteForByte verifies R3.3: stderr is compared
// byte-for-byte. Mismatch causes test failure.
func TestComparisonStderrByteForByte(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_STDERR_DIVERGE") == "1" {
		goBin := buildMockBinary(t, "go-bin", mockStderrSource)
		refBin := buildMockBinary(t, "ref-bin", mockNoopSource)
		RunDiffTests(t, goBin, refBin, []DiffTest{
			{Name: "stderr-diverge", ExitCode: 0},
		})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestComparisonStderrByteForByte$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_STDERR_DIVERGE=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail on stderr divergence")
	}
	if !strings.Contains(string(out), "divergence detected") {
		t.Errorf("expected 'divergence detected' in output, got:\n%s", out)
	}
}

// TestComparisonExitCodeExact verifies R3.4: exit codes are compared exactly.
// Mismatch causes test failure.
func TestComparisonExitCodeExact(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_EXIT_DIVERGE") == "1" {
		goBin := buildMockBinary(t, "go-bin", mockExitOneSource)
		refBin := buildMockBinary(t, "ref-bin", mockNoopSource)
		RunDiffTests(t, goBin, refBin, []DiffTest{
			{Name: "exit-diverge", ExitCode: 0},
		})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestComparisonExitCodeExact$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_EXIT_DIVERGE=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail on exit code divergence")
	}
	if !strings.Contains(string(out), "divergence detected") {
		t.Errorf("expected 'divergence detected' in output, got:\n%s", out)
	}
}

// TestComparisonExitCodeMatch verifies R3.4: matching exit codes pass.
func TestComparisonExitCodeMatch(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "exit1", mockExitOneSource)
	RunDiffTests(t, bin, bin, []DiffTest{{Name: "exit-match", ExitCode: 1}})
}
