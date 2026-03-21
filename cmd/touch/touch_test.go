// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/touch against gtouch (GNU coreutils).
// Implements prd062-touch R1.1–R1.4, R2.1–R2.4 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
