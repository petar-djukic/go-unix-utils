// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/truncate comparing against gtruncate (GNU coreutils).
// Covers prd083-truncate R1.1-R1.4, R2.1-R2.2, R3.1-R3.3.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// stderrNormalizer normalizes error messages between gtruncate and Go truncate.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?truncate|gtruncate`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`,
	)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("truncate"))
		b = tryHelp.ReplaceAll(b, nil)
		b = bytes.ToLower(b)
		return b
	}
}

// runBin runs a binary with args in the given working directory.
func runBin(
	t *testing.T, binary string, args []string, dir string,
) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
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
			t.Fatalf("failed to run %s: %v", binary, runErr)
		}
	}
	return cmdResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// compareResults asserts that ref and go binary outputs match.
func compareResults(t *testing.T, name string, ref, got cmdResult) {
	t.Helper()
	norm := stderrNormalizer()
	if !bytes.Equal(ref.stdout, got.stdout) {
		t.Errorf("[%s] stdout mismatch\n  ref: %q\n  go:  %q",
			name, ref.stdout, got.stdout)
	}
	refErr := norm(ref.stderr)
	goErr := norm(got.stderr)
	if !bytes.Equal(refErr, goErr) {
		t.Errorf("[%s] stderr mismatch\n  ref: %q\n  go:  %q",
			name, refErr, goErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("[%s] exit code mismatch: ref=%d go=%d",
			name, ref.exitCode, got.exitCode)
	}
}

// writeFile creates a file with given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(
		path, []byte(content), 0o644,
	); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// fileSize returns the size of a file.
func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}

// fileExists reports whether a file exists at path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// assertFileSize asserts that the file at path has the expected size.
func assertFileSize(
	t *testing.T, path string, want int64, label string,
) {
	t.Helper()
	got := fileSize(t, path)
	if got != want {
		t.Errorf("[%s] file size %s: want %d, got %d",
			label, path, want, got)
	}
}

// truncateTest runs both binaries in separate directories with
// identical setup, then compares outputs and resulting file sizes.
func truncateTest(
	t *testing.T, name string,
	goBin, refBin string,
	args []string,
	setup func(t *testing.T, dir string),
	checkFiles func(t *testing.T, refDir, goDir, label string),
) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		refDir := t.TempDir()
		goDir := t.TempDir()

		if setup != nil {
			setup(t, refDir)
			setup(t, goDir)
		}

		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)
		compareResults(t, name, refRes, goRes)

		if checkFiles != nil {
			checkFiles(t, refDir, goDir, name)
		}
	})
}

// TestDiffErrors tests error cases using RunDiffTests directly.
// R3.1: error diagnostics to stderr. R3.2: invalid argument handling.
func TestDiffErrors(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			Name:      "missing_size_and_ref",
			Args:      []string{"somefile"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			// R3.2: invalid size argument.
			Name:      "invalid_size",
			Args:      []string{"-s", "INVALID", "somefile"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestErrorNonexistentPath verifies R3.1: error on nonexistent parent
// directory prints diagnostic to stderr and exits 1.
func TestErrorNonexistentPath(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "nonexistent_path", goBin, refBin,
		[]string{"-s", "100", "/nonexistent/dir/file"},
		nil,
		nil,
	)
}

// TestAbsoluteSize verifies R1.1: -s SIZE sets file to SIZE bytes.
func TestAbsoluteSize(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "absolute_100", goBin, refBin,
		[]string{"-s", "100", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, filepath.Join(dir, "testfile"), "hello")
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestAbsoluteCreatesFile verifies R1.1/R1.4: missing file is created.
func TestAbsoluteCreatesFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "create_file", goBin, refBin,
		[]string{"-s", "1000", "newfile"},
		nil, // no setup — file should be created
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "newfile"))
			assertFileSize(t, filepath.Join(goDir, "newfile"),
				refSz, label)
		},
	)
}

// TestRelativeGrow verifies R1.2: +SIZE extends file.
func TestRelativeGrow(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "grow_500", goBin, refBin,
		[]string{"-s", "+500", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, filepath.Join(dir, "testfile"),
				"hello world test data")
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestRelativeShrink verifies R1.2: -SIZE shrinks file.
func TestRelativeShrink(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	// Create a 1000-byte file and shrink by 500.
	truncateTest(t, "shrink_500", goBin, refBin,
		[]string{"-s", "-500", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 1000)
			for i := range data {
				data[i] = 'A'
			}
			os.WriteFile(
				filepath.Join(dir, "testfile"), data, 0o644,
			)
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestAtMost verifies R1.2: <SIZE sets at most SIZE.
func TestAtMost(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	// File is 2000 bytes, at most 1000 → shrinks to 1000.
	truncateTest(t, "at_most_shrinks", goBin, refBin,
		[]string{"-s", "<1000", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 2000)
			os.WriteFile(
				filepath.Join(dir, "testfile"), data, 0o644,
			)
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)

	// File is 500 bytes, at most 1000 → stays at 500.
	truncateTest(t, "at_most_noop", goBin, refBin,
		[]string{"-s", "<1000", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 500)
			os.WriteFile(
				filepath.Join(dir, "testfile"), data, 0o644,
			)
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestAtLeast verifies R1.2: >SIZE sets at least SIZE.
func TestAtLeast(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	// File is 500 bytes, at least 1000 → grows to 1000.
	truncateTest(t, "at_least_grows", goBin, refBin,
		[]string{"-s", ">1000", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 500)
			os.WriteFile(
				filepath.Join(dir, "testfile"), data, 0o644,
			)
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestRoundDown verifies R1.2: /SIZE rounds down to multiple.
func TestRoundDown(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	// File is 1500 bytes, round down to multiple of 1024.
	truncateTest(t, "round_down_1024", goBin, refBin,
		[]string{"-s", "/1024", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 1500)
			os.WriteFile(
				filepath.Join(dir, "testfile"), data, 0o644,
			)
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestRoundUp verifies R1.2: %SIZE rounds up to multiple.
func TestRoundUp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	// File is 1500 bytes, round up to multiple of 1024.
	truncateTest(t, "round_up_1024", goBin, refBin,
		[]string{"-s", "%1024", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 1500)
			os.WriteFile(
				filepath.Join(dir, "testfile"), data, 0o644,
			)
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestReferenceFile verifies R2.1: -r uses RFILE's size.
func TestReferenceFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "reference_only", goBin, refBin,
		[]string{"-r", "reffile", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 750)
			os.WriteFile(
				filepath.Join(dir, "reffile"), data, 0o644,
			)
			writeFile(t, filepath.Join(dir, "testfile"), "small")
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestReferenceWithSize verifies R1.4/R2.1: -r combined with -s.
func TestReferenceWithSize(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "ref_plus_100", goBin, refBin,
		[]string{"-r", "reffile", "-s", "+100", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			data := make([]byte, 500)
			os.WriteFile(
				filepath.Join(dir, "reffile"), data, 0o644,
			)
			writeFile(t, filepath.Join(dir, "testfile"), "small")
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestNoCreate verifies R1.4: -c suppresses file creation.
func TestNoCreate(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "no_create", goBin, refBin,
		[]string{"-c", "-s", "100", "nonexistent"},
		nil,
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			if fileExists(filepath.Join(goDir, "nonexistent")) {
				t.Errorf("[%s] file should not exist with -c",
					label)
			}
			if fileExists(filepath.Join(refDir, "nonexistent")) {
				t.Errorf("[%s] ref file should not exist with -c",
					label)
			}
		},
	)
}

// TestMultipleFiles verifies R1.3: multiple file operands.
func TestMultipleFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "multiple_files", goBin, refBin,
		[]string{"-s", "200", "a.txt", "b.txt"},
		func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, filepath.Join(dir, "a.txt"), "aaa")
			writeFile(t, filepath.Join(dir, "b.txt"), "bbb")
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			for _, f := range []string{"a.txt", "b.txt"} {
				refSz := fileSize(t,
					filepath.Join(refDir, f))
				assertFileSize(t,
					filepath.Join(goDir, f), refSz, label+"/"+f)
			}
		},
	)
}

// TestSizeSuffix verifies R1.1: unit suffixes (K=1024).
func TestSizeSuffix(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skipf("reference binary gtruncate not in PATH: %v", err)
	}

	truncateTest(t, "size_1K", goBin, refBin,
		[]string{"-s", "1K", "testfile"},
		func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, filepath.Join(dir, "testfile"), "data")
		},
		func(t *testing.T, refDir, goDir, label string) {
			t.Helper()
			refSz := fileSize(t, filepath.Join(refDir, "testfile"))
			assertFileSize(t, filepath.Join(goDir, "testfile"),
				refSz, label)
		},
	)
}

// TestVersion verifies R3.3: --version outputs version to stdout, exits 0.
func TestVersion(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	res := runBin(t, goBin, []string{"--version"}, t.TempDir())
	if res.exitCode != 0 {
		t.Errorf("--version exit code: want 0, got %d", res.exitCode)
	}
	out := string(res.stdout)
	if !strings.Contains(out, "truncate") {
		t.Errorf("--version stdout should contain 'truncate': %q", out)
	}
}

// TestHelp verifies R3.3: --help outputs usage to stdout, exits 0.
func TestHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	res := runBin(t, goBin, []string{"--help"}, t.TempDir())
	if res.exitCode != 0 {
		t.Errorf("--help exit code: want 0, got %d", res.exitCode)
	}
	out := string(res.stdout)
	if !strings.Contains(out, "Usage:") {
		t.Errorf("--help stdout should contain 'Usage:': %q", out)
	}
}
