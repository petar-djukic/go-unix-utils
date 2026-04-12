// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff verifies touch behavior parity against gtouch (GNU coreutils).
// R1.1-R1.4: default behavior and file creation.
// R2.1-R2.4: timestamp selection and explicit times.
// R3.1-R3.4: reference file, date string, no-dereference.
// R4.1: exit 0 on success. R4.2: exit 1 on error, continue processing.
// R4.3: differential exit code comparison. R4.4: full case coverage.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}
	stderrNorm := makeBinaryNameNormalizer(refBin)
	errNorms := []testutils.NormalizeFunc{stderrNorm}
	stdoutBlank := []testutils.NormalizeFunc{replaceNonEmpty}

	tests := []testutils.DiffTest{
		// R1.1, R1.2, R4.1: create new file, exit 0.
		{
			Name: "create-new-file",
			Args: []string{"newfile"},
		},
		// R1.1, R4.1: update existing file (ref creates it first, go touches it).
		{
			Name: "update-existing-file",
			Args: []string{"existfile"},
		},
		// R1.3, R4.1: -c no-create on nonexistent file, exit 0.
		{
			Name: "no-create-flag",
			Args: []string{"-c", "nonexistent"},
		},
		// R1.3: --no-create long form.
		{
			Name: "no-create-long",
			Args: []string{"--no-create", "nonexistent"},
		},
		// R1.4, R4.1: multiple files.
		{
			Name: "multiple-files",
			Args: []string{"file1", "file2", "file3"},
		},
		// R2.1, R4.1: -a access only.
		{
			Name: "access-only",
			Args: []string{"-a", "afile"},
		},
		// R2.2, R4.1: -m modification only.
		{
			Name: "mod-only",
			Args: []string{"-m", "mfile"},
		},
		// R2.1, R2.2: both -a and -m (equivalent to default).
		{
			Name: "access-and-mod",
			Args: []string{"-am", "amfile"},
		},
		// R2.4, R4.1: -t explicit timestamp with seconds.
		{
			Name: "explicit-timestamp",
			Args: []string{"-t", "202401151030.00", "tsfile"},
		},
		// R2.4: -t timestamp without seconds.
		{
			Name: "explicit-timestamp-no-seconds",
			Args: []string{"-t", "202401151030", "tsfile2"},
		},
		// R2.4: -t with four-digit year.
		{
			Name: "explicit-timestamp-4digit-year",
			Args: []string{"-t", "202401151030.30", "tsfile3"},
		},
		// R2.4: -t with two-digit year.
		{
			Name: "explicit-timestamp-2digit-year",
			Args: []string{"-t", "2401151030.00", "tsfile4"},
		},
		// R2.4: -t with 8-digit (MMDDhhmm, current year).
		{
			Name: "explicit-timestamp-no-year",
			Args: []string{"-t", "01151030", "tsfile5"},
		},
		// R3.2, R4.1: -d date string ISO format.
		{
			Name: "date-string-iso",
			Args: []string{"-d", "2024-01-15 10:30:00", "dfile"},
		},
		// R3.2: -d date string date-only.
		{
			Name: "date-string-date-only",
			Args: []string{"-d", "2024-01-15", "dfile2"},
		},
		// R3.2: -d epoch format.
		{
			Name: "date-string-epoch",
			Args: []string{"-d", "@1700000000", "dfile3"},
		},
		// R3.2: --date= long form.
		{
			Name: "date-long-form",
			Args: []string{"--date=2024-06-01", "dfile4"},
		},
		// R3.3, R4.2: missing reference file, exit 1.
		{
			Name:      "ref-file-missing",
			Args:      []string{"-r", "/nonexistent_ref_file_xyz_abc", "targetfile"},
			ExitCode:  1,
			Normalize: errNorms,
		},
		// R4.2: invalid timestamp, exit 1.
		{
			Name:      "invalid-timestamp",
			Args:      []string{"-t", "notadate", "badtsfile"},
			ExitCode:  1,
			Normalize: errNorms,
		},
		// R4.2: missing file operand.
		{
			Name:      "missing-operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: errNorms,
		},
		// R3.2: invalid date string, exit 1.
		{
			Name:      "invalid-date-string",
			Args:      []string{"-d", "not-a-valid-date", "badfile"},
			ExitCode:  1,
			Normalize: errNorms,
		},
		// --help exits 0 with usage text.
		{
			Name:      "help-flag",
			Args:      []string{"--help"},
			Normalize: stdoutBlank,
		},
		// --version exits 0 with version text.
		{
			Name:      "version-flag",
			Args:      []string{"--version"},
			Normalize: stdoutBlank,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffReferenceFile verifies -r with an existing reference file.
// R3.1: use reference file timestamps. R4.3: differential comparison.
func TestDiffReferenceFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	workDir := t.TempDir()
	refFile := filepath.Join(workDir, "reffile")
	createFixture(t, refFile)

	tests := []testutils.DiffTest{
		// R3.1, R4.1: copy timestamps from reference file.
		{
			Name:    "reference-file",
			Args:    []string{"-r", refFile, "target_ref"},
			WorkDir: workDir,
		},
		// R3.1: --reference= long form.
		{
			Name:    "reference-file-long",
			Args:    []string{"--reference=" + refFile, "target_ref2"},
			WorkDir: workDir,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCombined verifies combined flag scenarios.
// R2.1, R2.4, R4.4: -a with -t. R2.2, R2.4: -m with -t.
func TestDiffCombined(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	tests := []testutils.DiffTest{
		// R2.1, R2.4: -a with explicit timestamp.
		{
			Name: "access-only-with-timestamp",
			Args: []string{"-a", "-t", "202401151030.00", "at_file"},
		},
		// R2.2, R2.4: -m with explicit timestamp.
		{
			Name: "mod-only-with-timestamp",
			Args: []string{"-m", "-t", "202401151030.00", "mt_file"},
		},
		// R2.3, R3.2: -d with both times changed.
		{
			Name: "date-both-times",
			Args: []string{"-d", "2024-01-15 10:30:00", "dt_file"},
		},
		// R1.3, R2.4: -c with -t on nonexistent file.
		{
			Name: "no-create-with-timestamp",
			Args: []string{"-c", "-t", "202401151030.00", "nc_ts_file"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorContinuation verifies that touch continues processing
// files after an error on one file. R4.2: exit 1 but process remaining.
func TestDiffErrorContinuation(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}
	stderrNorm := makeBinaryNameNormalizer(refBin)

	workDir := t.TempDir()
	// Create a directory that is not writable to cause a permission error.
	noWriteDir := filepath.Join(workDir, "noperm")
	if err := os.Mkdir(noWriteDir, 0o555); err != nil {
		t.Fatal(err)
	}
	badFile := filepath.Join(noWriteDir, "cantcreate")

	tests := []testutils.DiffTest{
		// R4.2: error on first file, but second file still processed.
		{
			Name:      "error-then-success",
			Args:      []string{badFile, filepath.Join(workDir, "goodfile")},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNorm, normalizePermError},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelpContent verifies --help output contains expected keywords.
func TestHelpContent(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help exited with error: %v", err)
	}
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Errorf("--help output missing 'Usage:': %q", out)
	}
}

// TestVersionContent verifies --version output contains the program name.
func TestVersionContent(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version exited with error: %v", err)
	}
	if !bytes.Contains(out, []byte("touch")) {
		t.Errorf("--version output missing 'touch': %q", out)
	}
}

// createFixture creates a file with known content for reference tests.
func createFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeBinaryNameNormalizer returns a NormalizeFunc that replaces the reference
// binary path with "touch" so stderr messages match between gtouch and our binary.
func makeBinaryNameNormalizer(refBin string) testutils.NormalizeFunc {
	refBase := filepath.Base(refBin)
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(progName))
		b = bytes.ReplaceAll(b, []byte(refBase), []byte(progName))
		return b
	}
}

// replaceNonEmpty replaces any non-empty output with a fixed marker.
// Used for --help and --version where content differs but both produce output.
func replaceNonEmpty(b []byte) []byte {
	if len(b) > 0 {
		return []byte("OK\n")
	}
	return b
}

// normalizePermError normalizes permission error messages that differ
// between GNU touch and our implementation.
func normalizePermError(b []byte) []byte {
	// GNU and Go may produce slightly different permission error text.
	// Normalize to just check that an error was reported.
	if len(b) > 0 {
		return []byte("ERROR\n")
	}
	return b
}
