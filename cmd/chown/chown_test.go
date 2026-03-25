// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chown comparing against gchown (GNU coreutils).
// Covers prd091-chown R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
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

// stderrNormalizer normalizes error messages between GNU gchown and Go chown.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?chown|gchown`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	goErrWrap := regexp.MustCompile(
		`(: )(stat|lstat|chown|lchown) [^:]+: `)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("chown"))
		b = tryHelp.ReplaceAll(b, nil)
		b = goErrWrap.ReplaceAll(b, []byte("$1"))
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
		context.Background(), 10*time.Second)
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
	if err := os.WriteFile(
		path, []byte(content), mode); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// lookupBinaries builds the Go binary and finds gchown.
func lookupBinaries(t *testing.T) (string, string) {
	t.Helper()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchown")
	if err != nil {
		t.Skipf("reference binary gchown not in PATH: %v", err)
	}
	return goBin, refBin
}

// currentUserInfo returns the current user's username, UID, GID, and group.
func currentUserInfo(t *testing.T) (string, string, string, string) {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("cannot get current user: %v", err)
	}
	g, err := user.LookupGroupId(u.Gid)
	if err != nil {
		t.Skipf("cannot resolve current GID %s: %v", u.Gid, err)
	}
	return u.Username, u.Uid, u.Gid, g.Name
}

// makeTree creates a directory tree for recursive tests.
// Returns the tree root path inside the given base directory.
func makeTree(t *testing.T, base string) string {
	t.Helper()
	dir := filepath.Join(base, "d")
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("makeTree MkdirAll: %v", err)
	}
	writeFile(t, filepath.Join(dir, "a.txt"), "a", 0o644)
	writeFile(t, filepath.Join(sub, "b.txt"), "b", 0o644)
	return dir
}

// --- R1.1: OWNER[:GROUP] syntax differential tests ---

// TestDiffOwnerOnly tests OWNER form (no colon, user only).
// R1.1: OWNER changes only the owner.
func TestDiffOwnerOnly(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, _ := currentUserInfo(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{username, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{username, "f.txt"}, goDir)
	compareResults(t, "owner_only", refRes, goRes)
}

// TestDiffOwnerGroup tests OWNER:GROUP form.
// R1.1: OWNER:GROUP changes both owner and group.
func TestDiffOwnerGroup(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{spec, "f.txt"}, goDir)
	compareResults(t, "owner_group", refRes, goRes)
}

// TestDiffOwnerColon tests OWNER: form (login group).
// R1.1: OWNER: changes owner and sets group to login group.
func TestDiffOwnerColon(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, _ := currentUserInfo(t)
	spec := username + ":"

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{spec, "f.txt"}, goDir)
	compareResults(t, "owner_colon", refRes, goRes)
}

// TestDiffGroupOnly tests :GROUP form.
// R1.1: :GROUP changes only the group.
func TestDiffGroupOnly(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, _, _, groupname := currentUserInfo(t)
	spec := ":" + groupname

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{spec, "f.txt"}, goDir)
	compareResults(t, "group_only", refRes, goRes)
}

// --- R1.2: Numeric UID/GID differential tests ---

// TestDiffNumericUID tests OWNER by numeric UID.
// R1.2: OWNER may be a numeric ID.
func TestDiffNumericUID(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, uid, _, _ := currentUserInfo(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{uid, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{uid, "f.txt"}, goDir)
	compareResults(t, "numeric_uid", refRes, goRes)
}

// TestDiffNumericUIDGID tests numeric UID:GID form.
// R1.2: both OWNER and GROUP may be numeric IDs.
func TestDiffNumericUIDGID(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, uid, gid, _ := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", uid, gid)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{spec, "f.txt"}, goDir)
	compareResults(t, "numeric_uid_gid", refRes, goRes)
}

// TestDiffNumericGroupOnly tests :GID form.
// R1.2: GROUP may be a numeric ID.
func TestDiffNumericGroupOnly(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, _, gid, _ := currentUserInfo(t)
	spec := ":" + gid

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{spec, "f.txt"}, goDir)
	compareResults(t, "numeric_gid_only", refRes, goRes)
}

// --- R1.3: --reference differential tests ---

// TestDiffReference tests --reference=RFILE.
// R1.3: sets each FILE's owner and group to match RFILE's.
func TestDiffReference(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "ref.txt"), "ref", 0o644)
	writeFile(t, filepath.Join(refDir, "target.txt"), "tgt", 0o644)
	writeFile(t, filepath.Join(goDir, "ref.txt"), "ref", 0o644)
	writeFile(t, filepath.Join(goDir, "target.txt"), "tgt", 0o644)

	refRes := runBin(t, refBin,
		[]string{"--reference=ref.txt", "target.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{"--reference=ref.txt", "target.txt"}, goDir)
	compareResults(t, "reference", refRes, goRes)
}

// TestDiffReferenceNonexistent tests --reference with missing file.
// R1.3/R1.4: error when reference file does not exist.
func TestDiffReferenceNonexistent(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tmpFile := filepath.Join(t.TempDir(), "f.txt")
	writeFile(t, tmpFile, "data", 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "reference_nonexistent",
			Args: []string{
				"--reference=/no_such_ref_file_xyz", tmpFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// --- R1.4: Error handling differential tests ---

// TestDiffErrorMissingOperand tests error on no arguments.
// R1.4: exit 1 with error to stderr.
func TestDiffErrorMissingOperand(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			Name:      "owner_no_file",
			Args:      []string{"root"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorInvalidUser tests error on invalid user name.
// R1.4: invalid user produces exit 1.
func TestDiffErrorInvalidUser(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tmpFile := filepath.Join(t.TempDir(), "f.txt")
	writeFile(t, tmpFile, "data", 0o644)

	tests := []testutils.DiffTest{
		{
			Name:      "invalid_user",
			Args:      []string{"nonexistentuser99999", tmpFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorInvalidGroup tests error on invalid group name.
// R1.4: invalid group produces exit 1.
func TestDiffErrorInvalidGroup(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tmpFile := filepath.Join(t.TempDir(), "f.txt")
	writeFile(t, tmpFile, "data", 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "invalid_group_colon",
			Args: []string{
				":nonexistentgroup99999", tmpFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorNonexistentFile tests error on nonexistent file.
// R1.4: nonexistent file produces exit 1.
func TestDiffErrorNonexistentFile(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, _, _, groupname := currentUserInfo(t)
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		{
			Name: "nonexistent_file",
			Args: []string{
				fmt.Sprintf(":%s", groupname),
				"/no/such/file/chown_test"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorMixed tests exit 1 when some files succeed and some fail.
// R1.4: any error → exit 1, continue processing remaining files.
func TestDiffErrorMixed(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, _, _, groupname := currentUserInfo(t)
	errNorm := stderrNormalizer()
	spec := ":" + groupname

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "good.txt"), "ok", 0o644)
	writeFile(t, filepath.Join(goDir, "good.txt"), "ok", 0o644)

	refRes := runBin(t, refBin,
		[]string{spec, "good.txt", "bad.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{spec, "good.txt", "bad.txt"}, goDir)

	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "mixed_success_failure", refRes, goRes)

	if goRes.exitCode != 1 {
		t.Errorf("expected exit 1 for mixed, got %d", goRes.exitCode)
	}
}

// TestDiffMultipleFiles tests successful ownership change on multiple files.
// R1.4: all files succeed → exit 0.
func TestDiffMultipleFiles(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(refDir, name), "x", 0o644)
		writeFile(t, filepath.Join(goDir, name), "x", 0o644)
	}

	refRes := runBin(t, refBin,
		[]string{spec, "a.txt", "b.txt", "c.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{spec, "a.txt", "b.txt", "c.txt"}, goDir)
	compareResults(t, "multiple_files", refRes, goRes)
}

// TestDiffSilentError tests -f suppresses error output.
// R1.4: silent mode still exits 1 on error.
func TestDiffSilentError(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, _, _, groupname := currentUserInfo(t)
	errNorm := stderrNormalizer()
	spec := ":" + groupname

	refRes := runBin(t, refBin,
		[]string{"-f", spec, "/no/such/file/chown_test"},
		t.TempDir())
	goRes := runBin(t, goBin,
		[]string{"-f", spec, "/no/such/file/chown_test"},
		t.TempDir())

	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "silent_error", refRes, goRes)
}

// TestDiffNumericUIDColon tests UID: form (numeric user with login group).
// GNU chown rejects numeric UID in OWNER: form as "invalid spec" because
// login group lookup requires a symbolic username. Match GNU behavior.
func TestDiffNumericUIDColon(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, uid, _, _ := currentUserInfo(t)
	errNorm := stderrNormalizer()
	spec := uid + ":"

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{spec, "f.txt"}, goDir)
	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "numeric_uid_colon", refRes, goRes)
}

// --- R1.3: --reference with absolute path ---

// TestDiffReferenceAbsPath tests --reference with an absolute path.
// R1.3: reference file can be specified with absolute path.
func TestDiffReferenceAbsPath(t *testing.T) {
	goBin, refBin := lookupBinaries(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	refFile := filepath.Join(refDir, "ref.txt")
	writeFile(t, refFile, "ref", 0o644)
	writeFile(t, filepath.Join(refDir, "target.txt"), "tgt", 0o644)
	goRefFile := filepath.Join(goDir, "ref.txt")
	writeFile(t, goRefFile, "ref", 0o644)
	writeFile(t, filepath.Join(goDir, "target.txt"), "tgt", 0o644)

	refRes := runBin(t, refBin,
		[]string{
			fmt.Sprintf("--reference=%s", refFile),
			"target.txt",
		}, refDir)
	goRes := runBin(t, goBin,
		[]string{
			fmt.Sprintf("--reference=%s", goRefFile),
			"target.txt",
		}, goDir)
	compareResults(t, "reference_abs_path", refRes, goRes)
}

// TestDiffOwnerByUID tests using UID for an existing system user.
// R1.2: verify UID 0 is resolved (owner change may fail without root,
// which is fine since both binaries should fail identically).
func TestDiffOwnerByUIDZero(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	// Changing to UID 0 requires root; both should fail identically.
	refRes := runBin(t, refBin, []string{"0", "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{"0", "f.txt"}, goDir)
	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "owner_uid_zero", refRes, goRes)
}

// TestDiffOwnerGroupRoot tests chown root:staff fails identically.
// R1.4: both binaries should produce matching error.
func TestDiffOwnerGroupRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin,
		[]string{"root:staff", "f.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{"root:staff", "f.txt"}, goDir)
	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "owner_group_root", refRes, goRes)

	// Verify: both ref and go fail with same exit code.
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("exit mismatch: ref=%d go=%d",
			refRes.exitCode, goRes.exitCode)
	}
}

// lookupSecondGroup finds a group different from the current GID.
func lookupSecondGroup(t *testing.T) string {
	t.Helper()
	currentGid := os.Getgid()
	// Try "everyone" (macOS) or common groups.
	candidates := []string{"everyone", "wheel", "daemon"}
	for _, name := range candidates {
		g, err := user.LookupGroup(name)
		if err != nil {
			continue
		}
		gid, _ := strconv.Atoi(g.Gid)
		if gid != currentGid {
			return name
		}
	}
	t.Skip("no second group found for testing")
	return ""
}

// TestDiffGroupOnlyDifferent tests :GROUP with a different group.
// R1.1: :GROUP changes only the group to a group the user belongs to.
func TestDiffGroupOnlyDifferent(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	secondGroup := lookupSecondGroup(t)
	spec := ":" + secondGroup

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{spec, "f.txt"}, goDir)
	compareResults(t, "group_only_different", refRes, goRes)
}

// --- R2.1: Recursive ownership change differential tests ---

// TestDiffRecursiveBasic tests -R on a directory tree.
// R2.1: -R changes ownership recursively for directories and their contents.
func TestDiffRecursiveBasic(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	makeTree(t, refDir)
	makeTree(t, goDir)

	refRes := runBin(t, refBin, []string{"-R", spec, "d"}, refDir)
	goRes := runBin(t, goBin, []string{"-R", spec, "d"}, goDir)
	compareResults(t, "recursive_basic", refRes, goRes)
}

// TestDiffRecursiveGroupOnly tests -R with :GROUP form.
// R2.1: recursive with group-only change.
func TestDiffRecursiveGroupOnly(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	_, _, _, groupname := currentUserInfo(t)
	spec := ":" + groupname

	refDir := t.TempDir()
	goDir := t.TempDir()
	makeTree(t, refDir)
	makeTree(t, goDir)

	refRes := runBin(t, refBin, []string{"-R", spec, "d"}, refDir)
	goRes := runBin(t, goBin, []string{"-R", spec, "d"}, goDir)
	compareResults(t, "recursive_group_only", refRes, goRes)
}

// TestDiffRecursiveLongFlag tests --recursive long flag.
// R2.1: --recursive is equivalent to -R.
func TestDiffRecursiveLongFlag(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	makeTree(t, refDir)
	makeTree(t, goDir)

	refRes := runBin(t, refBin, []string{"--recursive", spec, "d"}, refDir)
	goRes := runBin(t, goBin, []string{"--recursive", spec, "d"}, goDir)
	compareResults(t, "recursive_long_flag", refRes, goRes)
}

// TestDiffRecursiveNonexistent tests -R on a nonexistent directory.
// R2.1/R1.4: error when the target does not exist.
func TestDiffRecursiveNonexistent(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, _ := currentUserInfo(t)
	errNorm := stderrNormalizer()

	dir := t.TempDir()
	refRes := runBin(t, refBin,
		[]string{"-R", username, "no_such_dir"}, dir)
	goRes := runBin(t, goBin,
		[]string{"-R", username, "no_such_dir"}, dir)
	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "recursive_nonexistent", refRes, goRes)
}

// --- R2.2: Verbose and changes-only output differential tests ---

// TestDiffVerboseRetained tests -v output when ownership is retained.
// R3.1: -v prints diagnostic for every file, including retained.
func TestDiffVerboseRetained(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{"-v", spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{"-v", spec, "f.txt"}, goDir)
	compareResults(t, "verbose_retained", refRes, goRes)
}

// TestDiffChangesNoChange tests -c output when no change is made.
// R3.1: -c suppresses output when no change occurs.
func TestDiffChangesNoChange(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{"-c", spec, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{"-c", spec, "f.txt"}, goDir)
	compareResults(t, "changes_no_change", refRes, goRes)
}

// TestDiffVerboseRecursive tests -Rv on a directory tree.
// R3.1/R2.1: verbose with recursive.
func TestDiffVerboseRecursive(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	makeTree(t, refDir)
	makeTree(t, goDir)

	refRes := runBin(t, refBin, []string{"-Rv", spec, "d"}, refDir)
	goRes := runBin(t, goBin, []string{"-Rv", spec, "d"}, goDir)
	compareResults(t, "verbose_recursive", refRes, goRes)
}

// --- R2.3: Dereference control differential tests ---

// TestDiffNoDereference tests -h flag on a symlink.
// R2.2: -h changes the symlink itself.
func TestDiffNoDereference(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "target.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "target.txt"), "data", 0o644)
	os.Symlink("target.txt", filepath.Join(refDir, "link.txt"))
	os.Symlink("target.txt", filepath.Join(goDir, "link.txt"))

	refRes := runBin(t, refBin,
		[]string{"-h", spec, "link.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{"-h", spec, "link.txt"}, goDir)
	compareResults(t, "no_dereference", refRes, goRes)
}

// TestDiffRecursiveP tests -R -P (default: don't follow symlinks).
// R2.3: -P never follows symlinks during recursion.
func TestDiffRecursiveP(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	username, _, _, groupname := currentUserInfo(t)
	spec := fmt.Sprintf("%s:%s", username, groupname)

	refDir := t.TempDir()
	goDir := t.TempDir()
	// Create tree with symlink inside.
	for _, base := range []string{refDir, goDir} {
		d := filepath.Join(base, "d")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(d, "a.txt"), "a", 0o644)
		os.Symlink("a.txt", filepath.Join(d, "link.txt"))
	}

	refRes := runBin(t, refBin,
		[]string{"-R", "-P", spec, "d"}, refDir)
	goRes := runBin(t, goBin,
		[]string{"-R", "-P", spec, "d"}, goDir)
	compareResults(t, "recursive_P", refRes, goRes)
}

// --- R3.1: --version and --help differential tests ---

// TestDiffVersion tests --version output.
// R3.1: --version outputs version information.
func TestDiffVersion(t *testing.T) {
	goBin, _ := lookupBinaries(t)

	res := runBin(t, goBin, []string{"--version"}, t.TempDir())
	if res.exitCode != 0 {
		t.Errorf("--version exit code: got %d, want 0", res.exitCode)
	}
	if len(res.stdout) == 0 {
		t.Error("--version produced no output")
	}
}

// TestDiffHelp tests --help output.
// R3.1: --help displays usage information.
func TestDiffHelp(t *testing.T) {
	goBin, _ := lookupBinaries(t)

	res := runBin(t, goBin, []string{"--help"}, t.TempDir())
	if res.exitCode != 0 {
		t.Errorf("--help exit code: got %d, want 0", res.exitCode)
	}
	if len(res.stdout) == 0 {
		t.Error("--help produced no output")
	}
	if !bytes.Contains(res.stdout, []byte("Usage:")) {
		t.Error("--help output missing 'Usage:'")
	}
	if !bytes.Contains(res.stdout, []byte("--recursive")) {
		t.Error("--help output missing '--recursive'")
	}
}
