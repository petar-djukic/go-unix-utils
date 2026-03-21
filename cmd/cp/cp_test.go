// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cp covering prd056-cp R1.1 (basic file copying),
// R1.2 (interactive mode), R1.3 (force mode), R1.4 (no-clobber mode).
package main_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	normBin := makeBinaryNormalizer(refBin)
	runSharedTests(t, goBin, refBin, normBin)
	runIsolatedTests(t, goBin, refBin, normBin)
}

// runSharedTests runs tests where both binaries can share the same WorkDir
// (error cases where no filesystem mutation succeeds).
func runSharedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()

	dirWithFile := t.TempDir()
	setupFile(t, dirWithFile, "src.txt", "hello\n")

	dirWithDir := t.TempDir()
	requireMkdir(t, filepath.Join(dirWithDir, "subdir"))

	tests := []testutils.DiffTest{
		// R1.1: missing file operand.
		{
			Name:      "missing_operand",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// R1.1: missing destination operand.
		{
			Name:      "missing_dest_operand",
			Args:      []string{"src.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// R1.1: source does not exist.
		{
			Name:      "source_not_found",
			Args:      []string{filepath.Join(dirWithFile, "nonexistent"), filepath.Join(dirWithFile, "dest.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// R1.1: source is a directory without -r.
		{
			Name:      "directory_without_r",
			Args:      []string{filepath.Join(dirWithDir, "subdir"), filepath.Join(dirWithDir, "copy")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// R1.1: multiple sources, target is not a directory.
		{
			Name: "multi_source_target_not_dir",
			Args: []string{
				filepath.Join(dirWithFile, "src.txt"),
				filepath.Join(dirWithFile, "src.txt"),
				filepath.Join(dirWithFile, "src.txt"),
			},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a test where each binary runs in its own temp dir.
type isolatedCase struct {
	name   string
	args   []string
	stdin  []byte
	setup  func(t *testing.T, dir string)
	norm   []testutils.NormalizeFunc
	verify func(t *testing.T, dir string)
}

// runIsolatedTests runs tests where each binary needs its own WorkDir
// because both create or modify files.
func runIsolatedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	cases := buildIsolatedCases(normBin)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, tc)
		})
	}
}

// buildIsolatedCases returns all isolated test cases for R1.1–R1.4.
func buildIsolatedCases(normBin testutils.NormalizeFunc) []isolatedCase {
	return []isolatedCase{
		// R1.1: copy single file to new destination.
		{
			name: "single_file_copy",
			args: []string{"src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "hello world\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "hello world\n")
			},
		},
		// R1.1: copy overwrites existing file by default.
		{
			name: "overwrite_existing",
			args: []string{"src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "new content\n")
				setupFile(t, dir, "dest.txt", "old content\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "new content\n")
			},
		},
		// R1.1: copy multiple files into directory.
		{
			name: "multi_file_into_dir",
			args: []string{"a.txt", "b.txt", "dest"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "a.txt", "aaa\n")
				setupFile(t, dir, "b.txt", "bbb\n")
				requireMkdir(t, filepath.Join(dir, "dest"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest", "a.txt"), "aaa\n")
				verifyFileContent(t, filepath.Join(dir, "dest", "b.txt"), "bbb\n")
			},
		},
		// R1.1: copy single file into existing directory.
		{
			name: "single_file_into_dir",
			args: []string{"src.txt", "dest"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "data\n")
				requireMkdir(t, filepath.Join(dir, "dest"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest", "src.txt"), "data\n")
			},
		},
		// R1.2: -i with "n" response preserves destination.
		{
			name:  "interactive_no",
			args:  []string{"-i", "src.txt", "dest.txt"},
			stdin: []byte("n\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "new\n")
				setupFile(t, dir, "dest.txt", "old\n")
			},
			norm: []testutils.NormalizeFunc{normBin},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "old\n")
			},
		},
		// R1.2: -i with "y" response overwrites destination.
		{
			name:  "interactive_yes",
			args:  []string{"-i", "src.txt", "dest.txt"},
			stdin: []byte("y\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "new\n")
				setupFile(t, dir, "dest.txt", "old\n")
			},
			norm: []testutils.NormalizeFunc{normBin},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "new\n")
			},
		},
		// R1.2: -i does not prompt when destination doesn't exist.
		{
			name: "interactive_no_dest",
			args: []string{"-i", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "data\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "data\n")
			},
		},
		// R1.3: -f removes read-only dest and copies.
		{
			name: "force_readonly_dest",
			args: []string{"-f", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "new content\n")
				setupFile(t, dir, "dest.txt", "old content\n")
				if err := os.Chmod(filepath.Join(dir, "dest.txt"), 0o444); err != nil {
					t.Fatalf("chmod: %v", err)
				}
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "new content\n")
			},
		},
		// R1.4: -n does not overwrite existing file.
		{
			name: "no_clobber",
			args: []string{"-n", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "new\n")
				setupFile(t, dir, "dest.txt", "old\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "old\n")
			},
		},
		// R1.4: -n copies when destination doesn't exist.
		{
			name: "no_clobber_new_dest",
			args: []string{"-n", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "data\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "data\n")
			},
		},
		// R1.2: -i with declined copy exits 1.
		{
			name: "interactive_no_exit_code",
			args: []string{"--interactive", "src.txt", "dest.txt"},
			stdin: []byte("n\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "new\n")
				setupFile(t, dir, "dest.txt", "old\n")
			},
			norm: []testutils.NormalizeFunc{normBin},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "old\n")
			},
		},
		// R1.1: copy empty file.
		{
			name: "copy_empty_file",
			args: []string{"empty.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "empty.txt", "")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "")
			},
		},
	}
}

// TestNoClobberOverridesInteractive verifies R1.4: -n takes precedence over -i.
// This is a standalone test because GNU cp uses "last flag wins" semantics
// while the PRD specifies -n always takes precedence.
func TestNoClobberOverridesInteractive(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	setupFile(t, dir, "src.txt", "new\n")
	setupFile(t, dir, "dest.txt", "old\n")
	cmd := exec.Command(goBin, "-ni", "src.txt", "dest.txt")
	cmd.Dir = dir
	cmd.Env = buildTestEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("cp -ni should exit 0, got error: %v, stderr: %s", err, stderr.String())
	}
	verifyFileContent(t, filepath.Join(dir, "dest.txt"), "old\n")
}

// compareIsolated runs both binaries in separate temp dirs and compares.
func compareIsolated(t *testing.T, goBin, refBin string, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if tc.setup != nil {
		tc.setup(t, refDir)
		tc.setup(t, goDir)
	}
	refOut, refErr, refCode := execBinary(t, refBin, tc.args, refDir, tc.stdin)
	goOut, goErr, goCode := execBinary(t, goBin, tc.args, goDir, tc.stdin)
	refOut, goOut, refErr, goErr = applyNorm(tc.norm, refOut, goOut, refErr, goErr)
	assertMatch(t, tc.args, refOut, goOut, refErr, goErr, refCode, goCode)
	if tc.verify != nil {
		tc.verify(t, goDir)
	}
}

// assertMatch fails the test if ref and go outputs diverge.
func assertMatch(t *testing.T, args []string, refOut, goOut, refErr, goErr []byte, refCode, goCode int) {
	t.Helper()
	if !bytes.Equal(refOut, goOut) || !bytes.Equal(refErr, goErr) || refCode != goCode {
		t.Fatalf("divergence\nargs:       %v\n"+
			"ref stdout: %s\ngo  stdout: %s\n"+
			"ref stderr: %s\ngo  stderr: %s\n"+
			"ref exit:   %d\ngo  exit:   %d",
			args, refOut, goOut, refErr, goErr, refCode, goCode)
	}
}

// execBinary runs a binary with args and optional stdin in dir,
// returning stdout, stderr, and exit code.
func execBinary(t *testing.T, bin string, args []string, dir string, stdin []byte) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = buildTestEnv()
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("binary %s failed to execute: %v", bin, err)
	}
	return stdout.Bytes(), stderr.Bytes(), 0
}

// buildTestEnv returns the process environment with LC_ALL=C.
func buildTestEnv() []string {
	env := os.Environ()
	prefix := "LC_ALL="
	for i, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}

// makeBinaryNormalizer returns a normalizer that replaces the reference
// binary name with "cp" and lowercases to handle strerror differences.
func makeBinaryNormalizer(refBin string) testutils.NormalizeFunc {
	refDir := filepath.Dir(refBin)
	return func(data []byte) []byte {
		data = bytes.ReplaceAll(data, []byte(refBin), []byte("cp"))
		if refDir != "" {
			data = bytes.ReplaceAll(data, []byte(refDir+"/cp"), []byte("cp"))
		}
		data = bytes.ReplaceAll(data, []byte("gcp"), []byte("cp"))
		return bytes.ToLower(data)
	}
}

// setupFile creates a file with the given content in dir.
func setupFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup file %s: %v", path, err)
	}
}

// requireMkdir creates a directory, failing the test on error.
func requireMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// verifyFileContent checks that a file exists with the expected content.
func verifyFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file %s content = %q, want %q", path, string(data), want)
	}
}

// applyNorm applies normalizers to ref and go stdout/stderr pairs.
func applyNorm(
	norm []testutils.NormalizeFunc,
	refOut, goOut, refErr, goErr []byte,
) ([]byte, []byte, []byte, []byte) {
	for _, n := range norm {
		refOut = n(refOut)
		goOut = n(goOut)
		refErr = n(refErr)
		goErr = n(goErr)
	}
	return refOut, goOut, refErr, goErr
}
