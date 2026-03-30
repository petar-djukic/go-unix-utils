// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/touch against gtouch (GNU coreutils).
//
// Covers prd062-touch R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where the binary name prefix differs.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for cases where both binaries can
// share a WorkDir without conflict (error cases, no-create).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	// R1.3: -c with nonexistent file — no file created, exit 0
	t.Run("no_create_nonexistent", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:     "no_create_short",
				Args:     []string{"-c", "nonexistent"},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.3: --no-create long form
	t.Run("no_create_long", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:     "no_create_long",
				Args:     []string{"--no-create", "nonexistent"},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1: touch existing file updates timestamps, exit 0
	t.Run("update_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		existing := filepath.Join(dir, "existing")
		if err := os.WriteFile(existing, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		tests := []testutils.DiffTest{
			{
				Name:     "update_existing",
				Args:     []string{existing},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// Missing operand — error exit 1
	t.Run("missing_operand", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "no_args",
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{discardAll},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R2.4: invalid -t stamp — exit 1
	t.Run("invalid_t_stamp", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "bad_stamp",
				Args:      []string{"-t", "notadate", "file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{discardAll},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.3: -r with nonexistent reference file — exit 1
	t.Run("ref_missing", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "ref_not_found",
				Args:      []string{"-r", "nonexistent", "target"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{discardAll},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.2: invalid -d string — exit 1
	t.Run("invalid_d_string", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "bad_date",
				Args:      []string{"-d", "notadate", "file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{discardAll},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestTouchCreate verifies file creation by running both binaries
// in separate temp dirs and comparing exit codes and file existence.
// R1.2: create file when it does not exist.
func TestTouchCreate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	// R1.2: create a single new file
	t.Run("create_single", func(t *testing.T) {
		t.Parallel()
		compareTouch(t, goBin, refBin, []string{"newfile"})
		verifyFilesCreated(t, goBin, []string{"newfile"})
	})

	// R1.4: create multiple files
	t.Run("create_multiple", func(t *testing.T) {
		t.Parallel()
		compareTouch(t, goBin, refBin, []string{"a", "b", "c"})
		verifyFilesCreated(t, goBin, []string{"a", "b", "c"})
	})
}

// TestTouchNoCreateVerify confirms -c does not create files.
// R1.3: -c suppresses creation without error.
func TestTouchNoCreateVerify(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	target := filepath.Join(dir, "should_not_exist")

	_, stderr, exitCode := execBin(t, goBin, []string{"-c", target}, dir)
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d (stderr=%q)", exitCode, stderr)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("file should not have been created with -c")
	}
}

// TestTouchExplicitStamp verifies -t sets timestamps correctly.
// R2.4: -t STAMP uses [[CC]YY]MMDDhhmm[.ss].
func TestTouchExplicitStamp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	// R2.4: 12-digit CCYYMMDDhhmm with .ss
	t.Run("ccyymmdd_with_seconds", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, nil,
			[]string{"-t", "202401151030.45", "file"}, "file")
	})

	// R2.4: 12-digit CCYYMMDDhhmm without seconds
	t.Run("ccyymmdd_no_seconds", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, nil,
			[]string{"-t", "202401151030", "file"}, "file")
	})

	// R2.4: 10-digit YYMMDDhhmm
	t.Run("yymmdd", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, nil,
			[]string{"-t", "2401151030", "file"}, "file")
	})

	// R2.4: 8-digit MMDDhhmm (uses current year)
	t.Run("mmdd", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, nil,
			[]string{"-t", "01151030", "file"}, "file")
	})

	// R2.4: -t combined with other short flags (-at sets atime via stamp)
	t.Run("combined_at", func(t *testing.T) {
		t.Parallel()
		past := time.Date(2020, 6, 15, 12, 0, 0, 0, time.Local)
		setup := &fileSetup{name: "file", atime: past, mtime: past}
		compareTouchTimestamps(t, goBin, refBin, setup,
			[]string{"-at", "202401151030.00", "file"}, "file")
	})
}

// TestTouchAccessOnly verifies -a changes only the access time.
// R2.1: -a changes only access time.
func TestTouchAccessOnly(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	past := time.Date(2020, 6, 15, 12, 0, 0, 0, time.Local)

	// R2.1: -a with -t on existing file preserves mtime
	t.Run("a_with_t_existing", func(t *testing.T) {
		t.Parallel()
		setup := &fileSetup{name: "file", atime: past, mtime: past}
		compareTouchTimestamps(t, goBin, refBin, setup,
			[]string{"-a", "-t", "202401151030.00", "file"}, "file")
	})

	// R2.1: -a combined with -m sets both (same as default)
	t.Run("a_and_m_existing", func(t *testing.T) {
		t.Parallel()
		setup := &fileSetup{name: "file", atime: past, mtime: past}
		compareTouchTimestamps(t, goBin, refBin, setup,
			[]string{"-a", "-m", "-t", "202501011200.00", "file"}, "file")
	})
}

// TestTouchModOnly verifies -m changes only the modification time.
// R2.2: -m changes only modification time.
func TestTouchModOnly(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	past := time.Date(2020, 6, 15, 12, 0, 0, 0, time.Local)

	// R2.2: -m with -t on existing file preserves atime
	t.Run("m_with_t_existing", func(t *testing.T) {
		t.Parallel()
		setup := &fileSetup{name: "file", atime: past, mtime: past}
		compareTouchTimestamps(t, goBin, refBin, setup,
			[]string{"-m", "-t", "202401151030.00", "file"}, "file")
	})

	// R2.2: -m combined with -a sets both (same as default)
	t.Run("m_and_a_existing", func(t *testing.T) {
		t.Parallel()
		setup := &fileSetup{name: "file", atime: past, mtime: past}
		compareTouchTimestamps(t, goBin, refBin, setup,
			[]string{"-m", "-a", "-t", "202501011200.00", "file"}, "file")
	})
}

// TestTouchBothAM verifies -a -m changes both timestamps.
// R2.3: both -a and -m changes both.
func TestTouchBothAM(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	past := time.Date(2020, 6, 15, 12, 0, 0, 0, time.Local)
	setup := &fileSetup{name: "file", atime: past, mtime: past}

	t.Run("am_with_t", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, setup,
			[]string{"-a", "-m", "-t", "202401151030.00", "file"}, "file")
	})
}

// TestTouchRefFile verifies -r FILE copies timestamps from reference file.
// R3.1: -r FILE or --reference=FILE uses reference timestamps.
func TestTouchRefFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	past := time.Date(2020, 3, 15, 8, 30, 0, 0, time.Local)

	// R3.1: -r copies both timestamps to new file
	t.Run("ref_both", func(t *testing.T) {
		t.Parallel()
		compareTouchWithSetups(t, goBin, refBin,
			[]*fileSetup{{name: "ref", atime: past, mtime: past}},
			[]string{"-r", "ref", "target"}, "target")
	})

	// R3.1: --reference=FILE long form
	t.Run("ref_long_form", func(t *testing.T) {
		t.Parallel()
		compareTouchWithSetups(t, goBin, refBin,
			[]*fileSetup{{name: "ref", atime: past, mtime: past}},
			[]string{"--reference=ref", "target"}, "target")
	})

	// R3.1: -r with -a sets only access time from reference
	t.Run("ref_access_only", func(t *testing.T) {
		t.Parallel()
		older := time.Date(2019, 1, 1, 0, 0, 0, 0, time.Local)
		compareTouchWithSetups(t, goBin, refBin,
			[]*fileSetup{
				{name: "ref", atime: past, mtime: past},
				{name: "target", atime: older, mtime: older},
			},
			[]string{"-a", "-r", "ref", "target"}, "target")
	})

	// R3.1: -r with -m sets only modification time from reference
	t.Run("ref_mod_only", func(t *testing.T) {
		t.Parallel()
		older := time.Date(2019, 1, 1, 0, 0, 0, 0, time.Local)
		compareTouchWithSetups(t, goBin, refBin,
			[]*fileSetup{
				{name: "ref", atime: past, mtime: past},
				{name: "target", atime: older, mtime: older},
			},
			[]string{"-m", "-r", "ref", "target"}, "target")
	})
}

// TestTouchDateString verifies -d STRING parses date strings.
// R3.2: -d STRING or --date=STRING parses date and uses as timestamp.
func TestTouchDateString(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	// R3.2: -d with epoch seconds
	t.Run("date_epoch", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, nil,
			[]string{"-d", "@1705312200", "file"}, "file")
	})

	// R3.2: --date= long form with epoch
	t.Run("date_long_form", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, nil,
			[]string{"--date=@1705312200", "file"}, "file")
	})

	// R3.2: -d with ISO date-time
	t.Run("date_iso_datetime", func(t *testing.T) {
		t.Parallel()
		compareTouchTimestamps(t, goBin, refBin, nil,
			[]string{"-d", "2024-01-15 10:30:00", "file"}, "file")
	})

	// R3.2: -d with -a sets only access time
	t.Run("date_access_only", func(t *testing.T) {
		t.Parallel()
		past := time.Date(2020, 6, 15, 12, 0, 0, 0, time.Local)
		setup := &fileSetup{name: "file", atime: past, mtime: past}
		compareTouchTimestamps(t, goBin, refBin, setup,
			[]string{"-a", "-d", "@1705312200", "file"}, "file")
	})
}

// TestTouchNoDereference verifies -h affects the symlink itself.
// R3.4: -h or --no-dereference changes the symlink, not its target.
func TestTouchNoDereference(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	// R3.4: -h with -t on a symlink
	t.Run("h_symlink", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, dir := range []string{goDir, refDir} {
			setupSymlink(t, dir, "target", "link")
		}

		args := []string{"-h", "-t", "202401151030.00", "link"}
		_, _, goExit := execBin(t, goBin, args, goDir)
		_, _, refExit := execBin(t, refBin, args, refDir)

		if goExit != refExit {
			t.Errorf("exit code divergence: go=%d ref=%d", goExit, refExit)
		}

		// Compare symlink timestamps (not target)
		compareLstatTimestamps(t, goDir, refDir, "link")
	})

	// R3.4: --no-dereference long form
	t.Run("no_deref_long", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, dir := range []string{goDir, refDir} {
			setupSymlink(t, dir, "target", "link")
		}

		args := []string{"--no-dereference", "-t", "202401151030.00", "link"}
		_, _, goExit := execBin(t, goBin, args, goDir)
		_, _, refExit := execBin(t, refBin, args, refDir)

		if goExit != refExit {
			t.Errorf("exit code divergence: go=%d ref=%d", goExit, refExit)
		}

		compareLstatTimestamps(t, goDir, refDir, "link")
	})

	// R3.4: -h on nonexistent symlink with -c does not create
	t.Run("h_no_create", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:     "h_c_nonexistent",
				Args:     []string{"-h", "-c", "nonexistent"},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// fileSetup describes a pre-existing file with known timestamps.
type fileSetup struct {
	name  string
	atime time.Time
	mtime time.Time
}

// setupTestFile creates a file with specified timestamps in dir.
func setupTestFile(t *testing.T, dir string, fs *fileSetup) {
	t.Helper()
	path := filepath.Join(dir, fs.name)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fs.atime, fs.mtime); err != nil {
		t.Fatal(err)
	}
}

// setupSymlink creates a target file and a symlink to it.
func setupSymlink(t *testing.T, dir, targetName, linkName string) {
	t.Helper()
	target := filepath.Join(dir, targetName)
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, linkName)
	if err := os.Symlink(targetName, link); err != nil {
		t.Fatal(err)
	}
}

// statFunc is used to select between sys.Stat and sys.Lstat.
type statFunc func(string) (*sys.FileInfo, error)

// compareTimestamps compares atime and mtime using the given stat function.
func compareTimestamps(t *testing.T, goDir, refDir, name string, sf statFunc) {
	t.Helper()
	goSt, err := sf(filepath.Join(goDir, name))
	if err != nil {
		t.Fatalf("stat go file: %v", err)
	}
	refSt, err := sf(filepath.Join(refDir, name))
	if err != nil {
		t.Fatalf("stat ref file: %v", err)
	}
	if !goSt.AccessTime.Equal(refSt.AccessTime) {
		t.Errorf("atime divergence: go=%v ref=%v",
			goSt.AccessTime, refSt.AccessTime)
	}
	if !goSt.ModTime.Equal(refSt.ModTime) {
		t.Errorf("mtime divergence: go=%v ref=%v",
			goSt.ModTime, refSt.ModTime)
	}
}

// compareFileTimestamps compares atime and mtime of a file in two dirs.
func compareFileTimestamps(t *testing.T, goDir, refDir, name string) {
	t.Helper()
	compareTimestamps(t, goDir, refDir, name, sys.Stat)
}

// compareLstatTimestamps compares atime and mtime of a symlink in two dirs.
func compareLstatTimestamps(t *testing.T, goDir, refDir, name string) {
	t.Helper()
	compareTimestamps(t, goDir, refDir, name, sys.Lstat)
}

// compareTouchTimestamps runs both binaries and compares resulting timestamps.
func compareTouchTimestamps(
	t *testing.T, goBin, refBin string,
	setup *fileSetup, args []string, name string,
) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()

	if setup != nil {
		setupTestFile(t, goDir, setup)
		setupTestFile(t, refDir, setup)
	}

	_, _, goExit := execBin(t, goBin, args, goDir)
	_, _, refExit := execBin(t, refBin, args, refDir)

	if goExit != refExit {
		t.Errorf("exit code divergence: go=%d ref=%d", goExit, refExit)
	}

	compareFileTimestamps(t, goDir, refDir, name)
}

// compareTouchWithSetups runs both binaries with multiple file setups
// and compares the resulting timestamps of checkFile.
func compareTouchWithSetups(
	t *testing.T, goBin, refBin string,
	setups []*fileSetup, args []string, checkFile string,
) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, s := range setups {
		setupTestFile(t, goDir, s)
		setupTestFile(t, refDir, s)
	}

	_, _, goExit := execBin(t, goBin, args, goDir)
	_, _, refExit := execBin(t, refBin, args, refDir)

	if goExit != refExit {
		t.Errorf("exit code divergence: go=%d ref=%d", goExit, refExit)
	}

	compareFileTimestamps(t, goDir, refDir, checkFile)
}

// compareTouch runs both binaries in separate temp dirs and compares
// exit codes and stdout/stderr.
func compareTouch(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()

	refStdout, refStderr, refExit := execBin(t, refBin, args, refDir)
	goStdout, goStderr, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d (args=%v)",
			refExit, goExit, args)
	}
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout divergence:\nref: %q\ngo:  %q", refStdout, goStdout)
	}
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr divergence:\nref: %q\ngo:  %q", refStderr, goStderr)
	}
}

// verifyFilesCreated runs the Go binary and checks files were created.
func verifyFilesCreated(t *testing.T, goBin string, names []string) {
	t.Helper()
	dir := t.TempDir()

	_, stderr, exitCode := execBin(t, goBin, names, dir)
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d (stderr=%q)", exitCode, stderr)
	}

	for _, name := range names {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("file %q not created: %v", name, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("%q should be a file, not a directory", name)
		}
		if info.Size() != 0 {
			t.Errorf("%q should be empty, got size %d", name, info.Size())
		}
	}
}

// execBin runs a binary and returns stdout, stderr, and exit code.
func execBin(t *testing.T, bin string, args []string, workDir string) ([]byte, []byte, int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode
}
