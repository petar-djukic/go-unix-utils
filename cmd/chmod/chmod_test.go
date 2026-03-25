// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chmod comparing against gchmod (GNU coreutils).
// Covers prd089-chmod R4.1 (error handling), R4.2 (special permission bits),
// R4.3 (edge cases: no-op, mixed success/failure, recursive + verbose + special).
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// stderrNormalizer normalizes error messages between GNU gchmod and Go chmod.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?chmod|gchmod`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	// Go wraps syscall errors with "stat path:" or "lstat path:" prefixes.
	goErrWrap := regexp.MustCompile(`(: )(stat|lstat|chmod) [^:]+: `)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("chmod"))
		b = tryHelp.ReplaceAll(b, nil)
		b = goErrWrap.ReplaceAll(b, []byte("$1"))
		b = bytes.ToLower(b)
		return b
	}
}

// stdoutNormalizer normalizes verbose output paths between temp dirs.
func stdoutNormalizer(dir string) testutils.NormalizeFunc {
	re := regexp.MustCompile(regexp.QuoteMeta(dir))
	return func(b []byte) []byte {
		return re.ReplaceAll(b, []byte("/TMPDIR"))
	}
}

// runBin runs a binary with args in the given working directory.
func runBin(t *testing.T, binary string, args []string, dir string) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

// writeFile creates a file with given content and mode.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// mustMkdir creates a directory with the given mode.
func mustMkdir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Mkdir(path, mode); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// fileMode returns the permission bits of a file.
func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode()
}

// lookupBinaries builds the Go binary and finds gchmod.
func lookupBinaries(t *testing.T) (string, string) {
	t.Helper()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchmod")
	if err != nil {
		t.Skipf("reference binary gchmod not in PATH: %v", err)
	}
	return goBin, refBin
}

// --- R4.1: Error handling differential tests ---

// TestDiffErrorInvalidMode tests invalid mode strings against gchmod.
// R4.1: invalid mode specification produces exit 1.
func TestDiffErrorInvalidMode(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tmpFile := filepath.Join(t.TempDir(), "f.txt")
	writeFile(t, tmpFile, "data", 0o644)

	tests := []testutils.DiffTest{
		{
			Name:      "invalid_mode_string",
			Args:      []string{"xyz", tmpFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			Name:      "invalid_mode_empty_clause",
			Args:      []string{"", tmpFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			Name:      "invalid_octal_digit",
			Args:      []string{"899", tmpFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorNonexistentFile tests chmod on a nonexistent file.
// R4.1: nonexistent file produces exit 1.
func TestDiffErrorNonexistentFile(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{"644", "/no/such/file/xyz_chmod_test"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorMissingOperand tests chmod with no arguments.
// R4.1: missing operand produces exit 1.
func TestDiffErrorMissingOperand(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			Name:      "mode_only_no_file",
			Args:      []string{"755"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// --- R4.2: Special permission bits differential tests ---

// TestSpecialBitsOctal tests setuid/setgid/sticky via octal modes.
// R4.2: special bits in octal mode.
func TestSpecialBitsOctal(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	cases := []struct {
		name string
		mode string
		want os.FileMode
	}{
		{"setuid_octal", "4755", os.ModeSetuid | 0o755},
		{"setgid_octal", "2755", os.ModeSetgid | 0o755},
		{"sticky_octal", "1755", os.ModeSticky | 0o755},
		{"all_special_octal", "7777", os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0o777},
		{"setuid_setgid_octal", "6755", os.ModeSetuid | os.ModeSetgid | 0o755},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refDir := t.TempDir()
			goDir := t.TempDir()
			writeFile(t, filepath.Join(refDir, "f"), "x", 0o644)
			writeFile(t, filepath.Join(goDir, "f"), "x", 0o644)

			refRes := runBin(t, refBin, []string{tc.mode, "f"}, refDir)
			goRes := runBin(t, goBin, []string{tc.mode, "f"}, goDir)
			compareResults(t, tc.name, refRes, goRes)

			goMode := fileMode(t, filepath.Join(goDir, "f"))
			refMode := fileMode(t, filepath.Join(refDir, "f"))
			if goMode != refMode {
				t.Errorf("mode mismatch: ref=%v go=%v", refMode, goMode)
			}
		})
	}
}

// TestSpecialBitsSymbolic tests setuid/setgid/sticky via symbolic modes.
// R4.2: special bits in symbolic mode.
func TestSpecialBitsSymbolic(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	cases := []struct {
		name      string
		mode      string
		startMode string // octal mode to set before test
	}{
		{"setuid_symbolic", "u+s", "0755"},
		{"setgid_symbolic", "g+s", "0755"},
		{"sticky_symbolic", "+t", "0755"},
		{"setuid_plus_exec", "u+sx", "0644"},
		{"setgid_plus_exec", "g+sx", "0644"},
		{"combo_suid_sgid_sticky", "u+s,g+s,o+t", "0755"},
		{"assign_with_setuid", "u=rwxs", "0644"},
		{"assign_with_setgid", "g=rxs", "0644"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refDir := t.TempDir()
			goDir := t.TempDir()
			writeFile(t, filepath.Join(refDir, "f"), "x", 0o644)
			writeFile(t, filepath.Join(goDir, "f"), "x", 0o644)
			// Set starting mode using the reference binary.
			runBin(t, refBin, []string{tc.startMode, "f"}, refDir)
			runBin(t, refBin, []string{tc.startMode, "f"}, goDir)

			refRes := runBin(t, refBin, []string{tc.mode, "f"}, refDir)
			goRes := runBin(t, goBin, []string{tc.mode, "f"}, goDir)
			compareResults(t, tc.name, refRes, goRes)

			goMode := fileMode(t, filepath.Join(goDir, "f"))
			refMode := fileMode(t, filepath.Join(refDir, "f"))
			if goMode != refMode {
				t.Errorf("mode mismatch: ref=%v go=%v", refMode, goMode)
			}
		})
	}
}

// --- R4.3: Edge case differential tests ---

// TestNoOpModeChange tests applying a mode that matches the current mode.
// R4.3: no-op mode change still succeeds with exit 0.
func TestNoOpModeChange(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f"), "x", 0o644)
	writeFile(t, filepath.Join(goDir, "f"), "x", 0o644)

	refRes := runBin(t, refBin, []string{"644", "f"}, refDir)
	goRes := runBin(t, goBin, []string{"644", "f"}, goDir)
	compareResults(t, "noop_octal", refRes, goRes)

	if goRes.exitCode != 0 {
		t.Errorf("expected exit 0 for no-op, got %d", goRes.exitCode)
	}
}

// TestNoOpVerbose tests verbose output when mode doesn't change.
// R4.3: verbose no-op should print "retained" message.
func TestNoOpVerbose(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f"), "x", 0o644)
	writeFile(t, filepath.Join(goDir, "f"), "x", 0o644)

	refRes := runBin(t, refBin, []string{"-v", "644", "f"}, refDir)
	goRes := runBin(t, goBin, []string{"-v", "644", "f"}, goDir)

	// Normalize temp dir paths in stdout.
	refNorm := stdoutNormalizer(refDir)
	goNorm := stdoutNormalizer(goDir)
	refRes.stdout = refNorm(refRes.stdout)
	goRes.stdout = goNorm(goRes.stdout)
	compareResults(t, "noop_verbose", refRes, goRes)
}

// TestChangesOnlyNoOp tests -c with a no-op: should produce no output.
// R4.3: changes-only mode suppresses output when mode unchanged.
func TestChangesOnlyNoOp(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f"), "x", 0o644)
	writeFile(t, filepath.Join(goDir, "f"), "x", 0o644)

	refRes := runBin(t, refBin, []string{"-c", "644", "f"}, refDir)
	goRes := runBin(t, goBin, []string{"-c", "644", "f"}, goDir)
	compareResults(t, "changes_noop", refRes, goRes)

	if len(goRes.stdout) != 0 {
		t.Errorf("expected no stdout for -c no-op, got %q", goRes.stdout)
	}
}

// TestMultipleFilesMixedSuccess tests chmod on a mix of existing and
// nonexistent files. R4.3: continues processing, exits 1.
func TestMultipleFilesMixedSuccess(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "good.txt"), "ok", 0o644)
	writeFile(t, filepath.Join(goDir, "good.txt"), "ok", 0o644)

	// "bad.txt" does not exist — both binaries should error on it.
	refRes := runBin(t, refBin,
		[]string{"755", "good.txt", "bad.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{"755", "good.txt", "bad.txt"}, goDir)

	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "mixed_success_failure", refRes, goRes)

	// Verify the good file was still changed.
	goMode := fileMode(t, filepath.Join(goDir, "good.txt"))
	if goMode.Perm() != 0o755 {
		t.Errorf("good.txt mode = %v, want 0755", goMode)
	}
}

// TestRecursiveVerbose tests -Rv on a directory tree.
// R4.3: combined recursive + verbose.
func TestRecursiveVerbose(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	setupTree := func(base string) {
		mustMkdir(t, filepath.Join(base, "d"), 0o755)
		writeFile(t, filepath.Join(base, "d", "a.txt"), "a", 0o644)
		writeFile(t, filepath.Join(base, "d", "b.txt"), "b", 0o600)
		mustMkdir(t, filepath.Join(base, "d", "sub"), 0o755)
		writeFile(t, filepath.Join(base, "d", "sub", "c.txt"), "c", 0o644)
	}

	refDir := t.TempDir()
	goDir := t.TempDir()
	setupTree(refDir)
	setupTree(goDir)

	refRes := runBin(t, refBin, []string{"-Rv", "755", "d"}, refDir)
	goRes := runBin(t, goBin, []string{"-Rv", "755", "d"}, goDir)

	// Normalize paths and sort lines since traversal order may differ.
	refNorm := stdoutNormalizer(refDir)
	goNorm := stdoutNormalizer(goDir)
	refRes.stdout = sortLines(refNorm(refRes.stdout))
	goRes.stdout = sortLines(goNorm(goRes.stdout))
	compareResults(t, "recursive_verbose", refRes, goRes)
}

// TestRecursiveChangesOnly tests -Rc on a directory tree.
// R4.3: only files that change should produce output.
func TestRecursiveChangesOnly(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	setupTree := func(base string) {
		mustMkdir(t, filepath.Join(base, "d"), 0o755)
		writeFile(t, filepath.Join(base, "d", "change.txt"), "x", 0o600)
		writeFile(t, filepath.Join(base, "d", "keep.txt"), "x", 0o755)
	}

	refDir := t.TempDir()
	goDir := t.TempDir()
	setupTree(refDir)
	setupTree(goDir)

	refRes := runBin(t, refBin, []string{"-Rc", "755", "d"}, refDir)
	goRes := runBin(t, goBin, []string{"-Rc", "755", "d"}, goDir)

	refNorm := stdoutNormalizer(refDir)
	goNorm := stdoutNormalizer(goDir)
	refRes.stdout = sortLines(refNorm(refRes.stdout))
	goRes.stdout = sortLines(goNorm(goRes.stdout))
	compareResults(t, "recursive_changes_only", refRes, goRes)
}

// TestRecursiveSpecialBits tests -R with special bits.
// R4.3: combined recursive + special bits.
func TestRecursiveSpecialBits(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	setupTree := func(base string) {
		mustMkdir(t, filepath.Join(base, "d"), 0o755)
		writeFile(t, filepath.Join(base, "d", "f1"), "x", 0o644)
		writeFile(t, filepath.Join(base, "d", "f2"), "x", 0o644)
	}

	refDir := t.TempDir()
	goDir := t.TempDir()
	setupTree(refDir)
	setupTree(goDir)

	refRes := runBin(t, refBin, []string{"-R", "4755", "d"}, refDir)
	goRes := runBin(t, goBin, []string{"-R", "4755", "d"}, goDir)
	compareResults(t, "recursive_setuid", refRes, goRes)

	// Verify modes match.
	for _, name := range []string{"d/f1", "d/f2"} {
		goM := fileMode(t, filepath.Join(goDir, name))
		refM := fileMode(t, filepath.Join(refDir, name))
		if goM != refM {
			t.Errorf("%s mode mismatch: ref=%v go=%v", name, refM, goM)
		}
	}
}

// TestVerboseSpecialBits tests -v with special bits to verify diagnostic
// output format includes special bit markers.
// R4.3: verbose + special bits.
func TestVerboseSpecialBits(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f"), "x", 0o644)
	writeFile(t, filepath.Join(goDir, "f"), "x", 0o644)

	refRes := runBin(t, refBin, []string{"-v", "4755", "f"}, refDir)
	goRes := runBin(t, goBin, []string{"-v", "4755", "f"}, goDir)

	refNorm := stdoutNormalizer(refDir)
	goNorm := stdoutNormalizer(goDir)
	refRes.stdout = refNorm(refRes.stdout)
	goRes.stdout = goNorm(goRes.stdout)
	compareResults(t, "verbose_setuid", refRes, goRes)
}

// TestRecursiveVerboseSpecialBitsSymbolic tests -Rv with symbolic special bits.
// R4.3: combined recursive + verbose + special bits in symbolic mode.
func TestRecursiveVerboseSpecialBitsSymbolic(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	setupTree := func(base string) {
		mustMkdir(t, filepath.Join(base, "d"), 0o755)
		writeFile(t, filepath.Join(base, "d", "f1"), "x", 0o644)
	}

	refDir := t.TempDir()
	goDir := t.TempDir()
	setupTree(refDir)
	setupTree(goDir)

	refRes := runBin(t, refBin,
		[]string{"-Rv", "u+s,g+s,o+t", "d"}, refDir)
	goRes := runBin(t, goBin,
		[]string{"-Rv", "u+s,g+s,o+t", "d"}, goDir)

	refNorm := stdoutNormalizer(refDir)
	goNorm := stdoutNormalizer(goDir)
	refRes.stdout = sortLines(refNorm(refRes.stdout))
	goRes.stdout = sortLines(goNorm(goRes.stdout))
	compareResults(t, "recursive_verbose_special_symbolic", refRes, goRes)

	goM := fileMode(t, filepath.Join(goDir, "d", "f1"))
	refM := fileMode(t, filepath.Join(refDir, "d", "f1"))
	if goM != refM {
		t.Errorf("d/f1 mode mismatch: ref=%v go=%v", refM, goM)
	}
}

// TestSilentSuppressesErrors tests -f suppresses error messages.
// R4.3: silent mode edge case.
func TestSilentSuppressesErrors(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	refRes := runBin(t, refBin,
		[]string{"-f", "644", "/no/such/file/chmod_test"}, t.TempDir())
	goRes := runBin(t, goBin,
		[]string{"-f", "644", "/no/such/file/chmod_test"}, t.TempDir())

	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "silent_suppresses", refRes, goRes)
}

// TestReferenceMode tests --reference=RFILE copies mode from reference file.
// R4.3: edge case with reference mode.
func TestReferenceMode(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "ref"), "x", 0o755)
	writeFile(t, filepath.Join(refDir, "target"), "x", 0o644)
	writeFile(t, filepath.Join(goDir, "ref"), "x", 0o755)
	writeFile(t, filepath.Join(goDir, "target"), "x", 0o644)

	refRes := runBin(t, refBin,
		[]string{fmt.Sprintf("--reference=%s", "ref"), "target"}, refDir)
	goRes := runBin(t, goBin,
		[]string{fmt.Sprintf("--reference=%s", "ref"), "target"}, goDir)
	compareResults(t, "reference_mode", refRes, goRes)

	goM := fileMode(t, filepath.Join(goDir, "target"))
	refM := fileMode(t, filepath.Join(refDir, "target"))
	if goM != refM {
		t.Errorf("target mode mismatch: ref=%v go=%v", refM, goM)
	}
}

// TestMultipleFilesAllSuccess tests chmod on multiple files, all succeeding.
// R4.3: exit 0 when all files processed successfully.
func TestMultipleFilesAllSuccess(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(refDir, name), "x", 0o644)
		writeFile(t, filepath.Join(goDir, name), "x", 0o644)
	}

	refRes := runBin(t, refBin,
		[]string{"755", "a.txt", "b.txt", "c.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{"755", "a.txt", "b.txt", "c.txt"}, goDir)
	compareResults(t, "multi_all_success", refRes, goRes)

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		goM := fileMode(t, filepath.Join(goDir, name))
		if goM.Perm() != 0o755 {
			t.Errorf("%s mode = %v, want 0755", name, goM)
		}
	}
}

// sortLines sorts the lines of a byte slice for order-independent comparison.
func sortLines(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	lines := bytes.Split(b, []byte("\n"))
	// Simple insertion sort — line count is small.
	for i := 1; i < len(lines); i++ {
		key := lines[i]
		j := i - 1
		for j >= 0 && bytes.Compare(lines[j], key) > 0 {
			lines[j+1] = lines[j]
			j--
		}
		lines[j+1] = key
	}
	return bytes.Join(lines, []byte("\n"))
}
