// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cp against gcp (GNU coreutils).
// Implements srd056 R4.4 (differential testing) for R1.1-R1.4, R2.1-R2.4,
// R3.1-R3.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gcp"

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

// mkTestDir creates a subdirectory in dir.
func mkTestDir(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to create test dir %s: %v", path, err)
	}
}

// mkSymlink creates a symbolic link in dir.
func mkSymlink(t *testing.T, dir, name, target string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("failed to create symlink %s -> %s: %v", path, target, err)
	}
}

// programNameRe matches a program name or path prefix before ": " in error lines.
var programNameRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// tryHelpRe matches "Try 'BINARY --help'" with any binary path.
var tryHelpRe = regexp.MustCompile(`Try '[^']+' for`)

// stderrNormalizer replaces program name/path prefixes in error output
// so that gcp and cp produce identical normalized stderr.
func stderrNormalizer(data []byte) []byte {
	data = programNameRe.ReplaceAll(data, []byte("cp:"))
	data = tryHelpRe.ReplaceAll(data, []byte("Try 'cp --help' for"))
	return data
}

// pathPrefixRe replaces the full binary path in interactive prompts.
var pathPrefixRe = regexp.MustCompile(`/[^\s:]+/g?cp:`)

// promptNormalizer replaces full binary paths in interactive prompts.
func promptNormalizer(data []byte) []byte {
	data = pathPrefixRe.ReplaceAll(data, []byte("cp:"))
	return bytes.ReplaceAll(data, []byte("gcp:"), []byte("cp:"))
}

// TestDiff runs differential tests comparing cmd/cp against gcp.
// R4.4: covers single file copy, multi-file copy, -i, -f, -n, -p, -a, -v,
// --preserve, symlink handling, and error cases.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	norm := []testutils.NormalizeFunc{stderrNormalizer}
	promptNorm := []testutils.NormalizeFunc{promptNormalizer}

	// R1.1: single file copy
	t.Run("R1.1_single_file_copy", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "hello world\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "copy",
			Args:    []string{"src.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R1.1: copy overwrites existing destination by default
	t.Run("R1.1_overwrite_existing", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "new content\n")
		writeTestFile(t, dir, "dest.txt", "old content\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "overwrite",
			Args:    []string{"src.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R1.1: multi-file copy into directory
	t.Run("R1.1_multi_file_copy", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "a.txt", "file a\n")
		writeTestFile(t, dir, "b.txt", "file b\n")
		mkTestDir(t, dir, "destdir")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "multi",
			Args:    []string{"a.txt", "b.txt", "destdir"},
			WorkDir: dir,
		}})
	})

	// R1.1: single file copy into existing directory
	t.Run("R1.1_copy_into_directory", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "data\n")
		mkTestDir(t, dir, "destdir")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "into_dir",
			Args:    []string{"src.txt", "destdir"},
			WorkDir: dir,
		}})
	})

	// R1.1: multi-file copy, target not a directory
	t.Run("R1.1_target_not_directory", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "a.txt", "file a\n")
		writeTestFile(t, dir, "b.txt", "file b\n")
		writeTestFile(t, dir, "notdir", "not a dir\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "notdir",
			Args:      []string{"a.txt", "b.txt", "notdir"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: norm,
		}})
	})

	// R1.1: missing source file
	t.Run("R1.1_missing_source", func(t *testing.T) {
		dir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "missing",
			Args:      []string{"nonexistent.txt", "dest.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: norm,
		}})
	})

	// R1.4: no-clobber with existing destination
	t.Run("R1.4_no_clobber_existing", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "new content\n")
		writeTestFile(t, dir, "dest.txt", "old content\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "noclobber",
			Args:    []string{"-n", "src.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R1.4: no-clobber with non-existing destination (should copy)
	t.Run("R1.4_no_clobber_new", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "content\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "noclobber_new",
			Args:    []string{"-n", "src.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R1.3: force copy with existing writable destination
	t.Run("R1.3_force", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "new data\n")
		writeTestFile(t, dir, "dest.txt", "old data\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "force",
			Args:    []string{"-f", "src.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R1.2: interactive with "y" answer
	t.Run("R1.2_interactive_yes", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "new data\n")
		writeTestFile(t, dir, "dest.txt", "old data\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "interactive_yes",
			Args:      []string{"-i", "src.txt", "dest.txt"},
			Stdin:     []byte("y\n"),
			WorkDir:   dir,
			Normalize: promptNorm,
		}})
	})

	// R1.2: interactive with "n" answer
	t.Run("R1.2_interactive_no", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "new data\n")
		writeTestFile(t, dir, "dest.txt", "old data\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "interactive_no",
			Args:      []string{"-i", "src.txt", "dest.txt"},
			Stdin:     []byte("n\n"),
			WorkDir:   dir,
			Normalize: promptNorm,
		}})
	})

	// R1.4: no-clobber overrides interactive
	t.Run("R1.4_noclobber_overrides_interactive", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "new data\n")
		writeTestFile(t, dir, "dest.txt", "old data\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "in_precedence",
			Args:    []string{"-i", "-n", "src.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R1.1: missing operand
	t.Run("R1.1_missing_operand", func(t *testing.T) {
		dir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "no_args",
			Args:      []string{},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: norm,
		}})
	})

	// R1.1: missing destination operand
	t.Run("R1.1_missing_dest", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "data\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "one_arg",
			Args:      []string{"src.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: norm,
		}})
	})

	// R2.2: directory without -r produces error
	t.Run("R2.2_dir_without_recursive", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/file.txt", "content\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "dir_no_r",
			Args:      []string{"srcdir", "destdir"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: norm,
		}})
	})

	// R2.1: recursive directory copy
	t.Run("R2.1_recursive_copy", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/a.txt", "file a\n")
		writeTestFile(t, dir, "srcdir/b.txt", "file b\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "recursive",
			Args:    []string{"-r", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R2.1: recursive copy with nested subdirectories
	t.Run("R2.1_recursive_nested", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir/sub1")
		mkTestDir(t, dir, "srcdir/sub2")
		writeTestFile(t, dir, "srcdir/top.txt", "top\n")
		writeTestFile(t, dir, "srcdir/sub1/deep.txt", "deep\n")
		writeTestFile(t, dir, "srcdir/sub2/other.txt", "other\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "nested",
			Args:    []string{"-r", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R2.1: recursive copy into existing directory
	t.Run("R2.1_recursive_into_existing", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/a.txt", "file a\n")
		mkTestDir(t, dir, "destdir")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "r_into_dir",
			Args:    []string{"-r", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R2.3: -L follows symlinks (copies target content, not symlink)
	t.Run("R2.3_dereference_file", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "real.txt", "real content\n")
		mkSymlink(t, dir, "link.txt", "real.txt")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "deref",
			Args:    []string{"-L", "link.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R2.3: -rL follows symlinks within directory during recursive copy
	t.Run("R2.3_recursive_dereference", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/real.txt", "data\n")
		mkSymlink(t, dir, "srcdir/link.txt", "real.txt")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "rL_deref",
			Args:    []string{"-rL", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R2.4: -rP preserves symlinks in directory copy
	t.Run("R2.4_no_deref_preserves_symlink", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/real.txt", "data\n")
		mkSymlink(t, dir, "srcdir/link.txt", "real.txt")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "preserve_link",
			Args:    []string{"-rP", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R2.4: -r defaults to -P (preserves symlinks)
	t.Run("R2.4_recursive_default_noderef", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/real.txt", "data\n")
		mkSymlink(t, dir, "srcdir/link.txt", "real.txt")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "r_default_P",
			Args:    []string{"-r", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R2.1: -R long form works same as -r
	t.Run("R2.1_uppercase_R", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/a.txt", "file a\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "upper_R",
			Args:    []string{"-R", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R3.1: -p preserves mode and timestamps
	t.Run("R3.1_preserve_p", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "preserve test\n")
		if err := os.Chmod(filepath.Join(dir, "src.txt"), 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "preserve",
			Args:      []string{"-p", "src.txt", "dest.txt"},
			WorkDir:   dir,
			Normalize: norm,
		}})
	})

	// R3.1: -p with recursive directory copy
	t.Run("R3.1_preserve_recursive", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/a.txt", "data\n")
		if err := os.Chmod(filepath.Join(dir, "srcdir/a.txt"), 0o700); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "preserve_r",
			Args:      []string{"-rp", "srcdir", "destdir"},
			WorkDir:   dir,
			Normalize: norm,
		}})
	})

	// R3.2: -a archive mode (recursive + preserve all + no-dereference)
	t.Run("R3.2_archive", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/a.txt", "file a\n")
		mkSymlink(t, dir, "srcdir/link.txt", "a.txt")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "archive",
			Args:      []string{"-a", "srcdir", "destdir"},
			WorkDir:   dir,
			Normalize: norm,
		}})
	})

	// R3.2: -a preserves file mode
	t.Run("R3.2_archive_mode", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/exec.sh", "#!/bin/sh\n")
		if err := os.Chmod(filepath.Join(dir, "srcdir/exec.sh"), 0o755); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "archive_mode",
			Args:      []string{"-a", "srcdir", "destdir"},
			WorkDir:   dir,
			Normalize: norm,
		}})
	})

	// R3.3: --preserve=mode,timestamps
	t.Run("R3.3_preserve_mode_timestamps", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "data\n")
		if err := os.Chmod(filepath.Join(dir, "src.txt"), 0o700); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "preserve_mt",
			Args:      []string{"--preserve=mode,timestamps", "src.txt", "dest.txt"},
			WorkDir:   dir,
			Normalize: norm,
		}})
	})

	// R3.3: --preserve=timestamps only
	t.Run("R3.3_preserve_timestamps_only", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "timestamp test\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "preserve_ts",
			Args:      []string{"--preserve=timestamps", "src.txt", "dest.txt"},
			WorkDir:   dir,
			Normalize: norm,
		}})
	})

	// R3.4: -v verbose single file copy
	t.Run("R3.4_verbose_single", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "hello\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "verbose",
			Args:    []string{"-v", "src.txt", "dest.txt"},
			WorkDir: dir,
		}})
	})

	// R3.4: -v verbose multi-file copy
	t.Run("R3.4_verbose_multi", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "a.txt", "file a\n")
		writeTestFile(t, dir, "b.txt", "file b\n")
		mkTestDir(t, dir, "destdir")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "verbose_multi",
			Args:    []string{"-v", "a.txt", "b.txt", "destdir"},
			WorkDir: dir,
		}})
	})

	// R3.4: -rv verbose recursive copy (pre-create destdir so both
	// binaries see the same initial state in the shared WorkDir)
	t.Run("R3.4_verbose_recursive", func(t *testing.T) {
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/a.txt", "file a\n")
		mkTestDir(t, dir, "destdir")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:    "verbose_r",
			Args:    []string{"-rv", "srcdir", "destdir"},
			WorkDir: dir,
		}})
	})

	// R3.4: -v verbose with preserve
	t.Run("R3.4_verbose_preserve", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "data\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{{
			Name:      "verbose_p",
			Args:      []string{"-vp", "src.txt", "dest.txt"},
			WorkDir:   dir,
			Normalize: norm,
		}})
	})
}

// TestPreserveAttributes verifies that preservation actually sets attributes.
func TestPreserveAttributes(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// R3.1: verify mode is preserved with -p
	t.Run("mode_preserved", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "src.txt")
		if err := os.WriteFile(srcPath, []byte("data\n"), 0o755); err != nil {
			t.Fatalf("write: %v", err)
		}
		cmd := exec.Command(goBin, "-p", "src.txt", "dest.txt")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp -p failed: %v\n%s", err, out)
		}
		destInfo, err := os.Stat(filepath.Join(dir, "dest.txt"))
		if err != nil {
			t.Fatalf("stat dest: %v", err)
		}
		if destInfo.Mode().Perm() != 0o755 {
			t.Errorf("mode: got %o, want 0755", destInfo.Mode().Perm())
		}
	})

	// R3.3: verify --preserve=mode sets mode
	t.Run("preserve_mode_flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcPath := filepath.Join(dir, "src.txt")
		if err := os.WriteFile(srcPath, []byte("data\n"), 0o700); err != nil {
			t.Fatalf("write: %v", err)
		}
		cmd := exec.Command(goBin, "--preserve=mode", "src.txt", "dest.txt")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp --preserve=mode failed: %v\n%s", err, out)
		}
		destInfo, err := os.Stat(filepath.Join(dir, "dest.txt"))
		if err != nil {
			t.Fatalf("stat dest: %v", err)
		}
		if destInfo.Mode().Perm() != 0o700 {
			t.Errorf("mode: got %o, want 0700", destInfo.Mode().Perm())
		}
	})

	// R3.4: verify verbose output format
	t.Run("verbose_output", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTestFile(t, dir, "src.txt", "data\n")
		cmd := exec.Command(goBin, "-v", "src.txt", "dest.txt")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("cp -v failed: %v", err)
		}
		expected := "'src.txt' -> 'dest.txt'\n"
		if string(out) != expected {
			t.Errorf("verbose: got %q, want %q", out, expected)
		}
	})

	// R3.2: verify -a copies symlinks as symlinks
	t.Run("archive_symlinks", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mkTestDir(t, dir, "srcdir")
		writeTestFile(t, dir, "srcdir/real.txt", "data\n")
		mkSymlink(t, dir, "srcdir/link.txt", "real.txt")
		cmd := exec.Command(goBin, "-a", "srcdir", "destdir")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp -a failed: %v\n%s", err, out)
		}
		linkPath := filepath.Join(dir, "destdir", "link.txt")
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("readlink: %v", err)
		}
		if target != "real.txt" {
			t.Errorf("symlink target: got %q, want %q", target, "real.txt")
		}
	})
}
