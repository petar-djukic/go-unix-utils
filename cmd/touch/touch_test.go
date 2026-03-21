// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/touch against gtouch (GNU coreutils).
// Implements prd062-touch R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.4 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the binary name/path prefix in error
// messages so that "gtouch:" and "/path/to/touch:" both become "touch:".
func binaryNameNormalizer(b []byte) []byte {
	re := regexp.MustCompile(`(?m)^(?:\S+/)?g?touch:`)
	b = re.ReplaceAll(b, []byte("touch:"))
	reTry := regexp.MustCompile(`Try '[^']*' for more information\.`)
	b = reTry.ReplaceAll(b, []byte("Try 'touch --help' for more information."))
	return b
}

// caseNormalizer lowercases output for comparing error messages
// that may differ in capitalization between GNU and Go (e.g.,
// "No such file or directory" vs "no such file or directory").
func caseNormalizer(b []byte) []byte {
	return bytes.ToLower(b)
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skipf("reference binary gtouch not in PATH: %v", err)
	}

	norm := []testutils.NormalizeFunc{binaryNameNormalizer}

	tests := []testutils.DiffTest{
		// R1.2: create new file when it does not exist.
		{Name: "create_new_file", Args: []string{"newfile"}},
		// R1.1: update timestamps on existing file.
		{Name: "update_existing_file", Args: []string{"existingfile"}},
		// R1.4: multiple file arguments.
		{Name: "multiple_files", Args: []string{"file1", "file2", "file3"}},
		// R1.3: -c flag suppresses creation of nonexistent file.
		{Name: "no_create_short_flag", Args: []string{"-c", "nonexistent"}},
		// R1.3: --no-create long form.
		{Name: "no_create_long_flag", Args: []string{"--no-create", "nonexistent"}},
		// R1.1, R1.3: -c on existing file still updates timestamps.
		{Name: "no_create_existing_file", Args: []string{"-c", "existingfile"}},
		// Missing file operand.
		{Name: "missing_operand", Args: []string{}, ExitCode: 1, Normalize: norm},
		// R2.1: -a changes only access time.
		{Name: "access_only_flag", Args: []string{"-a", "afile"}},
		// R2.2: -m changes only modification time.
		{Name: "mod_only_flag", Args: []string{"-m", "mfile"}},
		// R2.3: -a -m together changes both.
		{Name: "access_and_mod_flags", Args: []string{"-a", "-m", "amfile"}},
		// R2.1, R2.2: combined -am flags.
		{Name: "combined_am_flags", Args: []string{"-am", "amfile2"}},
		// R2.4: -t with CCYYMMDDhhmm.ss format.
		{Name: "explicit_timestamp_full", Args: []string{"-t", "202401151030.30", "tfile"}},
		// R2.4: -t with CCYYMMDDhhmm (no seconds).
		{Name: "explicit_timestamp_no_sec", Args: []string{"-t", "202401151030", "tfile2"}},
		// R2.4: -t with YYMMDDhhmm format.
		{Name: "explicit_timestamp_yy", Args: []string{"-t", "2401151030", "tfile3"}},
		// R2.4: -t with MMDDhhmm format.
		{Name: "explicit_timestamp_mmdd", Args: []string{"-t", "01151030", "tfile4"}},
		// R2.4: invalid -t format.
		{
			Name: "invalid_timestamp_format", Args: []string{"-t", "invalid", "tfile5"},
			ExitCode: 1, Normalize: norm,
		},
		// R2.4: -t missing argument.
		{Name: "t_flag_missing_arg", Args: []string{"-t"}, ExitCode: 1, Normalize: norm},
		// R2.1 + R2.4: -a with explicit timestamp.
		{Name: "a_with_explicit_timestamp", Args: []string{"-a", "-t", "202401151030.00", "atfile"}},
		// R2.2 + R2.4: -m with explicit timestamp.
		{Name: "m_with_explicit_timestamp", Args: []string{"-m", "-t", "202401151030.00", "mtfile"}},
		// R2.4: -t combined with value (no space).
		{Name: "t_combined_value", Args: []string{"-t202401151030.00", "tcfile"}},
		// R2.1, R2.4: combined -at with value.
		{Name: "combined_at_value", Args: []string{"-at", "202401151030.00", "catfile"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffR3 runs differential tests for R3 requirements.
func TestDiffR3(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skipf("reference binary gtouch not in PATH: %v", err)
	}

	norm := []testutils.NormalizeFunc{binaryNameNormalizer}
	normCase := []testutils.NormalizeFunc{binaryNameNormalizer, caseNormalizer}

	// Set up reference file for -r tests.
	refDir := t.TempDir()
	refFile := filepath.Join(refDir, "reffile")
	if err := os.WriteFile(refFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	refTime := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(refFile, refTime, refTime); err != nil {
		t.Fatal(err)
	}

	// Pre-create file for -h tests on existing files.
	hDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hDir, "existfile"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R3.1: -r copies timestamps from reference file.
		{Name: "r3_reference_short", Args: []string{"-r", refFile, "target1"}},
		// R3.1: --reference=FILE long form.
		{Name: "r3_reference_long", Args: []string{"--reference=" + refFile, "target2"}},
		// R3.1: -r with -a (only change access time).
		{Name: "r3_reference_access_only", Args: []string{"-a", "-r", refFile, "target3"}},
		// R3.1: -r with -m (only change mod time).
		{Name: "r3_reference_mod_only", Args: []string{"-m", "-r", refFile, "target4"}},
		// R3.2: -d with ISO date string.
		{Name: "r3_date_iso", Args: []string{"-d", "2024-01-15 10:30:30", "dfile1"}},
		// R3.2: -d with epoch format.
		{Name: "r3_date_epoch", Args: []string{"-d", "@1705312230", "dfile2"}},
		// R3.2: --date=STRING long form.
		{Name: "r3_date_long", Args: []string{"--date=2024-01-15 10:30:30", "dfile3"}},
		// R3.2: -d with date only.
		{Name: "r3_date_only", Args: []string{"-d", "2024-01-15", "dfile4"}},
		// R3.3: missing reference file exits 1 with error.
		{Name: "r3_missing_ref", Args: []string{"-r", "/nonexistent/ref", "target"}, ExitCode: 1, Normalize: normCase},
		// R3.2: invalid date string exits 1 with error.
		{Name: "r3_invalid_date", Args: []string{"-d", "not-a-date", "dfile5"}, ExitCode: 1, Normalize: norm},
		// R3.4: -h on nonexistent file errors (utimensat cannot create).
		{Name: "r3_no_deref_nonexistent", Args: []string{"-h", "hfile"}, ExitCode: 1, Normalize: normCase},
		// R3.4: --no-dereference long form on nonexistent file.
		{Name: "r3_no_deref_long_nonexist", Args: []string{"--no-dereference", "hfile2"}, ExitCode: 1, Normalize: normCase},
		// R3.4: -h combined with -c on nonexistent file (silent skip).
		{Name: "r3_no_deref_no_create", Args: []string{"-hc", "nonexistent_h"}},
		// R3.4: -h on existing regular file works normally.
		{Name: "r3_no_deref_existing", Args: []string{"-h", "existfile"}, WorkDir: hDir},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffR4 runs differential tests for R4 requirements (exit codes and edge cases).
func TestDiffR4(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skipf("reference binary gtouch not in PATH: %v", err)
	}

	norm := []testutils.NormalizeFunc{binaryNameNormalizer}
	normCase := []testutils.NormalizeFunc{binaryNameNormalizer, caseNormalizer}

	// R4.2: set up a directory with no write permission.
	noWriteDir := t.TempDir()
	noWriteChild := filepath.Join(noWriteDir, "noperm")
	if err := os.Mkdir(noWriteChild, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(noWriteChild, 0o755) // best-effort restore for cleanup
	})

	tests := []testutils.DiffTest{
		// R4.1: exit 0 on single successful file creation.
		{Name: "r4_exit_zero_create", Args: []string{"okfile"}},
		// R4.1: exit 0 on multiple successful files.
		{Name: "r4_exit_zero_multi", Args: []string{"ok1", "ok2", "ok3"}},
		// R4.2: exit 1 on invalid -t timestamp.
		{
			Name: "r4_exit_one_invalid_t", Args: []string{"-t", "notadate", "file"},
			ExitCode: 1, Normalize: norm,
		},
		// R4.2: exit 1 on invalid -d date string.
		{
			Name: "r4_exit_one_invalid_d", Args: []string{"-d", "garbage", "file"},
			ExitCode: 1, Normalize: norm,
		},
		// R4.2: exit 1 on permission denied, continue processing remaining files.
		{
			Name:     "r4_continue_after_error",
			Args:     []string{filepath.Join(noWriteChild, "blocked"), "goodfile"},
			ExitCode: 1, Normalize: normCase,
		},
		// R4.2: exit 1 on missing reference file.
		{
			Name: "r4_exit_one_missing_ref", Args: []string{"-r", "/no/such/ref", "file"},
			ExitCode: 1, Normalize: normCase,
		},
		// R4.4: -d with ISO 8601 T separator.
		{Name: "r4_date_iso_t", Args: []string{"-d", "2024-01-15T10:30:30", "dtfile"}},
		// R4.4: -d with epoch and fractional seconds.
		{Name: "r4_date_epoch_frac", Args: []string{"-d", "@1705312230.500000000", "effile"}},
		// R4.4: -d with date only (no time).
		{Name: "r4_date_only", Args: []string{"-d", "2024-06-01", "dofile"}},
		// R4.4: unrecognized option exits 1.
		{
			Name: "r4_unrecognized_option", Args: []string{"--bogus", "file"},
			ExitCode: 1, Normalize: norm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCreateNewFile verifies R1.2: touch creates a new empty file.
func TestCreateNewFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "newfile")

	cmd := exec.Command(goBin, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file, got size %d", info.Size())
	}
}

// TestUpdateTimestamps verifies R1.1: touch updates timestamps.
func TestUpdateTimestamps(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "existing")

	// Create file with old timestamp.
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(target, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Add(-time.Second)
	cmd := exec.Command(goBin, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch failed: %v\noutput: %s", err, out)
	}
	after := time.Now().Add(time.Second)

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Before(before) || info.ModTime().After(after) {
		t.Fatalf("mod time not updated: got %v, want between %v and %v",
			info.ModTime(), before, after)
	}
}

// TestNoCreateFlag verifies R1.3: -c suppresses file creation.
func TestNoCreateFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "nonexistent")

	cmd := exec.Command(goBin, "-c", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch -c failed: %v\noutput: %s", err, out)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file should not have been created")
	}
}

// TestMultipleFiles verifies R1.4: multiple file arguments.
func TestMultipleFiles(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "a"),
		filepath.Join(dir, "b"),
		filepath.Join(dir, "c"),
	}

	cmd := exec.Command(goBin, files...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch failed: %v\noutput: %s", err, out)
	}

	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("file %s not created: %v", f, err)
		}
	}
}

// TestAccessOnlyPreservesModTime verifies R2.1: -a changes only access time.
func TestAccessOnlyPreservesModTime(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "testfile")

	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(target, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, "-a", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -a failed: %v\n%s", err, out)
	}

	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	// R2.1: mod time must be preserved.
	if fi.ModTime.Year() != 2020 {
		t.Fatalf("mod time changed: got %v, want year 2020", fi.ModTime)
	}
	// Access time should have been updated.
	if fi.AccessTime.Year() == 2020 {
		t.Fatalf("access time not updated: still %v", fi.AccessTime)
	}
}

// TestModOnlyPreservesAccessTime verifies R2.2: -m changes only mod time.
func TestModOnlyPreservesAccessTime(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "testfile")

	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(target, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, "-m", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -m failed: %v\n%s", err, out)
	}

	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	// R2.2: access time must be preserved.
	if fi.AccessTime.Year() != 2020 {
		t.Fatalf("access time changed: got %v, want year 2020", fi.AccessTime)
	}
	// Mod time should have been updated.
	if fi.ModTime.Year() == 2020 {
		t.Fatalf("mod time not updated: still %v", fi.ModTime)
	}
}

// TestExplicitTimestamp verifies R2.4: -t sets both timestamps.
func TestExplicitTimestamp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "testfile")

	cmd := exec.Command(goBin, "-t", "202401151030.30", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -t failed: %v\n%s", err, out)
	}

	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2024, 1, 15, 10, 30, 30, 0, time.Local)
	// R2.4: both times set to the explicit timestamp.
	if !fi.ModTime.Equal(expected) {
		t.Fatalf("mod time: got %v, want %v", fi.ModTime, expected)
	}
	if !fi.AccessTime.Equal(expected) {
		t.Fatalf("access time: got %v, want %v", fi.AccessTime, expected)
	}
}

// TestExplicitTimestampAccessOnly verifies R2.1+R2.4: -a -t sets only access time.
func TestExplicitTimestampAccessOnly(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "testfile")

	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(target, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, "-a", "-t", "202401151030.30", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -a -t failed: %v\n%s", err, out)
	}

	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2024, 1, 15, 10, 30, 30, 0, time.Local)
	if !fi.AccessTime.Equal(expected) {
		t.Fatalf("access time: got %v, want %v", fi.AccessTime, expected)
	}
	// Mod time must be preserved.
	if fi.ModTime.Year() != 2020 {
		t.Fatalf("mod time changed: got %v, want year 2020", fi.ModTime)
	}
}

// TestReferenceFile verifies R3.1: -r copies timestamps from a reference file.
func TestReferenceFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	refFile := filepath.Join(dir, "reffile")
	if err := os.WriteFile(refFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	refTime := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(refFile, refTime, refTime); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "target")
	cmd := exec.Command(goBin, "-r", refFile, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -r failed: %v\n%s", err, out)
	}

	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime.Equal(refTime) {
		t.Fatalf("mod time: got %v, want %v", fi.ModTime, refTime)
	}
	if !fi.AccessTime.Equal(refTime) {
		t.Fatalf("access time: got %v, want %v", fi.AccessTime, refTime)
	}
}

// TestDateString verifies R3.2: -d parses date string and sets timestamps.
func TestDateString(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "target")

	cmd := exec.Command(goBin, "-d", "2024-01-15 10:30:30", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -d failed: %v\n%s", err, out)
	}

	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2024, 1, 15, 10, 30, 30, 0, time.Local)
	if !fi.ModTime.Equal(expected) {
		t.Fatalf("mod time: got %v, want %v", fi.ModTime, expected)
	}
	if !fi.AccessTime.Equal(expected) {
		t.Fatalf("access time: got %v, want %v", fi.AccessTime, expected)
	}
}

// TestDateStringEpoch verifies R3.2: -d with @epoch format.
func TestDateStringEpoch(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "target")

	cmd := exec.Command(goBin, "-d", "@1705312230", target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -d @epoch failed: %v\n%s", err, out)
	}

	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Unix(1705312230, 0)
	if !fi.ModTime.Equal(expected) {
		t.Fatalf("mod time: got %v, want %v", fi.ModTime, expected)
	}
}

// TestMissingReferenceFile verifies R3.3: error on missing reference file.
func TestMissingReferenceFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "target")

	cmd := exec.Command(goBin, "-r", "/nonexistent/reffile", target)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for missing reference file")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(string(out), "failed to get attributes") {
		t.Fatalf("expected 'failed to get attributes' in output: %s", out)
	}
	// R3.3: target should not be created when reference file is missing.
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("target should not have been created")
	}
}

// TestNoDereferenceSymlink verifies R3.4: -h affects the symlink itself.
func TestNoDereferenceSymlink(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	// Create a regular file with old timestamps.
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(target, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create a symlink pointing to the target.
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	// Touch the symlink with -h and an explicit timestamp.
	cmd := exec.Command(goBin, "-h", "-t", "202401151030.00", link)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("touch -h failed: %v\n%s", err, out)
	}

	// R3.4: target's timestamps should be unchanged.
	fi, err := sys.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if fi.ModTime.Year() != 2020 {
		t.Fatalf("target mtime changed: got %v, want year 2020", fi.ModTime)
	}

	// R3.4: symlink's own timestamps should be updated.
	lfi, err := sys.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)
	if !lfi.ModTime.Equal(expected) {
		t.Fatalf("symlink mtime: got %v, want %v", lfi.ModTime, expected)
	}
}

// TestContinueAfterError verifies R4.2: touch continues processing files
// after an error and still exits 1.
func TestContinueAfterError(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	// Create a directory with no write permission so touch fails inside it.
	noWrite := filepath.Join(dir, "noperm")
	if err := os.Mkdir(noWrite, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(noWrite, 0o755) // best-effort restore for cleanup
	})

	badFile := filepath.Join(noWrite, "blocked")
	goodFile := filepath.Join(dir, "good")

	cmd := exec.Command(goBin, badFile, goodFile)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got %v\noutput: %s", err, out)
	}

	// R4.2: goodFile must still be created despite badFile failing.
	if _, err := os.Stat(goodFile); err != nil {
		t.Fatalf("good file not created after error: %v", err)
	}
}

// TestExitZeroOnSuccess verifies R4.1: touch exits 0 on success.
func TestExitZeroOnSuccess(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "okfile")

	cmd := exec.Command(goBin, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput: %s", err, out)
	}
}
