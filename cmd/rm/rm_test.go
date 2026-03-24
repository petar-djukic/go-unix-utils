// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rm against GNU grm.
// Covers prd058-rm R1.1-R1.4 (basic removal), R2.1-R2.4 (recursive/force/dir),
// R3.1-R3.4 (interactive modes, verbose, --interactive=WHEN).
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

// rmTestCase describes a differential test for rm.
// setup creates test fixtures in workDir and returns args for the binary.
type rmTestCase struct {
	name  string
	setup func(t *testing.T, workDir string) []string
}

// stderrNormalizer normalizes error messages between GNU grm and Go rm.
func stderrNormalizer(b []byte) []byte {
	binPath := regexp.MustCompile(`/[^\s:]+/g?rm|grm`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	isDir := regexp.MustCompile(`(?i)is a directory`)
	dirNotEmpty := regexp.MustCompile(`(?i)directory not empty`)
	b = binPath.ReplaceAll(b, []byte("rm"))
	b = tryHelp.ReplaceAll(b, nil)
	b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
	b = isDir.ReplaceAll(b, []byte("Is a directory"))
	b = dirNotEmpty.ReplaceAll(b, []byte("Directory not empty"))
	return b
}

// writeFile creates a file with content in dir and returns its path.
func writeFile(
	t *testing.T, dir, name string, content []byte,
) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write test file %s: %v", p, err)
	}
	return p
}

// makeDir creates a directory in dir and returns its path.
func makeDir(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("create test dir %s: %v", p, err)
	}
	return p
}

// pathExists reports whether path exists on disk.
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// binResult holds the captured output of a single binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary with args in workDir and captures output.
func runBin(
	t *testing.T, binary string, args []string, workDir string,
) binResult {
	t.Helper()
	return runBinWithStdin(t, binary, args, workDir, "")
}

// runBinWithStdin executes a binary with args and stdin piped.
func runBinWithStdin(
	t *testing.T, binary string, args []string,
	workDir, stdin string,
) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}
	return binResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// normalizePaths replaces absolute workDir paths with "WORK/" prefix.
func normalizePaths(b []byte, workDir string) []byte {
	return bytes.ReplaceAll(
		b, []byte(workDir+"/"), []byte("WORK/"))
}

// runRmDiffTest runs a single rm test against both binaries.
func runRmDiffTest(
	t *testing.T, goBin, refBin string, tc rmTestCase,
) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		refDir := t.TempDir()
		goDir := t.TempDir()
		refArgs := tc.setup(t, refDir)
		goArgs := tc.setup(t, goDir)
		refRes := runBin(t, refBin, refArgs, refDir)
		goRes := runBin(t, goBin, goArgs, goDir)
		compareRmResults(
			t, tc.name, refRes, goRes, refDir, goDir)
	})
}

// compareRmResults compares outputs after path normalization.
func compareRmResults(
	t *testing.T, name string,
	refRes, goRes binResult,
	refDir, goDir string,
) {
	t.Helper()
	refOut := stderrNormalizer(
		normalizePaths(refRes.stdout, refDir))
	goOut := stderrNormalizer(
		normalizePaths(goRes.stdout, goDir))
	refErr := stderrNormalizer(
		normalizePaths(refRes.stderr, refDir))
	goErr := stderrNormalizer(
		normalizePaths(goRes.stderr, goDir))
	if !bytes.Equal(refOut, goOut) ||
		!bytes.Equal(refErr, goErr) ||
		refRes.exitCode != goRes.exitCode {
		t.Errorf("divergence in %s\n"+
			"  ref stdout: %q\n  go  stdout: %q\n"+
			"  ref stderr: %q\n  go  stderr: %q\n"+
			"  ref exit: %d\n  go  exit: %d",
			name, refOut, goOut, refErr, goErr,
			refRes.exitCode, goRes.exitCode)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	tests := []rmTestCase{
		{
			// R1.1: remove a single regular file.
			name: "single_file",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f := writeFile(t, dir, "a.txt", []byte("hello\n"))
				return []string{f}
			},
		},
		{
			// R1.1: remove multiple files.
			name: "multi_file",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f1 := writeFile(t, dir, "x.txt", []byte("x\n"))
				f2 := writeFile(t, dir, "y.txt", []byte("y\n"))
				return []string{f1, f2}
			},
		},
		{
			// R1.2: error when removing directory without -r.
			name: "dir_without_r",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "subdir")
				return []string{d}
			},
		},
		{
			// R2.2: -f silently ignores nonexistent files.
			name: "force_nonexistent",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{"-f",
					filepath.Join(dir, "nosuch.txt")}
			},
		},
		{
			// R2.2: without -f, nonexistent file is an error.
			name: "nonexistent_error",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{
					filepath.Join(dir, "nosuch.txt")}
			},
		},
		{
			// R3.3: -v prints removed file names.
			name: "verbose_remove",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f := writeFile(t, dir, "v.txt", []byte("v\n"))
				return []string{"-v", f}
			},
		},
		{
			// R2.1: -r removes directories recursively.
			name: "recursive_dir",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "tree")
				writeFile(t, d, "inner.txt", []byte("data\n"))
				makeDir(t, d, "nested")
				writeFile(t, filepath.Join(d, "nested"),
					"deep.txt", []byte("deep\n"))
				return []string{"-r", d}
			},
		},
		{
			// R2.1 + R2.2: -rf combined flags.
			name: "rf_combined",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "rfdir")
				writeFile(t, d, "f.txt", []byte("rf\n"))
				return []string{"-rf", d}
			},
		},
		{
			// R2.4: -d removes empty directories.
			name: "d_empty_dir",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "emptydir")
				return []string{"-d", d}
			},
		},
		{
			// R2.4: -d on non-empty directory fails.
			name: "d_nonempty_dir",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "notempty")
				writeFile(t, d, "file.txt", []byte("data\n"))
				return []string{"-d", d}
			},
		},
		{
			// R3.3: -rv verbose recursive removal.
			name: "recursive_verbose",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "rvdir")
				writeFile(t, d, "a.txt", []byte("a\n"))
				return []string{"-rv", d}
			},
		},
		{
			// R2.1: -R uppercase is synonym for -r.
			name: "recursive_uppercase_R",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "capR")
				writeFile(t, d, "f.txt", []byte("cap\n"))
				return []string{"-R", d}
			},
		},
		{
			// R1.4: partial failure continues with remaining.
			name: "partial_failure",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f1 := writeFile(t, dir, "ok.txt", []byte("ok\n"))
				return []string{f1,
					filepath.Join(dir, "nope.txt")}
			},
		},
		{
			// No operands without -f.
			name: "missing_operand",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{}
			},
		},
		{
			// -f with no operands exits 0.
			name: "force_no_operands",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{"-f"}
			},
		},
	}

	for _, tc := range tests {
		runRmDiffTest(t, goBin, refBin, tc)
	}
}

// TestDiffVersionHelp tests --version and --help output.
func TestDiffVersionHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	versionNorm := func(b []byte) []byte {
		re := regexp.MustCompile(`(?s)^(rm) .*`)
		return re.ReplaceAll(b, []byte("$1 (version)\n"))
	}
	helpNorm := func(b []byte) []byte {
		if len(b) > 0 {
			return []byte("HELP_OUTPUT\n")
		}
		return b
	}

	tests := []testutils.DiffTest{
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{versionNorm},
		},
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{helpNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMissingOperandMessage verifies the missing-operand error format.
func TestMissingOperandMessage(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	res := runBin(t, goBin, []string{}, t.TempDir())
	if res.exitCode != 1 {
		t.Errorf("expected exit 1, got %d", res.exitCode)
	}
	stderr := string(res.stderr)
	if !strings.Contains(stderr, "missing operand") {
		t.Errorf(
			"expected 'missing operand' in stderr, got: %s", stderr)
	}
}

// TestDotDotDotRefusal verifies . and .. are refused.
// R1.3: prevent accidental directory tree destruction.
func TestDotDotDotRefusal(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	for _, path := range []string{".", "..", "/tmp/."} {
		res := runBin(t, goBin, []string{path}, t.TempDir())
		if res.exitCode != 1 {
			t.Errorf("rm %s: expected exit 1, got %d",
				path, res.exitCode)
		}
		stderr := string(res.stderr)
		if !strings.Contains(stderr, "refusing to remove") {
			t.Errorf("rm %s: expected refusing message, got: %s",
				path, stderr)
		}
	}
}

// TestInteractiveAlways tests -i prompting with y/n responses.
// R3.1: -i prompts before every removal.
func TestInteractiveAlways(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	t.Run("yes_removes", func(t *testing.T) {
		dir := t.TempDir()
		f := writeFile(t, dir, "target.txt", []byte("data\n"))
		res := runBinWithStdin(
			t, goBin, []string{"-i", f}, dir, "y\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if pathExists(f) {
			t.Error("file should be removed after y response")
		}
		stderr := string(res.stderr)
		if !strings.Contains(stderr, "remove") {
			t.Errorf("expected prompt in stderr, got: %q", stderr)
		}
	})

	t.Run("no_keeps", func(t *testing.T) {
		dir := t.TempDir()
		f := writeFile(t, dir, "keep.txt", []byte("data\n"))
		res := runBinWithStdin(
			t, goBin, []string{"-i", f}, dir, "n\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if !pathExists(f) {
			t.Error("file should still exist after n response")
		}
	})

	t.Run("force_overrides_i", func(t *testing.T) {
		// R2.2: last flag wins. -i then -f means force.
		dir := t.TempDir()
		f := writeFile(t, dir, "gone.txt", []byte("data\n"))
		res := runBin(
			t, goBin, []string{"-i", "-f", f}, dir)
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if pathExists(f) {
			t.Error("file should be removed with -if (force wins)")
		}
	})
}

// TestInteractiveOnce tests -I prompting conditions.
// R3.2: -I prompts once when >3 files or -r is active.
func TestInteractiveOnce(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	t.Run("three_or_fewer_no_prompt", func(t *testing.T) {
		// With 3 files (not > 3), -I should NOT prompt.
		dir := t.TempDir()
		f1 := writeFile(t, dir, "a.txt", []byte("a\n"))
		f2 := writeFile(t, dir, "b.txt", []byte("b\n"))
		f3 := writeFile(t, dir, "c.txt", []byte("c\n"))
		res := runBin(
			t, goBin, []string{"-I", f1, f2, f3}, dir)
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		for _, f := range []string{f1, f2, f3} {
			if pathExists(f) {
				t.Errorf("file %s should be removed", f)
			}
		}
	})

	t.Run("more_than_three_yes", func(t *testing.T) {
		// With 4 files, -I should prompt; answer y removes all.
		dir := t.TempDir()
		f1 := writeFile(t, dir, "a.txt", []byte("a\n"))
		f2 := writeFile(t, dir, "b.txt", []byte("b\n"))
		f3 := writeFile(t, dir, "c.txt", []byte("c\n"))
		f4 := writeFile(t, dir, "d.txt", []byte("d\n"))
		res := runBinWithStdin(
			t, goBin, []string{"-I", f1, f2, f3, f4},
			dir, "y\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		for _, f := range []string{f1, f2, f3, f4} {
			if pathExists(f) {
				t.Errorf("file %s should be removed", f)
			}
		}
	})

	t.Run("more_than_three_no", func(t *testing.T) {
		// With 4 files, -I with n keeps all files.
		dir := t.TempDir()
		f1 := writeFile(t, dir, "a.txt", []byte("a\n"))
		f2 := writeFile(t, dir, "b.txt", []byte("b\n"))
		f3 := writeFile(t, dir, "c.txt", []byte("c\n"))
		f4 := writeFile(t, dir, "d.txt", []byte("d\n"))
		res := runBinWithStdin(
			t, goBin, []string{"-I", f1, f2, f3, f4},
			dir, "n\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		for _, f := range []string{f1, f2, f3, f4} {
			if !pathExists(f) {
				t.Errorf("file %s should still exist", f)
			}
		}
	})

	t.Run("recursive_prompts", func(t *testing.T) {
		// -I with -r and 1 arg should prompt (recursive trigger).
		dir := t.TempDir()
		d := makeDir(t, dir, "rdir")
		writeFile(t, d, "f.txt", []byte("f\n"))
		res := runBinWithStdin(
			t, goBin, []string{"-rI", d}, dir, "y\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if pathExists(d) {
			t.Error("dir should be removed after y response")
		}
	})
}

// TestInteractiveWhen tests --interactive=WHEN flag.
// R3.4: WHEN controls prompting mode.
func TestInteractiveWhen(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	t.Run("never", func(t *testing.T) {
		// --interactive=never acts like -f.
		dir := t.TempDir()
		res := runBin(t, goBin,
			[]string{"--interactive=never",
				filepath.Join(dir, "nosuch")}, dir)
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
	})

	t.Run("always_yes", func(t *testing.T) {
		dir := t.TempDir()
		f := writeFile(t, dir, "a.txt", []byte("a\n"))
		res := runBinWithStdin(
			t, goBin, []string{"--interactive=always", f},
			dir, "y\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if pathExists(f) {
			t.Error("file should be removed")
		}
	})

	t.Run("always_no", func(t *testing.T) {
		dir := t.TempDir()
		f := writeFile(t, dir, "a.txt", []byte("a\n"))
		res := runBinWithStdin(
			t, goBin, []string{"--interactive=always", f},
			dir, "n\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if !pathExists(f) {
			t.Error("file should still exist")
		}
	})

	t.Run("invalid_when", func(t *testing.T) {
		dir := t.TempDir()
		res := runBin(t, goBin,
			[]string{"--interactive=bogus", "x"}, dir)
		if res.exitCode != 1 {
			t.Errorf("exit: got %d, want 1", res.exitCode)
		}
	})
}

// TestRootRefusal verifies that rm -r / is refused by default.
func TestRootRefusal(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	res := runBin(t, goBin, []string{"-r", "/"}, t.TempDir())
	if res.exitCode != 1 {
		t.Errorf("exit: got %d, want 1", res.exitCode)
	}
	stderr := string(res.stderr)
	if !strings.Contains(stderr, "dangerous") {
		t.Errorf(
			"expected 'dangerous' in stderr, got: %q", stderr)
	}
}

// TestRecursiveDescendPrompt verifies -i prompts for directory descent.
// R3.1: -i prompts "descend into directory?" before recursing.
func TestRecursiveDescendPrompt(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	t.Run("descend_yes_remove_yes", func(t *testing.T) {
		dir := t.TempDir()
		d := makeDir(t, dir, "mydir")
		writeFile(t, d, "f.txt", []byte("f\n"))
		// Prompts: descend? y, remove file? y, remove dir? y
		res := runBinWithStdin(
			t, goBin, []string{"-ri", d}, dir, "y\ny\ny\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if pathExists(d) {
			t.Error("dir should be removed")
		}
	})

	t.Run("descend_no_keeps_dir", func(t *testing.T) {
		dir := t.TempDir()
		d := makeDir(t, dir, "keepdir")
		writeFile(t, d, "f.txt", []byte("f\n"))
		// Prompt: descend? n → skip entire directory.
		res := runBinWithStdin(
			t, goBin, []string{"-ri", d}, dir, "n\n")
		if res.exitCode != 0 {
			t.Errorf("exit: got %d, want 0", res.exitCode)
		}
		if !pathExists(d) {
			t.Error("dir should still exist")
		}
	})
}
