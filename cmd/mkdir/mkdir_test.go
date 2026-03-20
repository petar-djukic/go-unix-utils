// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd034-mkdir R1.1–R1.4, R2.1–R2.3, R3.1, R3.4.
package main_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests for prd034-mkdir comparing the Go
// binary against the GNU reference binary (gmkdir).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}
	normBin := makeBinaryNormalizer(refBin)
	runSharedTests(t, goBin, refBin, normBin)
	runIsolatedTests(t, goBin, refBin, normBin)
}

// runSharedTests runs tests where both binaries can share the same WorkDir
// without diverging (error cases, -p on existing dirs, --help, --version).
func runSharedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	existsDir := prepareDir(t, "existingdir")
	parentsExistsDir := prepareDir(t, "existingdir")
	intermediateDir := prepareDir(t, "a")
	tests := []testutils.DiffTest{
		// R1.3: error when directory already exists without -p.
		{Name: "exists_error", Args: []string{"existingdir"}, WorkDir: existsDir,
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R1.4: error when parent directory does not exist.
		{Name: "missing_parent_error", Args: []string{"no/such/parent/dir"},
			Normalize: []testutils.NormalizeFunc{normBin}},
		// Error: missing operand.
		{Name: "missing_operand",
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R2.2: -p suppresses error when target already exists.
		{Name: "parents_exists_ok", Args: []string{"-p", "existingdir"},
			WorkDir: parentsExistsDir},
		// R2.3: -p succeeds when intermediate dirs already exist.
		{Name: "parents_intermediate_exists", Args: []string{"-p", "a/b/c"},
			WorkDir: intermediateDir},
		// --help output.
		{Name: "help_flag", Args: []string{"--help"},
			Normalize: []testutils.NormalizeFunc{normalizeHelpOutput}},
		// --version output.
		{Name: "version_flag", Args: []string{"--version"},
			Normalize: []testutils.NormalizeFunc{normalizeVersion}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// prepareDir creates a temp directory containing a single subdirectory.
func prepareDir(t *testing.T, subdir string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, subdir), 0o755); err != nil {
		t.Fatalf("setup: creating %s: %v", subdir, err)
	}
	return dir
}

// isolatedCase defines a test where each binary runs in its own temp dir
// to avoid cross-contamination from directory creation side effects.
type isolatedCase struct {
	name  string
	args  []string
	setup func(t *testing.T, dir string)
	norm  []testutils.NormalizeFunc
}

// runIsolatedTests runs tests where each binary needs its own WorkDir
// because both create directories (the ref would leave state that
// prevents the Go binary from matching).
func runIsolatedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		// R1.1: create a single directory.
		{name: "single_dir", args: []string{"newdir"}},
		// R1.2: create multiple directories independently.
		{name: "multiple_dirs", args: []string{"dir1", "dir2", "dir3"}},
		// R2.1: -p creates full parent chain.
		{name: "parents_chain", args: []string{"-p", "a/b/c"}},
		// R3.4: -v prints a message for each created directory.
		{name: "verbose_single", args: []string{"-v", "vdir"},
			norm: []testutils.NormalizeFunc{normBin}},
		// R3.4: -pv prints messages for all created directories.
		{name: "verbose_parents", args: []string{"-pv", "x/y"},
			norm: []testutils.NormalizeFunc{normBin}},
		// R3.1: -m sets permission mode (octal).
		{name: "mode_octal", args: []string{"-m", "0755", "modedir"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, tc)
		})
	}
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, and exit code.
func compareIsolated(t *testing.T, goBin, refBin string, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if tc.setup != nil {
		tc.setup(t, refDir)
		tc.setup(t, goDir)
	}
	refOut, refErr, refCode := execBinary(t, refBin, tc.args, refDir)
	goOut, goErr, goCode := execBinary(t, goBin, tc.args, goDir)
	refOut, goOut, refErr, goErr = applyNorm(tc.norm, refOut, goOut, refErr, goErr)
	if bytes.Equal(refOut, goOut) && bytes.Equal(refErr, goErr) && refCode == goCode {
		return
	}
	t.Fatalf("divergence\nargs:       %v\n"+
		"ref stdout: %s\ngo  stdout: %s\n"+
		"ref stderr: %s\ngo  stderr: %s\n"+
		"ref exit:   %d\ngo  exit:   %d",
		tc.args, refOut, goOut, refErr, goErr, refCode, goCode)
}

// applyNorm applies normalizers to ref and go stdout/stderr pairs.
func applyNorm(norm []testutils.NormalizeFunc, refOut, goOut, refErr, goErr []byte) ([]byte, []byte, []byte, []byte) {
	for _, n := range norm {
		refOut = n(refOut)
		goOut = n(goOut)
		refErr = n(refErr)
		goErr = n(goErr)
	}
	return refOut, goOut, refErr, goErr
}

// execBinary runs a binary with args in dir, returning stdout, stderr,
// and exit code. Fails the test on timeout or execution errors.
func execBinary(t *testing.T, bin string, args []string, dir string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = buildTestEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("binary %s failed to execute: %v", bin, err)
	}
	return stdout.Bytes(), stderr.Bytes(), 0
}

// buildTestEnv returns the process environment with LC_ALL=C set.
func buildTestEnv() []string {
	env := os.Environ()
	prefix := "LC_ALL="
	for i, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}

// makeBinaryNormalizer returns a normalizer that replaces the reference
// binary's full path and name with "mkdir", then lowercases everything
// to handle strerror() capitalization differences.
func makeBinaryNormalizer(refBin string) testutils.NormalizeFunc {
	refDir := filepath.Dir(refBin)
	return func(data []byte) []byte {
		// Replace full reference binary path (e.g., /opt/homebrew/bin/gmkdir).
		data = bytes.ReplaceAll(data, []byte(refBin), []byte("mkdir"))
		// Replace sibling path without g-prefix (e.g., /opt/homebrew/bin/mkdir)
		// in case the binary reports itself without the g-prefix.
		if refDir != "" {
			data = bytes.ReplaceAll(data, []byte(refDir+"/mkdir"), []byte("mkdir"))
		}
		data = bytes.ReplaceAll(data, []byte("gmkdir"), []byte("mkdir"))
		return bytes.ToLower(data)
	}
}

// normalizeHelpOutput keeps only the first line and normalizes the
// program name so both binaries produce identical output.
func normalizeHelpOutput(data []byte) []byte {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		data = data[:i+1]
	}
	prefix := []byte("Usage: ")
	if !bytes.HasPrefix(data, prefix) {
		return data
	}
	rest := data[len(prefix):]
	if sp := bytes.IndexByte(rest, ' '); sp >= 0 {
		normalized := make([]byte, 0, len(prefix)+5+len(rest)-sp)
		normalized = append(normalized, prefix...)
		normalized = append(normalized, []byte("mkdir")...)
		normalized = append(normalized, rest[sp:]...)
		data = normalized
	}
	return data
}

// normalizeVersion reduces version output to a common program name prefix
// so version numbers and distribution text do not cause divergence.
func normalizeVersion(data []byte) []byte {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		data = data[:i+1]
	}
	data = bytes.ReplaceAll(data, []byte("gmkdir"), []byte("mkdir"))
	if i := bytes.Index(data, []byte("mkdir")); i >= 0 {
		data = append(data[:i+5], '\n')
	}
	return data
}
