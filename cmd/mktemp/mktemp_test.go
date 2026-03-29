// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mktemp against gmktemp (GNU coreutils).
//
// Covers prd036-mktemp R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3,
// R3.1, R3.2, R3.3.
// Because mktemp generates random names, tests verify structural properties
// (exit code, path prefix, name pattern, permissions) rather than exact output.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for error cases where exit code
// comparison is sufficient (stdout/stderr text differs due to binary name).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.3: too few X's — two X's, minimum is three
		{
			Name:      "too_few_xs_two",
			Args:      []string{"fooXX"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: no trailing X's at all
		{
			Name:      "no_trailing_xs",
			Args:      []string{"foobar"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: single X — still below minimum
		{
			Name:      "too_few_xs_one",
			Args:      []string{"fooX"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMktempDefault verifies default file creation behavior.
// R1.1: file created in TMPDIR.
// R1.2: name matches tmp.XXXXXXXXXX pattern (10 random alphanumeric chars).
// R1.4: file has mode 0600.
func TestMktempDefault(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	// R1.1: default creates in TMPDIR
	t.Run("default_in_tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		env := []string{"TMPDIR=" + tmpDir}
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)

		goPath, goExit := runMktemp(t, goBin, nil, env, "")
		refPath, refExit := runMktemp(t, refBin, nil, env, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, tmpDir, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, tmpDir, pattern, 0o600)
	})
}

// TestMktempCustomTemplate verifies custom template handling.
// R1.3: trailing X's replaced with random characters.
// R1.4: file has mode 0600.
func TestMktempCustomTemplate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	// R1.3: custom template with six X's
	t.Run("custom_six_xs", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^myapp\.[A-Za-z0-9]{6}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"myapp.XXXXXX"}, nil, goDir)
		refPath, refExit := runMktemp(t, refBin, []string{"myapp.XXXXXX"}, nil, refDir)

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goDir, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refDir, pattern, 0o600)
	})

	// R1.3: minimum three X's
	t.Run("minimum_three_xs", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^test\.[A-Za-z0-9]{3}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"test.XXX"}, nil, goDir)
		refPath, refExit := runMktemp(t, refBin, []string{"test.XXX"}, nil, refDir)

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goDir, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refDir, pattern, 0o600)
	})

	// R1.3: template with directory component
	t.Run("template_with_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^app\.[A-Za-z0-9]{6}$`)

		goTmpl := filepath.Join(goDir, "app.XXXXXX")
		refTmpl := filepath.Join(refDir, "app.XXXXXX")

		goPath, goExit := runMktemp(t, goBin, []string{goTmpl}, nil, "")
		refPath, refExit := runMktemp(t, refBin, []string{refTmpl}, nil, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goDir, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refDir, pattern, 0o600)
	})
}

// TestMktempDirectory verifies -d/--directory creates directories.
// R2.1: -d creates a directory instead of a file.
// R2.2: directory has mode 0700.
// R2.3: prints absolute path of the created directory.
func TestMktempDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	// R2.1, R2.2, R2.3: -d creates directory with mode 0700
	t.Run("dir_short_flag", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		env := []string{"TMPDIR=" + tmpDir}
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"-d"}, env, "")
		refPath, refExit := runMktemp(t, refBin, []string{"-d"}, env, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedDir(t, "go", goPath, tmpDir, pattern, 0o700)
		verifyCreatedDir(t, "ref", refPath, tmpDir, pattern, 0o700)
	})

	// R2.1: --directory long flag
	t.Run("dir_long_flag", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		env := []string{"TMPDIR=" + tmpDir}
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"--directory"}, env, "")
		refPath, refExit := runMktemp(t, refBin, []string{"--directory"}, env, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedDir(t, "go", goPath, tmpDir, pattern, 0o700)
		verifyCreatedDir(t, "ref", refPath, tmpDir, pattern, 0o700)
	})

	// R2.1: -d with custom template
	t.Run("dir_custom_template", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^mydir\.[A-Za-z0-9]{6}$`)

		goTmpl := filepath.Join(goDir, "mydir.XXXXXX")
		refTmpl := filepath.Join(refDir, "mydir.XXXXXX")

		goPath, goExit := runMktemp(t, goBin, []string{"-d", goTmpl}, nil, "")
		refPath, refExit := runMktemp(t, refBin, []string{"-d", refTmpl}, nil, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedDir(t, "go", goPath, goDir, pattern, 0o700)
		verifyCreatedDir(t, "ref", refPath, refDir, pattern, 0o700)
	})
}

// TestMktempExitCodes verifies R1.5: exit 0 on success, exit 1 on failure.
func TestMktempExitCodes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	// R1.5: success exits 0
	t.Run("success_exit_0", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		env := []string{"TMPDIR=" + tmpDir}
		_, goExit := runMktemp(t, goBin, nil, env, "")
		_, refExit := runMktemp(t, refBin, nil, env, "")
		compareExitCodes(t, goExit, refExit)
		if goExit != 0 {
			t.Errorf("expected exit 0 on success, got %d", goExit)
		}
	})

	// R1.5: failure exits 1
	t.Run("failure_exit_1", func(t *testing.T) {
		t.Parallel()
		_, goExit := runMktemp(t, goBin, []string{"noX"}, nil, "")
		_, refExit := runMktemp(t, refBin, []string{"noX"}, nil, "")
		compareExitCodes(t, goExit, refExit)
		if goExit != 1 {
			t.Errorf("expected exit 1 on failure, got %d", goExit)
		}
	})

	// R1.5: failure prints error to stderr
	t.Run("failure_stderr_not_empty", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "noX")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		_ = cmd.Run() // expected to fail
		if stderr.Len() == 0 {
			t.Error("expected error message on stderr for invalid template")
		}
	})
}

// TestMktempParentDir verifies R3.1: -p DIR and --tmpdir=DIR.
func TestMktempParentDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	// R3.1: -p DIR uses DIR as parent
	t.Run("p_flag_default_template", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"-p", goDir}, nil, "")
		refPath, refExit := runMktemp(t, refBin, []string{"-p", refDir}, nil, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goDir, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refDir, pattern, 0o600)
	})

	// R3.1: -p DIR with custom template
	t.Run("p_flag_custom_template", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^myapp\.[A-Za-z0-9]{6}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"-p", goDir, "myapp.XXXXXX"}, nil, "")
		refPath, refExit := runMktemp(t, refBin, []string{"-p", refDir, "myapp.XXXXXX"}, nil, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goDir, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refDir, pattern, 0o600)
	})

	// R3.1: --tmpdir=DIR uses DIR as parent
	t.Run("tmpdir_equals_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"--tmpdir=" + goDir}, nil, "")
		refPath, refExit := runMktemp(t, refBin, []string{"--tmpdir=" + refDir}, nil, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goDir, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refDir, pattern, 0o600)
	})

	// R3.1: -p DIR with -d (directory mode)
	t.Run("p_flag_directory_mode", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"-d", "-p", goDir}, nil, "")
		refPath, refExit := runMktemp(t, refBin, []string{"-d", "-p", refDir}, nil, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedDir(t, "go", goPath, goDir, pattern, 0o700)
		verifyCreatedDir(t, "ref", refPath, refDir, pattern, 0o700)
	})
}

// TestMktempTmpdirNoValue verifies R3.2: --tmpdir without value.
func TestMktempTmpdirNoValue(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	// R3.2: --tmpdir (no value) with custom template uses TMPDIR
	t.Run("tmpdir_no_value_with_template", func(t *testing.T) {
		t.Parallel()
		goTmp := t.TempDir()
		refTmp := t.TempDir()
		pattern := regexp.MustCompile(`^myapp\.[A-Za-z0-9]{6}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"--tmpdir", "myapp.XXXXXX"}, []string{"TMPDIR=" + goTmp}, "")
		refPath, refExit := runMktemp(t, refBin, []string{"--tmpdir", "myapp.XXXXXX"}, []string{"TMPDIR=" + refTmp}, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goTmp, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refTmp, pattern, 0o600)
	})

	// R3.2: --tmpdir (no value) without template uses TMPDIR + default
	t.Run("tmpdir_no_value_default_template", func(t *testing.T) {
		t.Parallel()
		goTmp := t.TempDir()
		refTmp := t.TempDir()
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)

		goPath, goExit := runMktemp(t, goBin, []string{"--tmpdir"}, []string{"TMPDIR=" + goTmp}, "")
		refPath, refExit := runMktemp(t, refBin, []string{"--tmpdir"}, []string{"TMPDIR=" + refTmp}, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goTmp, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refTmp, pattern, 0o600)
	})
}

// TestMktempSuffix verifies R3.3: --suffix=SUFF.
func TestMktempSuffix(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	// R3.3: --suffix=.txt appends suffix after random characters
	t.Run("suffix_txt", func(t *testing.T) {
		t.Parallel()
		goTmp := t.TempDir()
		refTmp := t.TempDir()
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}\.txt$`)

		goPath, goExit := runMktemp(t, goBin, []string{"--suffix=.txt"}, []string{"TMPDIR=" + goTmp}, "")
		refPath, refExit := runMktemp(t, refBin, []string{"--suffix=.txt"}, []string{"TMPDIR=" + refTmp}, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goTmp, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refTmp, pattern, 0o600)
	})

	// R3.3: --suffix with custom template and -p
	t.Run("suffix_with_template", func(t *testing.T) {
		t.Parallel()
		goTmp := t.TempDir()
		refTmp := t.TempDir()
		pattern := regexp.MustCompile(`^myapp\.[A-Za-z0-9]{6}\.log$`)

		goPath, goExit := runMktemp(t, goBin, []string{"--suffix=.log", "-p", goTmp, "myapp.XXXXXX"}, nil, "")
		refPath, refExit := runMktemp(t, refBin, []string{"--suffix=.log", "-p", refTmp, "myapp.XXXXXX"}, nil, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedFile(t, "go", goPath, goTmp, pattern, 0o600)
		verifyCreatedFile(t, "ref", refPath, refTmp, pattern, 0o600)
	})

	// R3.3: suffix with slash is rejected
	t.Run("suffix_with_slash_fails", func(t *testing.T) {
		t.Parallel()
		_, goExit := runMktemp(t, goBin, []string{"--suffix=/bad"}, nil, "")
		_, refExit := runMktemp(t, refBin, []string{"--suffix=/bad"}, nil, "")
		compareExitCodes(t, goExit, refExit)
		if goExit != 1 {
			t.Errorf("expected exit 1 for suffix with slash, got %d", goExit)
		}
	})

	// R3.3: --suffix with -d (directory mode)
	t.Run("suffix_directory_mode", func(t *testing.T) {
		t.Parallel()
		goTmp := t.TempDir()
		refTmp := t.TempDir()
		pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}\.dat$`)

		goPath, goExit := runMktemp(t, goBin, []string{"-d", "--suffix=.dat"}, []string{"TMPDIR=" + goTmp}, "")
		refPath, refExit := runMktemp(t, refBin, []string{"-d", "--suffix=.dat"}, []string{"TMPDIR=" + refTmp}, "")

		compareExitCodes(t, goExit, refExit)
		verifyCreatedDir(t, "go", goPath, goTmp, pattern, 0o700)
		verifyCreatedDir(t, "ref", refPath, refTmp, pattern, 0o700)
	})
}

// runMktemp executes a mktemp binary and returns the output path and exit code.
func runMktemp(t *testing.T, bin string, args, extraEnv []string, workDir string) (string, int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Env = append(cmd.Env, extraEnv...)

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

	return strings.TrimRight(stdout.String(), "\n"), exitCode
}

// compareExitCodes verifies both binaries returned the same exit code.
func compareExitCodes(t *testing.T, goExit, refExit int) {
	t.Helper()
	if goExit != refExit {
		t.Errorf("exit code divergence: go=%d ref=%d", goExit, refExit)
	}
}

// verifyCreatedFile checks structural properties of mktemp file output.
// R4.2: path in expected dir, name matches pattern, file exists, mode correct.
func verifyCreatedFile(
	t *testing.T, label, output, expectedDir string,
	pattern *regexp.Regexp, expectedMode os.FileMode,
) {
	t.Helper()

	if output == "" {
		t.Errorf("[%s] empty output", label)
		return
	}

	absPath := resolvePath(t, output, expectedDir)
	verifyBaseName(t, label, output, pattern)
	verifyFileExists(t, label, absPath, expectedMode)
	verifyParentDir(t, label, absPath, expectedDir)
}

// verifyCreatedDir checks structural properties of mktemp directory output.
// R2.2: directory exists with mode 0700.
// R2.3: output is the absolute path of the created directory.
func verifyCreatedDir(
	t *testing.T, label, output, expectedDir string,
	pattern *regexp.Regexp, expectedMode os.FileMode,
) {
	t.Helper()

	if output == "" {
		t.Errorf("[%s] empty output", label)
		return
	}

	absPath := resolvePath(t, output, expectedDir)
	verifyBaseName(t, label, output, pattern)
	verifyDirExists(t, label, absPath, expectedMode)
	verifyParentDir(t, label, absPath, expectedDir)
}

// resolvePath converts a potentially relative path to absolute.
func resolvePath(t *testing.T, path, workDir string) string {
	t.Helper()
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}

// verifyBaseName checks the filename matches the expected pattern.
func verifyBaseName(t *testing.T, label, path string, pattern *regexp.Regexp) {
	t.Helper()
	base := filepath.Base(path)
	if !pattern.MatchString(base) {
		t.Errorf("[%s] name %q does not match pattern %s", label, base, pattern)
	}
}

// verifyFileExists checks the file exists with the expected permissions.
// R1.4: mode 0600.
func verifyFileExists(t *testing.T, label, absPath string, expectedMode os.FileMode) {
	t.Helper()
	info, err := os.Stat(absPath)
	if err != nil {
		t.Errorf("[%s] file not found: %v", label, err)
		return
	}
	if info.IsDir() {
		t.Errorf("[%s] expected file but got directory: %s", label, absPath)
		return
	}
	if info.Mode().Perm() != expectedMode {
		t.Errorf("[%s] mode: got %o want %o", label, info.Mode().Perm(), expectedMode)
	}
}

// verifyDirExists checks the directory exists with the expected permissions.
// R2.2: mode 0700.
func verifyDirExists(t *testing.T, label, absPath string, expectedMode os.FileMode) {
	t.Helper()
	info, err := os.Stat(absPath)
	if err != nil {
		t.Errorf("[%s] directory not found: %v", label, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("[%s] expected directory but got file: %s", label, absPath)
		return
	}
	if info.Mode().Perm() != expectedMode {
		t.Errorf("[%s] mode: got %o want %o", label, info.Mode().Perm(), expectedMode)
	}
}

// verifyParentDir checks the file/directory was created in the expected directory.
func verifyParentDir(t *testing.T, label, absPath, expectedDir string) {
	t.Helper()
	dir := filepath.Dir(absPath)
	if dir != expectedDir {
		t.Errorf("[%s] directory: got %q want %q", label, dir, expectedDir)
	}
}
