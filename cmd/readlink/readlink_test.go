// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd050-readlink R1.5-R1.6, R2.1-R2.2, R3.1-R3.2, R4.1-R4.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("greadlink")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?readlink`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("readlink"))
	})

	normalizeErrors := testutils.NormalizeFunc(func(b []byte) []byte {
		s := strings.ToLower(string(b))
		s = strings.ReplaceAll(s, "'", "")
		s = strings.ReplaceAll(s, "`", "")
		return []byte(s)
	})

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	lcEnv := []string{"LC_ALL=C"}
	errorNorm := []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrors}

	tmp := t.TempDir()

	regularFile := filepath.Join(tmp, "regular")
	if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	symlink := filepath.Join(tmp, "link")
	if err := os.Symlink(regularFile, symlink); err != nil {
		t.Fatal(err)
	}

	partialPath := filepath.Join(tmp, "nonexistent")
	missingPath := filepath.Join(tmp, "nodir", "nofile")

	tests := []testutils.DiffTest{
		// R1.1: symlink target
		{Name: "symlink-target", Args: []string{symlink}, Env: lcEnv},
		// R1.2: non-symlink operand
		{Name: "non-symlink", Args: []string{regularFile}, Env: lcEnv, ExitCode: 1},
		// R1.2: directory (not a symlink)
		{Name: "directory", Args: []string{tmp}, Env: lcEnv, ExitCode: 1},
		// R1.3: -f with existing file
		{Name: "f-existing", Args: []string{"-f", regularFile}, Env: lcEnv},
		// R1.3: -f with symlink
		{Name: "f-symlink", Args: []string{"-f", symlink}, Env: lcEnv},
		// R1.3: -f with partial path (parent exists, last component doesn't)
		{Name: "f-partial", Args: []string{"-f", partialPath}, Env: lcEnv},
		// R1.3: -f with missing parent directory
		{Name: "f-missing-parent", Args: []string{"-f", missingPath}, Env: lcEnv, ExitCode: 1},
		// R1.3: --canonicalize long form
		{Name: "f-long", Args: []string{"--canonicalize", regularFile}, Env: lcEnv},
		// R1.4: -e with existing file
		{Name: "e-existing", Args: []string{"-e", regularFile}, Env: lcEnv},
		// R1.4: -e with symlink
		{Name: "e-symlink", Args: []string{"-e", symlink}, Env: lcEnv},
		// R1.4: -e with missing path
		{Name: "e-missing", Args: []string{"-e", partialPath}, Env: lcEnv, ExitCode: 1},
		// R1.4: --canonicalize-existing long form
		{Name: "e-long", Args: []string{"--canonicalize-existing", regularFile}, Env: lcEnv},
		// R3.1: no operand
		{Name: "error-no-operand", Env: lcEnv, ExitCode: 1, Normalize: errorNorm},
		// R3.2: unknown long flag
		{Name: "error-unknown-flag", Args: []string{"--bogus"}, Env: lcEnv, ExitCode: 1, Normalize: errorNorm},
		// R3.2: unknown short flag
		{Name: "error-unknown-short-flag", Args: []string{"-Q"}, Env: lcEnv, ExitCode: 1, Normalize: errorNorm},
		// R1.5: -m with completely missing path
		{Name: "m-missing", Args: []string{"-m", missingPath}, Env: lcEnv},
		// R1.5: -m with partial path (parent exists, file doesn't)
		{Name: "m-partial", Args: []string{"-m", partialPath}, Env: lcEnv},
		// R1.5: -m with existing file
		{Name: "m-existing", Args: []string{"-m", regularFile}, Env: lcEnv},
		// R1.5: -m with symlink
		{Name: "m-symlink", Args: []string{"-m", symlink}, Env: lcEnv},
		// R1.5: --canonicalize-missing long form
		{Name: "m-long", Args: []string{"--canonicalize-missing", missingPath}, Env: lcEnv},
		// R1.6: -n with single operand (no trailing newline)
		{Name: "n-single", Args: []string{"-n", symlink}, Env: lcEnv},
		// R1.6: -n with -f
		{Name: "nf-single", Args: []string{"-nf", regularFile}, Env: lcEnv},
		// R1.6: --no-newline long form
		{Name: "n-long", Args: []string{"--no-newline", symlink}, Env: lcEnv},
		// R2.1: multiple operands
		{Name: "multi-operands", Args: []string{"-f", regularFile, symlink}, Env: lcEnv},
		// R2.2: multiple operands with -n (newlines still printed)
		{Name: "multi-n-ignored", Args: []string{"-nf", regularFile, symlink}, Env: lcEnv, Normalize: []testutils.NormalizeFunc{normalizeBinaryName}},
		// --help and --version (discard stdout since text differs)
		{Name: "help", Args: []string{"--help"}, Env: lcEnv, Normalize: []testutils.NormalizeFunc{discardStdout}},
		{Name: "version", Args: []string{"--version"}, Env: lcEnv, Normalize: []testutils.NormalizeFunc{discardStdout}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
