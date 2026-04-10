// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mv against gmv (GNU coreutils).
// Implements srd057 differential testing for R1.1-R1.4.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gmv"

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// writeFile creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// mkDir creates a subdirectory in dir.
func mkDir(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

// programNameRe matches the program name prefix in error output.
var programNameRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// tryHelpRe matches "Try 'BINARY --help'" with any binary path.
var tryHelpRe = regexp.MustCompile(`Try '[^']+' for`)

// normalizeMv normalizes program name in stderr output.
func normalizeMv(data []byte) []byte {
	data = programNameRe.ReplaceAll(data, []byte("mv:"))
	data = tryHelpRe.ReplaceAll(data, []byte("Try 'mv --help' for"))
	return data
}

// runBin executes a binary and captures its output.
func runBin(t *testing.T, bin string, args []string, dir string) binResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	err := cmd.Run()
	code := extractExitCode(t, err, bin)
	return binResult{
		stdout: stdout.Bytes(),
		stderr: stderr.Bytes(),
		exitCode: code,
	}
}

// extractExitCode gets the exit code from a command result.
func extractExitCode(t *testing.T, err error, bin string) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	t.Fatalf("failed to execute %s: %v", bin, err)
	return -1
}

// runMvDiff runs a differential test with separate temp dirs.
// Creates identical directory state for ref and go binaries,
// then compares stdout, stderr, and exit code.
func runMvDiff(t *testing.T, goBin, refBin string,
	setup func(t *testing.T, dir string), args []string) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if setup != nil {
		setup(t, refDir)
		setup(t, goDir)
	}
	ref := runBin(t, refBin, args, refDir)
	got := runBin(t, goBin, args, goDir)
	compareMvResults(t, args, ref, got)
}

// compareMvResults compares stdout, stderr, and exit code.
func compareMvResults(t *testing.T, args []string,
	ref, got binResult) {
	t.Helper()
	if !bytes.Equal(ref.stdout, got.stdout) {
		t.Errorf("stdout mismatch\nargs: %v\nref:  %q\ngot:  %q",
			args, ref.stdout, got.stdout)
	}
	refErr := normalizeMv(ref.stderr)
	gotErr := normalizeMv(got.stderr)
	if !bytes.Equal(refErr, gotErr) {
		t.Errorf("stderr mismatch\nargs: %v\nref:  %q\ngot:  %q",
			args, refErr, gotErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch\nargs: %v\nref: %d\ngot: %d",
			args, ref.exitCode, got.exitCode)
	}
}

// TestDiff runs differential tests comparing cmd/mv against gmv.
// D2: uses exec.LookPath("gmv") and t.Skip if not found.
// D3: uses testutils.BuildBinary(t, ".") to compile the Go binary.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v",
			refBinName, err)
	}
	t.Run("R1.1", func(t *testing.T) { testR1_1(t, goBin, refBin) })
	t.Run("R1.2", func(t *testing.T) { testR1_2(t, goBin, refBin) })
	t.Run("R1.3", func(t *testing.T) { testR1_3(t, goBin, refBin) })
	t.Run("R1.4", func(t *testing.T) { testR1_4(t, goBin, refBin) })
}

// testR1_1 tests single file rename.
// R1.1: rename SOURCE to DEST on same filesystem.
func testR1_1(t *testing.T, goBin, refBin string) {
	t.Run("single_rename", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "src.txt", "hello\n")
			},
			[]string{"src.txt", "dest.txt"},
		)
	})
	t.Run("overwrite_existing", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "src.txt", "new\n")
				writeFile(t, dir, "dest.txt", "old\n")
			},
			[]string{"src.txt", "dest.txt"},
		)
	})
	t.Run("missing_source", func(t *testing.T) {
		runMvDiff(t, goBin, refBin, nil,
			[]string{"nonexistent.txt", "dest.txt"},
		)
	})
}

// testR1_2 tests multi-file move into directory.
// R1.2: move multiple SOURCEs into DEST directory.
func testR1_2(t *testing.T, goBin, refBin string) {
	t.Run("multi_into_dir", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a.txt", "file a\n")
				writeFile(t, dir, "b.txt", "file b\n")
				mkDir(t, dir, "destdir")
			},
			[]string{"a.txt", "b.txt", "destdir"},
		)
	})
	t.Run("multi_target_not_dir", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a.txt", "file a\n")
				writeFile(t, dir, "b.txt", "file b\n")
				writeFile(t, dir, "notdir", "not a dir\n")
			},
			[]string{"a.txt", "b.txt", "notdir"},
		)
	})
}

// testR1_3 tests directory move without -r flag.
// R1.3: directories are moved without requiring a recursive flag.
func testR1_3(t *testing.T, goBin, refBin string) {
	t.Run("move_directory", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "srcdir")
				writeFile(t, dir, "srcdir/file.txt", "content\n")
			},
			[]string{"srcdir", "destdir"},
		)
	})
	t.Run("move_nested_directory", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "srcdir/sub")
				writeFile(t, dir, "srcdir/a.txt", "top\n")
				writeFile(t, dir, "srcdir/sub/b.txt", "deep\n")
			},
			[]string{"srcdir", "destdir"},
		)
	})
}

// testR1_4 tests move into existing directory.
// R1.4: when DEST exists and is a directory, move SOURCE into it.
func testR1_4(t *testing.T, goBin, refBin string) {
	t.Run("file_into_existing_dir", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "src.txt", "data\n")
				mkDir(t, dir, "destdir")
			},
			[]string{"src.txt", "destdir"},
		)
	})
	t.Run("dir_into_existing_dir", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "srcdir")
				writeFile(t, dir, "srcdir/file.txt", "content\n")
				mkDir(t, dir, "destdir")
			},
			[]string{"srcdir", "destdir"},
		)
	})
}

// TestErrors tests error handling for mv.
func TestErrors(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v",
			refBinName, err)
	}
	t.Run("no_args", func(t *testing.T) {
		runMvDiff(t, goBin, refBin, nil, []string{})
	})
	t.Run("one_arg", func(t *testing.T) {
		runMvDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "src.txt", "data\n")
			},
			[]string{"src.txt"},
		)
	})
}
