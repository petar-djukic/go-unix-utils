// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/touch against GNU gtouch.
// Covers prd062-touch R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gtouch and Go touch.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?touch|gtouch`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	// GNU uses "invalid date format" for -d; Go uses "invalid date".
	dateFmt := regexp.MustCompile(`invalid date format`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("touch"))
		b = tryHelp.ReplaceAll(b, nil)
		b = dateFmt.ReplaceAll(b, []byte("invalid date"))
		// GNU strerror capitalizes; Go syscall errors are lowercase.
		b = bytes.ReplaceAll(b,
			[]byte("No such file or directory"),
			[]byte("no such file or directory"))
		return b
	}
}

// setupRefFile creates a reference file with known timestamps for -r tests.
func setupRefFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "reffile")
	if err := os.WriteFile(p, []byte("ref"), 0o644); err != nil {
		t.Fatalf("create ref file: %v", err)
	}
	refTime := time.Date(2023, 3, 15, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(p, refTime, refTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return p
}

// makeExistingFile creates a temp dir with a file at a known timestamp.
func makeExistingFile(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	name := "existing.txt"
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("create existing file: %v", err)
	}
	known := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, known, known); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return dir, name
}

// makeSymlinkDir creates a temp dir with a target file and a symlink to it.
func makeSymlinkDir(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(targetPath, []byte("t"), 0o644); err != nil {
		t.Fatalf("create target: %v", err)
	}
	known := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(targetPath, known, known); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	linkName := "link.txt"
	linkPath := filepath.Join(dir, linkName)
	if err := os.Symlink("target.txt", linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return dir, linkName
}

// basicTests returns R4.1 test cases for file creation and update.
func basicTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	existDir, existName := makeExistingFile(t)
	return []testutils.DiffTest{
		// R4.1: create new file (ref creates, go updates; both exit 0).
		{Name: "create_new_file", Args: []string{"newfile.txt"}},
		// R4.1: update existing file timestamps.
		{
			Name:    "update_existing",
			Args:    []string{existName},
			WorkDir: existDir,
		},
		// R4.1: multiple file arguments processed in order.
		{Name: "multiple_files", Args: []string{"a.txt", "b.txt", "c.txt"}},
	}
}

// flagSelectionTests returns R4.2 test cases for -a, -m, -c, -h flags.
func flagSelectionTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	existDir, existName := makeExistingFile(t)
	symDir1, symName1 := makeSymlinkDir(t)
	symDir2, symName2 := makeSymlinkDir(t)
	return []testutils.DiffTest{
		// R4.2: -a changes only access time.
		{Name: "flag_a_access", Args: []string{"-a", "afile.txt"}},
		// R4.2: -m changes only modification time.
		{Name: "flag_m_mod", Args: []string{"-m", "mfile.txt"}},
		// R4.2: -am changes both times.
		{Name: "flag_am_both", Args: []string{"-am", "amfile.txt"}},
		// R4.2: -c suppresses creation of nonexistent file.
		{Name: "flag_c_no_create", Args: []string{"-c", "nonexistent.txt"}},
		// R4.2: -c on existing file still updates timestamps.
		{
			Name:    "flag_c_existing",
			Args:    []string{"-c", existName},
			WorkDir: existDir,
		},
		// R4.2: -h affects symlink itself, not target.
		{
			Name:    "flag_h_symlink",
			Args:    []string{"-h", symName1},
			WorkDir: symDir1,
		},
		// R4.2: --no-create long form.
		{Name: "flag_no_create_long", Args: []string{"--no-create", "nope.txt"}},
		// R4.2: --no-dereference long form.
		{
			Name:    "flag_no_deref_long",
			Args:    []string{"--no-dereference", symName2},
			WorkDir: symDir2,
		},
	}
}

// flagTimestampTests returns R4.2 test cases for -t, -d, -r flags.
func flagTimestampTests(t *testing.T, refFile string) []testutils.DiffTest {
	t.Helper()
	return []testutils.DiffTest{
		// R4.2: -t with CCYYMMDDhhmm.ss format.
		{
			Name: "flag_t_full",
			Args: []string{"-t", "202401151030.00", "tfile.txt"},
		},
		// R4.2: -t with MMDDhhmm (current year).
		{Name: "flag_t_short", Args: []string{"-t", "01151030", "tshort.txt"}},
		// R4.2: -t with CCYYMMDDhhmm (no seconds).
		{
			Name: "flag_t_century",
			Args: []string{"-t", "202401151030", "tcent.txt"},
		},
		// R4.2: -d with ISO datetime string.
		{
			Name: "flag_d_iso",
			Args: []string{"-d", "2024-01-15 10:30:00", "diso.txt"},
		},
		// R4.2: -d with @epoch timestamp.
		{
			Name: "flag_d_epoch",
			Args: []string{"-d", "@1700000000", "depoch.txt"},
		},
		// R4.2: -d with date-only string.
		{
			Name: "flag_d_date_only",
			Args: []string{"-d", "2024-01-15", "donly.txt"},
		},
		// R4.2: -r copies timestamps from reference file.
		{Name: "flag_r_ref", Args: []string{"-r", refFile, "rfile.txt"}},
		// R4.2: --reference= long form.
		{
			Name: "flag_reference_long",
			Args: []string{"--reference=" + refFile, "rlong.txt"},
		},
		// R4.2: --date= long form.
		{
			Name: "flag_date_long",
			Args: []string{"--date=2024-01-15 10:30:00", "dlong.txt"},
		},
		// R4.2: -a with -t (access time with explicit timestamp).
		{
			Name: "flag_a_with_t",
			Args: []string{"-a", "-t", "202401151030.00", "atf.txt"},
		},
		// R4.2: -m with -d (modification time with date string).
		{
			Name: "flag_m_with_d",
			Args: []string{"-m", "-d", "2024-01-15", "mdf.txt"},
		},
	}
}

// errorTests returns R4.3 test cases for error conditions.
func errorTests(errNorm testutils.NormalizeFunc) []testutils.DiffTest {
	norms := []testutils.NormalizeFunc{errNorm}
	return []testutils.DiffTest{
		// R4.3: invalid -t timestamp format.
		{
			Name:      "error_invalid_t",
			Args:      []string{"-t", "bad", "e.txt"},
			Normalize: norms,
		},
		// R4.3: invalid -d date string.
		{
			Name:      "error_invalid_d",
			Args:      []string{"-d", "not-a-date", "e.txt"},
			Normalize: norms,
		},
		// R4.3: nonexistent reference file.
		{
			Name:      "error_missing_ref",
			Args:      []string{"-r", "/nonexistent/path/ref", "e.txt"},
			Normalize: norms,
		},
		// R4.3: missing file operand.
		{
			Name:      "error_no_operand",
			Args:      []string{},
			Normalize: norms,
		},
		// R4.3: invalid option flag.
		{
			Name:      "error_invalid_option",
			Args:      []string{"-Q", "e.txt"},
			Normalize: norms,
		},
	}
}

// edgeCaseTests returns R4.4 edge case test cases.
func edgeCaseTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	symDir, symName := makeSymlinkDir(t)
	return []testutils.DiffTest{
		// R4.4: -c prevents creation of nonexistent files.
		{Name: "edge_c_prevents_creation", Args: []string{"-c", "nope.txt"}},
		// R4.4: touch symlink without -h follows the link.
		{
			Name:    "edge_symlink_follow",
			Args:    []string{symName},
			WorkDir: symDir,
		},
		// R4.4: -- separator for file arguments.
		{Name: "edge_double_dash", Args: []string{"--", "dashfile.txt"}},
		// R4.4: -t with seconds component.
		{
			Name: "edge_t_with_seconds",
			Args: []string{"-t", "202401151030.30", "sec.txt"},
		},
		// R4.4: -c with multiple nonexistent files.
		{
			Name: "edge_c_multiple",
			Args: []string{"-c", "no1.txt", "no2.txt"},
		},
		// R4.4: -d with ISO 8601 T separator.
		{
			Name: "edge_d_iso_T",
			Args: []string{"-d", "2024-01-15T10:30:00", "isot.txt"},
		},
		// R4.4: -d with epoch zero.
		{
			Name: "edge_d_epoch_zero",
			Args: []string{"-d", "@0", "e0.txt"},
		},
	}
}

// TestDiff runs differential tests comparing Go touch against GNU gtouch.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skipf("reference binary gtouch not in PATH: %v", err)
	}
	errNorm := stderrNormalizer()
	refFile := setupRefFile(t)

	var tests []testutils.DiffTest
	tests = append(tests, basicTests(t)...)
	tests = append(tests, flagSelectionTests(t)...)
	tests = append(tests, flagTimestampTests(t, refFile)...)
	tests = append(tests, errorTests(errNorm)...)
	tests = append(tests, edgeCaseTests(t)...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
