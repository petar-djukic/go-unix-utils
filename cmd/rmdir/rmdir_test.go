// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against grmdir (GNU coreutils).
// Implements prd035-rmdir R1.1-R1.4, R2.1-R2.3 test coverage.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	// Differential tests for error cases where no directory state changes.
	tests := []testutils.DiffTest{
		// R1.2: non-existent directory — error, exit 1.
		{
			Name:      "R1.2_nonexistent",
			Args:      []string{"nosuchdir"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
		// No arguments — error, exit 1.
		{
			Name:      "no_args_error",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Filesystem-modifying tests run each binary independently with fresh state.

	// R1.1: remove a single empty directory, exit 0.
	t.Run("R1.1_remove_empty_dir", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"emptydir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.1: remove multiple empty directories.
	t.Run("R1.1_remove_multiple", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"dir1", "dir2"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "dir1"), 0o755) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "dir2"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.2: non-empty directory — error, exit 1.
	t.Run("R1.2_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"nonemptydir"}, 1, func(dir string) {
			p := filepath.Join(dir, "nonemptydir")
			os.Mkdir(p, 0o755)                                                     //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(p, "file.txt"), []byte("data"), 0o644)       //nolint:errcheck // test setup
		})
	})

	// R1.2: argument is a regular file — error, exit 1.
	t.Run("R1.2_not_a_directory", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"afile"}, 1, func(dir string) {
			os.WriteFile(filepath.Join(dir, "afile"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R1.3: --ignore-fail-on-non-empty suppresses non-empty errors.
	t.Run("R1.3_ignore_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nonemptydir"}, 0, func(dir string) {
			p := filepath.Join(dir, "nonemptydir")
			os.Mkdir(p, 0o755)                                                     //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(p, "file.txt"), []byte("data"), 0o644)       //nolint:errcheck // test setup
		})
	})

	// R1.4: -p removes parent directory components.
	t.Run("R1.4_parents_short", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"-p", "a/b/c"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.4: --parents long form.
	t.Run("R1.4_parents_long", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"--parents", "x/y"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "x", "y"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.4: -p with non-empty parent — partial removal, exit 1.
	t.Run("R1.4_parents_partial_fail", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "a/b/c"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                       //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644)     //nolint:errcheck // test setup
		})
	})

	// R1.3 + R1.4: --ignore-fail-on-non-empty with -p.
	t.Run("R1.3_R1.4_ignore_parents", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "--ignore-fail-on-non-empty", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                       //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644)     //nolint:errcheck // test setup
		})
	})

	// R2.3: -p with multiple arguments processed independently.
	t.Run("R2.3_parents_multiple_args", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"-p", "a/b/c", "x/y"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
			os.MkdirAll(filepath.Join(dir, "x", "y"), 0o755)      //nolint:errcheck // test setup
		})
	})

	// R2.1: -p on a nonexistent path — error, exit 1.
	t.Run("R2.1_parents_nonexistent", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "no/such/path"}, 1, func(dir string) {
			// no setup — path does not exist
		})
	})

	// R2.2: -p where intermediate ancestor is non-empty — partial removal, exit 1.
	// Deepest child and middle dir removed, top dir kept because it has a file.
	t.Run("R2.2_parents_intermediate_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "a/b/c"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                   //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "other.txt"), []byte("x"), 0o644)   //nolint:errcheck // test setup
		})
	})

	// R2.3: -p with multiple args where one fails and one succeeds — exit 1.
	t.Run("R2.3_parents_mixed_success_fail", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "good/sub", "bad/sub"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "good", "sub"), 0o755)                    //nolint:errcheck // test setup
			os.MkdirAll(filepath.Join(dir, "bad", "sub"), 0o755)                     //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "bad", "file.txt"), []byte("x"), 0o644)  //nolint:errcheck // test setup
		})
	})
}

// assertBothRemoveDir runs both binaries with fresh setup and verifies both
// exit 0 with matching exit codes.
func assertBothRemoveDir(t *testing.T, goBin, refBin string, args []string, setup func(dir string)) {
	t.Helper()

	refExit := runWithSetup(t, refBin, args, setup)
	goExit := runWithSetup(t, goBin, args, setup)

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d, args=%v", refExit, goExit, args)
	}
	if goExit != 0 {
		t.Errorf("expected exit 0, got %d, args=%v", goExit, args)
	}
}

// assertBothExitCode runs both binaries with fresh setup and verifies both
// produce the expected exit code.
func assertBothExitCode(t *testing.T, goBin, refBin string, args []string, wantExit int, setup func(dir string)) {
	t.Helper()

	refExit := runWithSetup(t, refBin, args, setup)
	goExit := runWithSetup(t, goBin, args, setup)

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d, args=%v", refExit, goExit, args)
	}
	if goExit != wantExit {
		t.Errorf("expected exit %d, got %d, args=%v", wantExit, goExit, args)
	}
}

// runWithSetup creates a fresh temp directory, runs setup, then executes the
// binary. Returns the exit code.
func runWithSetup(t *testing.T, binary string, args []string, setup func(dir string)) int {
	t.Helper()

	dir := t.TempDir()
	setup(dir)

	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	t.Fatalf("failed to execute %s: %v", binary, err)
	return -1
}

// normalizeAllOutput replaces all output with empty bytes so that only
// exit codes are compared. Used for error messages where the exact format
// may differ between implementations.
func normalizeAllOutput(b []byte) []byte {
	return nil
}
