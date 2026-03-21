// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cp covering prd056-cp R1.1 (basic file copying),
// R1.2 (interactive mode), R1.3 (force mode), R1.4 (no-clobber mode),
// R2.1 (recursive copy), R2.2 (directory without -r), R2.3 (dereference),
// R2.4 (no-dereference/preserve symlinks), R3.1 (preserve mode/timestamps),
// R3.2 (archive mode), R3.3 (--preserve=ATTR_LIST), R3.4 (verbose output),
// R4.1 (exit 0 on success), R4.2 (exit 1 on failure), R4.3 (-t target-directory),
// R4.4 (comprehensive differential tests).
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
		// R2.2: source is a directory without -r.
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
		// R4.3: -t with nonexistent target directory.
		{
			Name: "target_dir_not_found",
			Args: []string{
				"-t", "/nonexistent_target_dir_for_cp_test",
				filepath.Join(dirWithFile, "src.txt"),
			},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// R4.3: -t with target that is a regular file, not a directory.
		{
			Name: "target_dir_is_file",
			Args: []string{
				"--target-directory=" + filepath.Join(dirWithFile, "src.txt"),
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
	cases = append(cases, buildR2IsolatedCases()...)
	cases = append(cases, buildR3IsolatedCases(normBin)...)
	cases = append(cases, buildR4IsolatedCases(normBin)...)
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

// buildR2IsolatedCases returns isolated test cases for R2.1–R2.4.
func buildR2IsolatedCases() []isolatedCase {
	return []isolatedCase{
		// R2.1: recursive copy of directory with files.
		{
			name: "recursive_dir_copy",
			args: []string{"-r", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "a.txt"), "aaa\n")
				setupFile(t, dir, filepath.Join("srcdir", "b.txt"), "bbb\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "destdir", "a.txt"), "aaa\n")
				verifyFileContent(t, filepath.Join(dir, "destdir", "b.txt"), "bbb\n")
			},
		},
		// R2.1: recursive copy with nested subdirectories.
		{
			name: "recursive_nested_dirs",
			args: []string{"-R", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdirAll(t, filepath.Join(dir, "srcdir", "sub1", "sub2"))
				setupFile(t, dir, filepath.Join("srcdir", "top.txt"), "top\n")
				setupFile(t, dir, filepath.Join("srcdir", "sub1", "mid.txt"), "mid\n")
				setupFile(t, dir, filepath.Join("srcdir", "sub1", "sub2", "deep.txt"), "deep\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "destdir", "top.txt"), "top\n")
				verifyFileContent(t, filepath.Join(dir, "destdir", "sub1", "mid.txt"), "mid\n")
				verifyFileContent(t, filepath.Join(dir, "destdir", "sub1", "sub2", "deep.txt"), "deep\n")
			},
		},
		// R2.1: recursive copy with --recursive long flag.
		{
			name: "recursive_long_flag",
			args: []string{"--recursive", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "file.txt"), "content\n")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "destdir", "file.txt"), "content\n")
			},
		},
		// R2.1: recursive copy of empty directory.
		{
			name: "recursive_empty_dir",
			args: []string{"-r", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				info, err := os.Stat(filepath.Join(dir, "destdir"))
				if err != nil {
					t.Fatalf("destdir not created: %v", err)
				}
				if !info.IsDir() {
					t.Fatal("destdir is not a directory")
				}
			},
		},
		// R2.1: recursive copy into existing directory.
		{
			name: "recursive_into_existing_dir",
			args: []string{"-r", "srcdir", "dest"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "f.txt"), "data\n")
				requireMkdir(t, filepath.Join(dir, "dest"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest", "srcdir", "f.txt"), "data\n")
			},
		},
		// R2.3: -L follows symlinks, copying the target file.
		{
			name: "dereference_symlink",
			args: []string{"-rL", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "real.txt"), "real content\n")
				requireSymlink(t, "real.txt",
					filepath.Join(dir, "srcdir", "link.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				// With -L, link.txt should be a regular file copy
				verifyFileContent(t, filepath.Join(dir, "destdir", "link.txt"), "real content\n")
				verifyNotSymlink(t, filepath.Join(dir, "destdir", "link.txt"))
			},
		},
		// R2.4: -P preserves symlinks (default with -r).
		{
			name: "no_dereference_symlink",
			args: []string{"-r", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "real.txt"), "real content\n")
				requireSymlink(t, "real.txt",
					filepath.Join(dir, "srcdir", "link.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				// With -r (default -P), link.txt should be a symlink
				verifyIsSymlink(t, filepath.Join(dir, "destdir", "link.txt"))
				target, err := os.Readlink(filepath.Join(dir, "destdir", "link.txt"))
				if err != nil {
					t.Fatalf("readlink: %v", err)
				}
				if target != "real.txt" {
					t.Fatalf("symlink target = %q, want %q", target, "real.txt")
				}
			},
		},
		// R2.4: explicit -P flag preserves symlinks.
		{
			name: "explicit_no_dereference",
			args: []string{"-rP", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "target.txt"), "target\n")
				requireSymlink(t, "target.txt",
					filepath.Join(dir, "srcdir", "slink"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyIsSymlink(t, filepath.Join(dir, "destdir", "slink"))
			},
		},
		// R2.3: --dereference long flag.
		{
			name: "dereference_long_flag",
			args: []string{"-r", "--dereference", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "real.txt"), "data\n")
				requireSymlink(t, "real.txt",
					filepath.Join(dir, "srcdir", "link.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "destdir", "link.txt"), "data\n")
				verifyNotSymlink(t, filepath.Join(dir, "destdir", "link.txt"))
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

// requireMkdirAll creates a directory tree, failing the test on error.
func requireMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdirall %s: %v", path, err)
	}
}

// requireSymlink creates a symlink at path pointing to target.
func requireSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink %s -> %s: %v", path, target, err)
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

// verifyIsSymlink checks that the path is a symlink.
func verifyIsSymlink(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink (mode=%v)", path, info.Mode())
	}
}

// verifyNotSymlink checks that the path is not a symlink.
func verifyNotSymlink(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("%s is a symlink, expected regular file", path)
	}
}

// buildR3IsolatedCases returns isolated test cases for R3.1–R3.4.
func buildR3IsolatedCases(normBin testutils.NormalizeFunc) []isolatedCase {
	return []isolatedCase{
		// R3.1: -p preserves mode and timestamps.
		{
			name: "preserve_p_flag",
			args: []string{"-p", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "preserved\n")
				requireChmod(t, filepath.Join(dir, "src.txt"), 0o755)
				setOldTimestamps(t, filepath.Join(dir, "src.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "preserved\n")
				verifyModeMatch(t, filepath.Join(dir, "src.txt"), filepath.Join(dir, "dest.txt"))
				verifyTimestampMatch(t, filepath.Join(dir, "src.txt"), filepath.Join(dir, "dest.txt"))
			},
		},
		// R3.1: --preserve long flag defaults to mode,ownership,timestamps.
		{
			name: "preserve_long_flag",
			args: []string{"--preserve", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "long flag\n")
				requireChmod(t, filepath.Join(dir, "src.txt"), 0o700)
				setOldTimestamps(t, filepath.Join(dir, "src.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyModeMatch(t, filepath.Join(dir, "src.txt"), filepath.Join(dir, "dest.txt"))
				verifyTimestampMatch(t, filepath.Join(dir, "src.txt"), filepath.Join(dir, "dest.txt"))
			},
		},
		// R3.2: -a recursive with metadata preservation.
		{
			name: "archive_recursive",
			args: []string{"-a", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "f.txt"), "archived\n")
				requireChmod(t, filepath.Join(dir, "srcdir", "f.txt"), 0o755)
				setOldTimestamps(t, filepath.Join(dir, "srcdir", "f.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				destFile := filepath.Join(dir, "destdir", "f.txt")
				verifyFileContent(t, destFile, "archived\n")
				verifyModeMatch(t, filepath.Join(dir, "srcdir", "f.txt"), destFile)
				verifyTimestampMatch(t, filepath.Join(dir, "srcdir", "f.txt"), destFile)
			},
		},
		// R3.2: --archive long flag.
		{
			name: "archive_long_flag",
			args: []string{"--archive", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "f.txt"), "archive\n")
				requireChmod(t, filepath.Join(dir, "srcdir", "f.txt"), 0o700)
				setOldTimestamps(t, filepath.Join(dir, "srcdir", "f.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				destFile := filepath.Join(dir, "destdir", "f.txt")
				verifyFileContent(t, destFile, "archive\n")
				verifyModeMatch(t, filepath.Join(dir, "srcdir", "f.txt"), destFile)
				verifyTimestampMatch(t, filepath.Join(dir, "srcdir", "f.txt"), destFile)
			},
		},
		// R3.3: --preserve=timestamps preserves only timestamps.
		{
			name: "preserve_timestamps_only",
			args: []string{"--preserve=timestamps", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "ts only\n")
				setOldTimestamps(t, filepath.Join(dir, "src.txt"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyTimestampMatch(t, filepath.Join(dir, "src.txt"),
					filepath.Join(dir, "dest.txt"))
			},
		},
		// R3.3: --preserve=mode preserves only mode.
		{
			name: "preserve_mode_only",
			args: []string{"--preserve=mode", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "mode only\n")
				requireChmod(t, filepath.Join(dir, "src.txt"), 0o755)
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyModeMatch(t, filepath.Join(dir, "src.txt"),
					filepath.Join(dir, "dest.txt"))
			},
		},
		// R3.4: -v verbose output.
		{
			name: "verbose_output",
			args: []string{"-v", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "verbose\n")
			},
			norm: []testutils.NormalizeFunc{normBin},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "verbose\n")
			},
		},
		// R3.4: --verbose long flag.
		{
			name: "verbose_long_flag",
			args: []string{"--verbose", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "verbose long\n")
			},
			norm: []testutils.NormalizeFunc{normBin},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "verbose long\n")
			},
		},
		// R3.4: -v with recursive copy.
		{
			name: "verbose_recursive",
			args: []string{"-rv", "srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "srcdir"))
				setupFile(t, dir, filepath.Join("srcdir", "a.txt"), "aaa\n")
			},
			norm: []testutils.NormalizeFunc{normBin},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "destdir", "a.txt"), "aaa\n")
			},
		},
		// R3.1+R3.4: -pv combined flags.
		{
			name: "preserve_verbose_combined",
			args: []string{"-pv", "src.txt", "dest.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "pv combo\n")
				requireChmod(t, filepath.Join(dir, "src.txt"), 0o755)
				setOldTimestamps(t, filepath.Join(dir, "src.txt"))
			},
			norm: []testutils.NormalizeFunc{normBin},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest.txt"), "pv combo\n")
				verifyModeMatch(t, filepath.Join(dir, "src.txt"), filepath.Join(dir, "dest.txt"))
				verifyTimestampMatch(t, filepath.Join(dir, "src.txt"), filepath.Join(dir, "dest.txt"))
			},
		},
	}
}

// requireChmod sets the file mode, failing the test on error.
func requireChmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// setOldTimestamps sets mtime and atime to a fixed past date.
func setOldTimestamps(t *testing.T, path string) {
	t.Helper()
	oldTime := time.Date(2020, 1, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// verifyModeMatch checks that src and dest have the same permission bits.
func verifyModeMatch(t *testing.T, src, dest string) {
	t.Helper()
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat src %s: %v", src, err)
	}
	destInfo, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat dest %s: %v", dest, err)
	}
	if srcInfo.Mode().Perm() != destInfo.Mode().Perm() {
		t.Fatalf("mode mismatch: src=%o, dest=%o",
			srcInfo.Mode().Perm(), destInfo.Mode().Perm())
	}
}

// verifyTimestampMatch checks that src and dest have the same modification time.
func verifyTimestampMatch(t *testing.T, src, dest string) {
	t.Helper()
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat src %s: %v", src, err)
	}
	destInfo, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat dest %s: %v", dest, err)
	}
	if !srcInfo.ModTime().Equal(destInfo.ModTime()) {
		t.Fatalf("mtime mismatch: src=%v, dest=%v",
			srcInfo.ModTime(), destInfo.ModTime())
	}
}

// buildR4IsolatedCases returns isolated test cases for R4.1–R4.4.
func buildR4IsolatedCases(normBin testutils.NormalizeFunc) []isolatedCase {
	return []isolatedCase{
		// R4.3: -t copies multiple files into target directory.
		{
			name: "target_dir_short_flag",
			args: []string{"-t", "dest", "a.txt", "b.txt"},
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
		// R4.3: --target-directory= long flag with equals.
		{
			name: "target_dir_long_equals",
			args: []string{"--target-directory=dest", "src.txt"},
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
		// R4.3: --target-directory as separate argument.
		{
			name: "target_dir_long_separate",
			args: []string{"--target-directory", "dest", "src.txt"},
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
		// R4.3: -t with single file copy.
		{
			name: "target_dir_single_file",
			args: []string{"-t", "dest", "src.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "single\n")
				requireMkdir(t, filepath.Join(dir, "dest"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest", "src.txt"), "single\n")
			},
		},
		// R4.3: -t combined with other flags (-vt).
		{
			name: "target_dir_combined_flags",
			args: []string{"-vt", "dest", "src.txt"},
			norm: []testutils.NormalizeFunc{normBin},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "src.txt", "combined\n")
				requireMkdir(t, filepath.Join(dir, "dest"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest", "src.txt"), "combined\n")
			},
		},
		// R4.2: permission denied on source file.
		{
			name: "permission_denied_source",
			args: []string{"noperm.txt", "dest.txt"},
			norm: []testutils.NormalizeFunc{normBin},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "noperm.txt", "secret\n")
				requireChmod(t, filepath.Join(dir, "noperm.txt"), 0o000)
			},
		},
		// R4.1/R4.2: partial failure exits 1 (one good, one bad source).
		{
			name: "partial_failure_exit_code",
			args: []string{"good.txt", "nonexistent.txt", "dest"},
			norm: []testutils.NormalizeFunc{normBin},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "good.txt", "ok\n")
				requireMkdir(t, filepath.Join(dir, "dest"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyFileContent(t, filepath.Join(dir, "dest", "good.txt"), "ok\n")
			},
		},
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
