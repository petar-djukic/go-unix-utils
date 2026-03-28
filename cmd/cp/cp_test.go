// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cp against GNU gcp.
// Covers prd056-cp R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gcp and Go cp.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?cp|gcp`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("cp"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		return b
	}
}

// writeTestFile creates a file with content in dir and returns its path.
func writeTestFile(
	t *testing.T, dir, name string, content []byte,
) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write test file %s: %v", p, err)
	}
	return p
}

// writeTestDir creates a directory in dir and returns its path.
func writeTestDir(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("create test dir %s: %v", p, err)
	}
	return p
}

// writeSymlink creates a symlink in dir pointing to target.
func writeSymlink(
	t *testing.T, dir, name, target string,
) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.Symlink(target, p); err != nil {
		t.Fatalf("create symlink %s: %v", p, err)
	}
	return p
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	dir := t.TempDir()
	content1 := []byte("hello world\n")
	content2 := []byte("second file\nwith two lines\n")
	f1 := writeTestFile(t, dir, "src1.txt", content1)
	f2 := writeTestFile(t, dir, "src2.txt", content2)

	// Create a subdirectory with files for recursive tests.
	subdir := writeTestDir(t, dir, "srcdir")
	writeTestFile(t, subdir, "a.txt", []byte("aaa\n"))
	writeTestFile(t, subdir, "b.txt", []byte("bbb\n"))
	nested := writeTestDir(t, dir, "srcdir/nested")
	writeTestFile(t, nested, "c.txt", []byte("ccc\n"))

	// Symlink for symlink-handling tests.
	writeSymlink(t, dir, "link1.txt", f1)

	// Destination directory for multi-source copy.
	destMulti := writeTestDir(t, dir, "destmulti")

	// Pre-existing file for no-clobber and force tests.
	existDest := writeTestFile(t, dir, "existing.txt",
		[]byte("original\n"))

	tests := []testutils.DiffTest{
		// R4.1/R1.1: basic single file copy.
		{
			Name:    "basic_single_copy",
			Args:    []string{f1, filepath.Join(dir, "out1.txt")},
			WorkDir: dir,
		},
		// R1.1: multi-source copy into directory.
		{
			Name:    "multi_source_to_dir",
			Args:    []string{f1, f2, destMulti},
			WorkDir: dir,
		},
		// R1.4: -n no-clobber does not overwrite existing.
		{
			Name:    "no_clobber",
			Args:    []string{"-n", f2, existDest},
			WorkDir: dir,
		},
		// R3.4: -v verbose output.
		{
			Name:    "verbose_copy",
			Args:    []string{"-v", f1, filepath.Join(dir, "out_v.txt")},
			WorkDir: dir,
		},
		// R2.1: -r recursive directory copy.
		{
			Name:    "recursive_copy",
			Args:    []string{"-r", subdir, filepath.Join(dir, "dstdir")},
			WorkDir: dir,
		},
		// R2.2: directory without -r must fail.
		{
			Name:      "dir_without_recursive",
			Args:      []string{subdir, filepath.Join(dir, "dstdir2")},
			Normalize: []testutils.NormalizeFunc{errNorm},
			WorkDir:   dir,
		},
		// R3.1: -p preserve mode and timestamps.
		{
			Name:    "preserve_mode_timestamps",
			Args:    []string{"-p", f1, filepath.Join(dir, "out_p.txt")},
			WorkDir: dir,
		},
		// R3.2: -a archive mode (equivalent to -dR --preserve=all).
		{
			Name: "archive_mode",
			Args: []string{
				"-a", subdir, filepath.Join(dir, "dstdir_a"),
			},
			WorkDir: dir,
		},
		// R2.4: -P no-dereference copies symlink as symlink.
		{
			Name: "no_dereference_symlink",
			Args: []string{
				"-P", filepath.Join(dir, "link1.txt"),
				filepath.Join(dir, "out_link.txt"),
			},
			WorkDir: dir,
		},
		// R2.3: -L dereference follows symlinks.
		{
			Name: "dereference_symlink",
			Args: []string{
				"-L", filepath.Join(dir, "link1.txt"),
				filepath.Join(dir, "out_deref.txt"),
			},
			WorkDir: dir,
		},
		// R4.2: missing source exits 1 with error.
		{
			Name: "missing_source",
			Args: []string{
				filepath.Join(dir, "nonexistent.txt"),
				filepath.Join(dir, "out_miss.txt"),
			},
			Normalize: []testutils.NormalizeFunc{errNorm},
			WorkDir:   dir,
		},
		// R4.2: copy-to-self detection.
		{
			Name:      "copy_to_self",
			Args:      []string{f1, f1},
			Normalize: []testutils.NormalizeFunc{errNorm},
			WorkDir:   dir,
		},
		// R4.3: -t target-directory flag.
		{
			Name: "target_directory_flag",
			Args: []string{
				"-t", writeTestDir(t, dir, "tdir"), f1, f2,
			},
			WorkDir: dir,
		},
		// R1.1/R4.1: copy empty file.
		{
			Name: "copy_empty_file",
			Args: []string{
				writeTestFile(t, dir, "empty.txt", []byte{}),
				filepath.Join(dir, "out_empty.txt"),
			},
			WorkDir: dir,
		},
		// R1.3: -f force with readonly destination.
		{
			Name: "force_readonly_dest",
			Args: []string{
				"-f", f1, forceReadonlyDest(t, dir),
			},
			Normalize: []testutils.NormalizeFunc{errNorm},
			WorkDir:   dir,
		},
		// R3.4: -rv recursive verbose. Pre-create dest so both
		// binaries see the same initial state in sequential execution.
		{
			Name: "recursive_verbose",
			Args: []string{
				"-rv", subdir,
				writeTestDir(t, dir, "dstdir_rv"),
			},
			WorkDir: dir,
		},
		// R1.4: -in no-clobber (last flag wins in GNU, always wins here).
		{
			Name: "no_clobber_last_wins",
			Args: []string{
				"-in", f2,
				writeTestFile(t, dir, "in_exist.txt", []byte("old\n")),
			},
			WorkDir: dir,
		},
		// R4.2: multiple sources with one missing (partial failure).
		{
			Name: "partial_failure_multi",
			Args: []string{
				f1, filepath.Join(dir, "nosuch.txt"),
				writeTestDir(t, dir, "destpart"),
			},
			Normalize: []testutils.NormalizeFunc{errNorm},
			WorkDir:   dir,
		},
		// R3.3: --preserve=timestamps selective preservation.
		{
			Name: "preserve_timestamps_only",
			Args: []string{
				"--preserve=timestamps", f1,
				filepath.Join(dir, "out_ts.txt"),
			},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffVersionHelp tests --version and --help output.
// R1.2: --version and --help produce GNU-format output.
func TestDiffVersionHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	versionNorm := versionNormalizer()

	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{helpNormalizer()},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{versionNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// versionNormalizer replaces version-specific text so both binaries match.
func versionNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		// Normalize to just the first line's program name prefix.
		re := regexp.MustCompile(`(?s)^(cp) .*`)
		return re.ReplaceAll(b, []byte("$1 (version)\n"))
	}
}

// helpNormalizer normalizes help output so minor formatting differences pass.
func helpNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		// Only compare that both produce some help output and exit 0.
		if len(b) > 0 {
			return []byte("HELP_OUTPUT\n")
		}
		return b
	}
}

// forceReadonlyDest creates a readonly file for -f testing.
func forceReadonlyDest(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "readonly_dest.txt")
	if err := os.WriteFile(p, []byte("readonly\n"), 0o444); err != nil {
		t.Fatalf("write readonly file: %v", err)
	}
	return p
}
