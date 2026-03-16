// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln against gln (GNU coreutils).
// Implements prd037-ln R4.1-R4.3 test coverage for R1.1-R1.4, R2.1-R2.4.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests for error cases and flag parsing
// that do not require filesystem mutation via RunDiffTests directly.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.4: no arguments prints error to stderr and exits non-zero.
		{
			Name:      "no_args",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.4: unrecognized long option exits non-zero.
		{
			Name:      "unrecognized_long_option",
			Args:      []string{"--bogus"},
			ExitCode:  1, // GNU exits 1 for unrecognized option
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// --version exits 0.
		{
			Name:      "version_exit_0",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// --help exits 0.
		{
			Name:      "help_exit_0",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHardLink runs differential tests for hard link creation.
// Each subtest uses isolated temp dirs since both binaries mutate the filesystem.
func TestDiffHardLink(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// R1.1: ln TARGET LINK_NAME creates a hard link.
	t.Run("two_arg_hard_link", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Setup: create target file in both dirs.
		setupFile(t, refDir, "target.txt", "hello\n")
		setupFile(t, goDir, "target.txt", "hello\n")

		args := []string{"target.txt", "link.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify hard link: same inode.
		verifyHardLink(t, filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt"))
	})

	// R1.2 (task): ln TARGET (single-argument form) creates link in cwd.
	t.Run("single_arg_hard_link", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Setup: create a subdirectory with a target file.
		subRef := filepath.Join(refDir, "sub")
		subGo := filepath.Join(goDir, "sub")
		if err := os.Mkdir(subRef, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.Mkdir(subGo, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		setupFile(t, refDir, "sub/file.txt", "data\n")
		setupFile(t, goDir, "sub/file.txt", "data\n")

		args := []string{"sub/file.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify the link was created in cwd with the basename.
		verifyHardLink(t, filepath.Join(goDir, "sub/file.txt"), filepath.Join(goDir, "file.txt"))
	})

	// R1.3 (task): ln TARGET... DIRECTORY creates links in directory.
	t.Run("multi_target_into_directory", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Setup: create target files and destination directory.
		setupFile(t, refDir, "a.txt", "aaa\n")
		setupFile(t, refDir, "b.txt", "bbb\n")
		if err := os.Mkdir(filepath.Join(refDir, "dest"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		setupFile(t, goDir, "a.txt", "aaa\n")
		setupFile(t, goDir, "b.txt", "bbb\n")
		if err := os.Mkdir(filepath.Join(goDir, "dest"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		args := []string{"a.txt", "b.txt", "dest"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify links were created in dest directory.
		verifyHardLink(t, filepath.Join(goDir, "a.txt"), filepath.Join(goDir, "dest/a.txt"))
		verifyHardLink(t, filepath.Join(goDir, "b.txt"), filepath.Join(goDir, "dest/b.txt"))
	})

	// R1.3 (prd037): error when hard linking a directory.
	t.Run("error_hard_link_directory", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Setup: create a directory as target.
		if err := os.Mkdir(filepath.Join(refDir, "somedir"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.Mkdir(filepath.Join(goDir, "somedir"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		args := []string{"somedir", "linktodir"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		// Both should fail.
		if refExit == 0 {
			t.Skipf("reference binary unexpectedly succeeded")
		}
		if goExit != refExit {
			t.Errorf("exit code mismatch: ref=%d go=%d\nref stderr: %q\ngo stderr: %q",
				refExit, goExit, refErr, goErr)
		}
	})

	// R1.4 (prd037): error when destination already exists without -f.
	t.Run("error_existing_destination", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Setup: create target and destination files.
		setupFile(t, refDir, "target.txt", "hello\n")
		setupFile(t, refDir, "existing.txt", "old\n")
		setupFile(t, goDir, "target.txt", "hello\n")
		setupFile(t, goDir, "existing.txt", "old\n")

		args := []string{"target.txt", "existing.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)
	})

	// -f force overwrite of existing destination.
	t.Run("force_overwrite", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "target.txt", "hello\n")
		setupFile(t, refDir, "existing.txt", "old\n")
		setupFile(t, goDir, "target.txt", "hello\n")
		setupFile(t, goDir, "existing.txt", "old\n")

		args := []string{"-f", "target.txt", "existing.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify link was created successfully.
		verifyHardLink(t, filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "existing.txt"))
	})
}

// TestDiffSymLink runs differential tests for symbolic link creation (R2.1-R2.4).
func TestDiffSymLink(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// R2.1: ln -s TARGET LINK_NAME creates a symbolic link.
	t.Run("symbolic_link_basic", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "target.txt", "hello\n")
		setupFile(t, goDir, "target.txt", "hello\n")

		args := []string{"-s", "target.txt", "link.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify symbolic link was created.
		verifySymlink(t, filepath.Join(goDir, "link.txt"), "target.txt")
	})

	// R2.2: symbolic links to directories are allowed.
	t.Run("symbolic_link_to_directory", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		if err := os.Mkdir(filepath.Join(refDir, "somedir"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.Mkdir(filepath.Join(goDir, "somedir"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		args := []string{"-s", "somedir", "dirlink"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify symlink was created.
		verifySymlink(t, filepath.Join(goDir, "dirlink"), "somedir")
	})

	// R2.3: target string stored as-is in symlink.
	t.Run("symbolic_link_preserves_target", func(t *testing.T) {
		t.Parallel()

		goDir := t.TempDir()

		setupFile(t, goDir, "target.txt", "data\n")

		cmd := exec.Command(goBin, "-s", "target.txt", "mylink")
		cmd.Dir = goDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ln -s failed: %v\noutput: %s", err, out)
		}

		got, err := os.Readlink(filepath.Join(goDir, "mylink"))
		if err != nil {
			t.Fatalf("readlink: %v", err)
		}
		if got != "target.txt" {
			t.Errorf("symlink target: got %q, want %q", got, "target.txt")
		}
	})

	// R3.1 + R2.1: ln -sf TARGET EXISTING replaces existing file with symlink.
	t.Run("force_symbolic_overwrite", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "target.txt", "hello\n")
		setupFile(t, refDir, "existing.txt", "old\n")
		setupFile(t, goDir, "target.txt", "hello\n")
		setupFile(t, goDir, "existing.txt", "old\n")

		args := []string{"-sf", "target.txt", "existing.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify symlink was created.
		verifySymlink(t, filepath.Join(goDir, "existing.txt"), "target.txt")
	})

	// R3.2: ln -n treats symlink-to-directory as a file.
	t.Run("no_dereference_symlink_to_dir", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Setup: create a directory and a symlink pointing to it.
		if err := os.Mkdir(filepath.Join(refDir, "realdir"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.Symlink("realdir", filepath.Join(refDir, "dirlink")); err != nil {
			t.Fatalf("setup: %v", err)
		}
		setupFile(t, refDir, "target.txt", "data\n")

		if err := os.Mkdir(filepath.Join(goDir, "realdir"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.Symlink("realdir", filepath.Join(goDir, "dirlink")); err != nil {
			t.Fatalf("setup: %v", err)
		}
		setupFile(t, goDir, "target.txt", "data\n")

		// Without -n, ln would follow "dirlink" -> "realdir" and create link inside it.
		// With -sfn, ln treats dirlink as a regular file and replaces it.
		args := []string{"-sfn", "target.txt", "dirlink"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify dirlink now points to target.txt, not realdir.
		verifySymlink(t, filepath.Join(goDir, "dirlink"), "target.txt")
	})

	// R2.4: ln -sr creates relative symbolic link.
	t.Run("relative_symbolic_link", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Setup: create a subdirectory with a target file.
		if err := os.Mkdir(filepath.Join(refDir, "sub"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		setupFile(t, refDir, "sub/target.txt", "data\n")

		if err := os.Mkdir(filepath.Join(goDir, "sub"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		setupFile(t, goDir, "sub/target.txt", "data\n")

		args := []string{"-sr", "sub/target.txt", "rellink"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)

		// Verify the created symlink uses a relative path.
		goLink, err := os.Readlink(filepath.Join(goDir, "rellink"))
		if err != nil {
			t.Fatalf("readlink go: %v", err)
		}
		refLink, err := os.Readlink(filepath.Join(refDir, "rellink"))
		if err != nil {
			t.Fatalf("readlink ref: %v", err)
		}
		if goLink != refLink {
			t.Errorf("symlink target mismatch: go=%q ref=%q", goLink, refLink)
		}
	})

	// Symlink without -f fails when destination exists.
	t.Run("symbolic_link_existing_no_force", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "target.txt", "hello\n")
		setupFile(t, refDir, "existing.txt", "old\n")
		setupFile(t, goDir, "target.txt", "hello\n")
		setupFile(t, goDir, "existing.txt", "old\n")

		args := []string{"-s", "target.txt", "existing.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)
	})
}

// verifySymlink checks that path is a symbolic link pointing to expectedTarget.
func verifySymlink(t *testing.T, path, expectedTarget string) {
	t.Helper()

	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink, got mode %v", path, fi.Mode())
	}
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if target != expectedTarget {
		t.Errorf("symlink %s: got target %q, want %q", path, target, expectedTarget)
	}
}

// TestDiffVerbose verifies -v flag output matches gln.
func TestDiffVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	t.Run("verbose_hard_link", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "src.txt", "data\n")
		setupFile(t, goDir, "src.txt", "data\n")

		args := []string{"-v", "src.txt", "dst.txt"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		assertMatch(t, args, refOut, goOut, refErr, goErr, refExit, goExit)
	})
}

// TestHardLinkInode verifies that a hard link shares the same inode.
func TestHardLinkInode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	setupFile(t, dir, "original.txt", "content\n")

	cmd := exec.Command(goBin, "original.txt", "hardlink.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ln failed: %v\noutput: %s", err, out)
	}

	verifyHardLink(t, filepath.Join(dir, "original.txt"), filepath.Join(dir, "hardlink.txt"))
}

// setupFile creates a file with the given content in the directory.
func setupFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: failed to write %s: %v", name, err)
	}
}

// verifyHardLink checks that two paths refer to the same inode (hard link).
func verifyHardLink(t *testing.T, path1, path2 string) {
	t.Helper()

	fi1, err := os.Stat(path1)
	if err != nil {
		t.Fatalf("stat %s: %v", path1, err)
	}
	fi2, err := os.Stat(path2)
	if err != nil {
		t.Fatalf("stat %s: %v", path2, err)
	}

	if !os.SameFile(fi1, fi2) {
		t.Errorf("expected %s and %s to be hard links (same inode), but they are not", path1, path2)
	}
}

// runCmd executes a binary and returns stdout, stderr, and exit code.
func runCmd(t *testing.T, binary string, args []string, workDir string) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Env = buildTestEnv()

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("failed to execute %s: %v", binary, runErr)
	}
	return outBuf.Bytes(), errBuf.Bytes(), 0
}

// buildTestEnv constructs the environment with LC_ALL=C set.
func buildTestEnv() []string {
	var result []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "LC_ALL=") {
			result = append(result, e)
		}
	}
	return append(result, "LC_ALL=C")
}

// normalizeProgramName replaces the reference binary name and path with
// "ln", then lowercases the output so OS-level error string case
// differences do not cause false divergence.
func normalizeProgramName(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		line = normalizePathPrefix(line)
		lines[i] = bytes.ReplaceAll(line, []byte("gln"), []byte("ln"))
	}
	b = bytes.Join(lines, []byte("\n"))
	// Normalize "Try '/path/to/ln --help'" paths.
	for {
		idx := bytes.Index(b, []byte("Try '"))
		if idx == -1 {
			break
		}
		rest := b[idx+5:]
		end := bytes.IndexByte(rest, '\'')
		if end == -1 {
			break
		}
		inner := rest[:end]
		if mIdx := bytes.LastIndex(inner, []byte("ln")); mIdx > 0 {
			prefix := inner[:mIdx]
			if slashIdx := bytes.LastIndexByte(prefix, '/'); slashIdx >= 0 {
				replacement := append([]byte("Try '"), inner[slashIdx+1:]...)
				replacement = append(replacement, '\'')
				old := b[idx : idx+5+end+1]
				b = bytes.Replace(b, old, replacement, 1)
				continue
			}
		}
		break
	}
	return bytes.ToLower(b)
}

// normalizePathPrefix replaces a leading "/path/to/gln: " or
// "/path/to/ln: " with "ln: ".
func normalizePathPrefix(line []byte) []byte {
	colonIdx := bytes.Index(line, []byte(": "))
	if colonIdx == -1 {
		return line
	}
	prog := line[:colonIdx]
	if slashIdx := bytes.LastIndexByte(prog, '/'); slashIdx >= 0 {
		base := prog[slashIdx+1:]
		if bytes.Equal(base, []byte("gln")) || bytes.Equal(base, []byte("ln")) {
			return append([]byte("ln"), line[colonIdx:]...)
		}
	}
	return line
}

// assertMatch compares stdout, stderr, and exit code between ref and go binaries.
func assertMatch(t *testing.T, args []string, refOut, goOut, refErr, goErr []byte, refExit, goExit int) {
	t.Helper()

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d\nargs: %v\nref stderr: %q\ngo stderr: %q",
			refExit, goExit, args, refErr, goErr)
	}
	if !bytes.Equal(refOut, goOut) {
		t.Errorf("stdout mismatch\nargs: %v\nref: %q\ngo:  %q", args, refOut, goOut)
	}
	if !bytes.Equal(refErr, goErr) {
		t.Errorf("stderr mismatch\nargs: %v\nref: %q\ngo:  %q", args, refErr, goErr)
	}
}

// clearOutput returns nil, used for tests where output content differs
// by design (e.g., --version) but exit code must match.
func clearOutput(b []byte) []byte {
	return nil
}
