// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mktemp against gmktemp (GNU coreutils).
//
// Covers prd036-mktemp R1.1, R1.2, R1.3, R1.4.
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

// verifyCreatedFile checks structural properties of mktemp output.
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
	verifyDirectory(t, label, absPath, expectedDir)
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
	if info.Mode().Perm() != expectedMode {
		t.Errorf("[%s] mode: got %o want %o", label, info.Mode().Perm(), expectedMode)
	}
}

// verifyDirectory checks the file was created in the expected directory.
func verifyDirectory(t *testing.T, label, absPath, expectedDir string) {
	t.Helper()
	dir := filepath.Dir(absPath)
	if dir != expectedDir {
		t.Errorf("[%s] directory: got %q want %q", label, dir, expectedDir)
	}
}
