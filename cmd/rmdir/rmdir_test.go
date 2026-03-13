// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd035-rmdir R1.1–R1.4, R2.1–R2.3 differential tests
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

// programNameNormalizer replaces the binary name (grmdir or the full go binary
// path) with the canonical name "rmdir" so stderr messages are comparable
// between the Go and reference binaries.
func programNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("rmdir"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("rmdir"))
		b = bytes.ReplaceAll(b, []byte("grmdir"), []byte("rmdir"))
		return b
	}
}

// errCaseNormalizer normalizes error message casing differences between Go's
// os package (lowercase) and GNU coreutils (title case).
var errCaseNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`(?m): [A-Z][a-z]`)
	return re.ReplaceAllFunc(b, bytes.ToLower)
}

// outputClearNormalizer replaces all output with empty bytes. Used for
// --version and --help differential tests where the content differs
// between Go and GNU binaries but the exit code must match.
var outputClearNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	// R1.3: non-empty directory error — both binaries see a non-empty
	// directory and fail identically.
	t.Run("nonempty_dir_error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		nonempty := filepath.Join(tmpDir, "nonempty")
		if mkErr := os.Mkdir(nonempty, 0o755); mkErr != nil {
			t.Fatalf("setup: %v", mkErr)
		}
		if wErr := os.WriteFile(filepath.Join(nonempty, "file.txt"), []byte("data"), 0o644); wErr != nil {
			t.Fatalf("setup: %v", wErr)
		}
		tests := []testutils.DiffTest{
			{
				Name:      "error on non-empty directory",
				Args:      []string{nonempty},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.4: non-existent directory error.
	t.Run("nonexistent_dir_error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		nonexistent := filepath.Join(tmpDir, "does_not_exist")
		tests := []testutils.DiffTest{
			{
				Name:      "error on non-existent directory",
				Args:      []string{nonexistent},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.4: not a directory error (target is a file).
	t.Run("not_a_directory_error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		aFile := filepath.Join(tmpDir, "afile")
		if wErr := os.WriteFile(aFile, []byte("data"), 0o644); wErr != nil {
			t.Fatalf("setup: %v", wErr)
		}
		tests := []testutils.DiffTest{
			{
				Name:      "error on file target",
				Args:      []string{aFile},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// Missing operand error.
	t.Run("missing_operand", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "no arguments",
				Args:      []string{},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.3: --version prints version information and exits 0.
	// Output content differs between Go and GNU; only exit code is compared.
	t.Run("version_flag", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "version exits 0",
				Args:      []string{"--version"},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  0,
				Normalize: []testutils.NormalizeFunc{outputClearNormalizer},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.4: --help prints usage information and exits 0.
	// Output content differs between Go and GNU; only exit code is compared.
	t.Run("help_flag", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "help exits 0",
				Args:      []string{"--help"},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  0,
				Normalize: []testutils.NormalizeFunc{outputClearNormalizer},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R2.2: -p on a non-existent path reports an error.
	t.Run("parents_nonexistent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		nonexistent := filepath.Join(tmpDir, "x", "y", "z")
		tests := []testutils.DiffTest{
			{
				Name:      "parents with non-existent path",
				Args:      []string{"-p", nonexistent},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestRemoveSingleEmptyDir verifies R1.1: rmdir removes a single empty directory.
// Tested independently because rmdir is destructive — RunDiffTests would run
// the ref binary first, removing the dir, causing the Go binary to fail.
func TestRemoveSingleEmptyDir(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "emptydir")
	if mkErr := os.Mkdir(target, 0o755); mkErr != nil {
		t.Fatalf("setup: %v", mkErr)
	}

	cmd := exec.Command(goBin, target)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rmdir failed: %v\noutput: %s", err, out)
	}

	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("directory %q still exists after rmdir", target)
	}
}

// TestRemoveMultipleEmptyDirs verifies R1.2: rmdir removes multiple empty
// directories when given multiple arguments.
func TestRemoveMultipleEmptyDirs(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	dirs := []string{
		filepath.Join(tmpDir, "dir_a"),
		filepath.Join(tmpDir, "dir_b"),
		filepath.Join(tmpDir, "dir_c"),
	}
	for _, d := range dirs {
		if mkErr := os.Mkdir(d, 0o755); mkErr != nil {
			t.Fatalf("setup: %v", mkErr)
		}
	}

	cmd := exec.Command(goBin, dirs...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rmdir failed: %v\noutput: %s", err, out)
	}

	for _, d := range dirs {
		if _, statErr := os.Stat(d); !os.IsNotExist(statErr) {
			t.Errorf("directory %q still exists after rmdir", d)
		}
	}
}

// TestHelp verifies R1.4: --help prints usage to stdout and exits 0.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--help produced no output")
	}
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Errorf("--help output missing 'Usage:': %s", out)
	}
}

// TestParentsRemovesAncestors verifies R2.1: rmdir -p a/b/c removes c, b, then a.
// Uses relative paths with WorkDir to avoid ascending into the tmpdir.
func TestParentsRemovesAncestors(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	if mkErr := os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c"), 0o755); mkErr != nil {
		t.Fatalf("setup: %v", mkErr)
	}

	cmd := exec.Command(goBin, "-p", "a/b/c")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("rmdir -p failed: %v\noutput: %s", runErr, out)
	}

	// Verify c, b, and a are all removed.
	for _, d := range []string{"a/b/c", "a/b", "a"} {
		if _, statErr := os.Stat(filepath.Join(tmpDir, d)); !os.IsNotExist(statErr) {
			t.Errorf("directory %q still exists after rmdir -p", d)
		}
	}
}

// TestParentsMultipleArgs verifies R2.3: rmdir -p processes each argument independently.
func TestParentsMultipleArgs(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	if mkErr := os.MkdirAll(filepath.Join(tmpDir, "x", "y"), 0o755); mkErr != nil {
		t.Fatalf("setup: %v", mkErr)
	}
	if mkErr := os.MkdirAll(filepath.Join(tmpDir, "m", "n"), 0o755); mkErr != nil {
		t.Fatalf("setup: %v", mkErr)
	}

	cmd := exec.Command(goBin, "-p", "x/y", "m/n")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("rmdir -p failed: %v\noutput: %s", runErr, out)
	}

	for _, d := range []string{"x/y", "x", "m/n", "m"} {
		if _, statErr := os.Stat(filepath.Join(tmpDir, d)); !os.IsNotExist(statErr) {
			t.Errorf("directory %q still exists after rmdir -p", d)
		}
	}
}

// TestParentsStopsOnNonempty verifies R2.2: rmdir -p stops ascending when a
// parent is not empty. Run separately per binary since -p is destructive.
func TestParentsStopsOnNonempty(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	for _, bin := range []struct {
		name string
		path string
	}{
		{"ref", refBin},
		{"go", goBin},
	} {
		t.Run(bin.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			if mkErr := os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c"), 0o755); mkErr != nil {
				t.Fatalf("setup: %v", mkErr)
			}
			if wErr := os.WriteFile(filepath.Join(tmpDir, "a", "blocker.txt"), []byte("data"), 0o644); wErr != nil {
				t.Fatalf("setup: %v", wErr)
			}

			cmd := exec.Command(bin.path, "-p", "a/b/c")
			cmd.Dir = tmpDir
			cmd.Env = append(os.Environ(), "LC_ALL=C")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()

			exitCode := 0
			if runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("run error: %v", runErr)
				}
			}

			if exitCode != 1 {
				t.Errorf("expected exit code 1, got %d", exitCode)
			}

			stderrBytes := stderr.Bytes()
			for _, fn := range normalize {
				stderrBytes = fn(stderrBytes)
			}

			// Both binaries should report the non-empty ancestor.
			if !bytes.Contains(stderrBytes, []byte("a")) {
				t.Errorf("stderr should mention ancestor 'a': %s", stderrBytes)
			}
		})
	}
}

// TestVersion verifies R1.3: --version prints version to stdout and exits 0.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--version produced no output")
	}
	if !bytes.Contains(out, []byte("rmdir")) {
		t.Errorf("--version output missing 'rmdir': %s", out)
	}
}
