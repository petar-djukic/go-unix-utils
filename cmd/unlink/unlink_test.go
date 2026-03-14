// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd038-unlink R3.1–R3.3 differential tests
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

// programNameNormalizer replaces the binary name (gunlink or the full go binary
// path) with the canonical name "unlink" so stderr messages are comparable
// between the Go and reference binaries.
func programNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("unlink"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("unlink"))
		b = bytes.ReplaceAll(b, []byte("gunlink"), []byte("unlink"))
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

// TestDiff runs differential tests against the GNU gunlink reference binary
// for error cases that are non-destructive (both binaries can run on the
// same filesystem state).
//
// R3.1: differential tests compare stdout, stderr, and exit codes.
// R3.2: covers zero-argument, multi-argument, non-existent, and directory errors.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	// R2.1: zero arguments.
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

	// R2.2: too many arguments.
	t.Run("extra_operand", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "two arguments",
				Args:      []string{"a", "b"},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R2.3: non-existent file.
	t.Run("nonexistent_file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		nonexistent := filepath.Join(tmpDir, "does_not_exist")
		tests := []testutils.DiffTest{
			{
				Name:      "non-existent file",
				Args:      []string{nonexistent},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R2.4: directory argument.
	t.Run("directory_error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		dirTarget := filepath.Join(tmpDir, "adir")
		if mkErr := os.Mkdir(dirTarget, 0o755); mkErr != nil {
			t.Fatalf("setup: %v", mkErr)
		}
		tests := []testutils.DiffTest{
			{
				Name:      "directory argument",
				Args:      []string{dirTarget},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// --version exits 0.
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

	// --help exits 0.
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
}

// runBinaryCapture executes binary with args in the given dir and returns
// stdout, stderr, and exit code.
func runBinaryCapture(t *testing.T, binary string, args []string, dir string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary %q: %v", binary, runErr)
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

// applyNormalizers applies a list of normalizer functions to a byte slice.
func applyNormalizers(b []byte, fns []testutils.NormalizeFunc) []byte {
	for _, fn := range fns {
		b = fn(b)
	}
	return b
}

// compareBinaryOutputs runs both binaries in separate temp dirs with identical
// setup, compares exit code, stdout, and stderr after normalization.
// Used for destructive operations where RunDiffTests cannot be used.
// R3.3: postCheckFn runs after both binaries execute to verify post-conditions
// (e.g., file removal) in both directories.
func compareBinaryOutputs(t *testing.T, name string, goBin, refBin string, args []string,
	setupFn func(t *testing.T, dir string), normalize []testutils.NormalizeFunc,
	postCheckFn func(t *testing.T, goDir, refDir string)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()
		setupFn(t, refDir)
		setupFn(t, goDir)

		refStdout, refStderr, refCode := runBinaryCapture(t, refBin, args, refDir)
		goStdout, goStderr, goCode := runBinaryCapture(t, goBin, args, goDir)

		refStdout = applyNormalizers(refStdout, normalize)
		goStdout = applyNormalizers(goStdout, normalize)
		refStderr = applyNormalizers(refStderr, normalize)
		goStderr = applyNormalizers(goStderr, normalize)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refStdout, goStdout)
		}
		if !bytes.Equal(refStderr, goStderr) {
			t.Errorf("stderr mismatch:\nref: %q\ngo:  %q", refStderr, goStderr)
		}

		if postCheckFn != nil {
			postCheckFn(t, goDir, refDir)
		}
	})
}

// TestDiffRemoveRegularFile verifies R3.2, R3.3: differential test for
// successful removal of a regular file. Both binaries get separate temp dirs
// since unlink is destructive.
func TestDiffRemoveRegularFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	// R3.3: postCheckFn verifies the file no longer exists after successful unlink.
	compareBinaryOutputs(t, "remove_regular_file", goBin, refBin,
		[]string{"target.txt"},
		func(t *testing.T, dir string) {
			t.Helper()
			if wErr := os.WriteFile(filepath.Join(dir, "target.txt"), []byte("data"), 0o644); wErr != nil {
				t.Fatalf("setup: %v", wErr)
			}
		},
		normalize,
		func(t *testing.T, goDir, refDir string) {
			t.Helper()
			goTarget := filepath.Join(goDir, "target.txt")
			if _, statErr := os.Lstat(goTarget); !os.IsNotExist(statErr) {
				t.Errorf("R3.3: file %q still exists after go unlink", goTarget)
			}
			refTarget := filepath.Join(refDir, "target.txt")
			if _, statErr := os.Lstat(refTarget); !os.IsNotExist(statErr) {
				t.Errorf("R3.3: file %q still exists after ref unlink", refTarget)
			}
		},
	)
}

// TestDiffRemoveSymlink verifies R3.2: differential test for successful
// removal of a symbolic link.
func TestDiffRemoveSymlink(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	// R3.3: postCheckFn verifies the symlink no longer exists after successful unlink.
	compareBinaryOutputs(t, "remove_symlink", goBin, refBin,
		[]string{"link"},
		func(t *testing.T, dir string) {
			t.Helper()
			target := filepath.Join(dir, "target.txt")
			if wErr := os.WriteFile(target, []byte("data"), 0o644); wErr != nil {
				t.Fatalf("setup: %v", wErr)
			}
			if sErr := os.Symlink(target, filepath.Join(dir, "link")); sErr != nil {
				t.Fatalf("setup: %v", sErr)
			}
		},
		normalize,
		func(t *testing.T, goDir, refDir string) {
			t.Helper()
			// R3.3: symlink itself must be removed.
			goLink := filepath.Join(goDir, "link")
			if _, statErr := os.Lstat(goLink); !os.IsNotExist(statErr) {
				t.Errorf("R3.3: symlink %q still exists after go unlink", goLink)
			}
			refLink := filepath.Join(refDir, "link")
			if _, statErr := os.Lstat(refLink); !os.IsNotExist(statErr) {
				t.Errorf("R3.3: symlink %q still exists after ref unlink", refLink)
			}
			// R3.2: symlink target must still exist (unlink removes the link, not the target).
			goTarget := filepath.Join(goDir, "target.txt")
			if _, statErr := os.Stat(goTarget); statErr != nil {
				t.Errorf("R3.2: symlink target %q was removed by go unlink (should only remove link)", goTarget)
			}
		},
	)
}

// TestRemoveRegularFile verifies R1.1, R1.2, R1.3, R3.3: the Go binary
// successfully removes a regular file, produces no stdout, and the file
// no longer exists afterward.
func TestRemoveRegularFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "testfile")
	if wErr := os.WriteFile(target, []byte("content"), 0o644); wErr != nil {
		t.Fatalf("setup: %v", wErr)
	}

	cmd := exec.Command(goBin, target)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("unlink failed: %v\nstderr: %s", runErr, stderr.String())
	}

	// R1.2: no stdout output on success.
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got: %q", stdout.String())
	}

	// R3.3: file no longer exists after successful invocation.
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("file %q still exists after unlink", target)
	}
}

// TestRemoveSymlink verifies R3.2, R3.3: the Go binary successfully removes
// a symbolic link, the link no longer exists, and the target file is preserved.
func TestRemoveSymlink(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	if wErr := os.WriteFile(target, []byte("content"), 0o644); wErr != nil {
		t.Fatalf("setup: %v", wErr)
	}
	link := filepath.Join(tmpDir, "testlink")
	if sErr := os.Symlink(target, link); sErr != nil {
		t.Fatalf("setup: %v", sErr)
	}

	cmd := exec.Command(goBin, link)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("unlink failed: %v\nstderr: %s", runErr, stderr.String())
	}

	// R1.2: no stdout output on success.
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got: %q", stdout.String())
	}

	// R3.3: symlink no longer exists after successful invocation.
	if _, statErr := os.Lstat(link); !os.IsNotExist(statErr) {
		t.Fatalf("symlink %q still exists after unlink", link)
	}

	// R3.2: symlink target must still exist.
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("symlink target %q was removed (should only remove link): %v", target, statErr)
	}
}

// TestHelp verifies --help prints usage to stdout and exits 0.
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

// TestVersion verifies --version prints version to stdout and exits 0.
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
	if !bytes.Contains(out, []byte("unlink")) {
		t.Errorf("--version output missing 'unlink': %s", out)
	}
}
