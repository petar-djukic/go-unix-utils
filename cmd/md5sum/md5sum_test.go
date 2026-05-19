// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmd5sum")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	helloFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(helloFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	helloHash := md5hex([]byte("hello\n"))
	emptyHash := md5hex([]byte{})

	validChecksum := filepath.Join(tmpDir, "valid.md5")
	writeFile(t, validChecksum, fmt.Sprintf("%s  %s\n", helloHash, helloFile))

	multiChecksum := filepath.Join(tmpDir, "multi.md5")
	writeFile(t, multiChecksum, fmt.Sprintf("%s  %s\n%s  %s\n", helloHash, helloFile, emptyHash, emptyFile))

	mismatchChecksum := filepath.Join(tmpDir, "mismatch.md5")
	writeFile(t, mismatchChecksum, fmt.Sprintf("%s  %s\n", "00000000000000000000000000000000", helloFile))

	mixedChecksum := filepath.Join(tmpDir, "mixed.md5")
	writeFile(t, mixedChecksum, fmt.Sprintf("%s  %s\n%s  %s\n", helloHash, helloFile, "00000000000000000000000000000000", emptyFile))

	malformedChecksum := filepath.Join(tmpDir, "malformed.md5")
	writeFile(t, malformedChecksum, fmt.Sprintf("this is not a checksum\n%s  %s\n", helloHash, helloFile))

	bsdChecksum := filepath.Join(tmpDir, "bsd.md5")
	writeFile(t, bsdChecksum, fmt.Sprintf("MD5 (%s) = %s\n", helloFile, helloHash))

	tests := []testutils.DiffTest{
		// R1.2: stdin with no arguments
		{
			Name:  "stdin-no-args",
			Stdin: []byte("abc"),
		},
		// R1.2: stdin with explicit "-"
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("abc"),
		},
		// R1.1: single file, text mode (default)
		{
			Name: "single-file-text",
			Args: []string{helloFile},
		},
		// R1.1: single file, binary mode
		{
			Name: "single-file-binary",
			Args: []string{"-b", helloFile},
		},
		// R1.3: BSD tag format
		{
			Name: "tag-format",
			Args: []string{"--tag", helloFile},
		},
		// R1.3: tag with binary flag (mode flag has no effect on tag)
		{
			Name: "tag-with-binary",
			Args: []string{"--tag", "-b", helloFile},
		},
		// R1.1: multiple files
		{
			Name: "multiple-files",
			Args: []string{helloFile, emptyFile},
		},
		// R1.2: empty stdin
		{
			Name:  "empty-stdin",
			Stdin: []byte{},
		},
		// R1.1: empty file
		{
			Name: "empty-file",
			Args: []string{emptyFile},
		},
		// R1.4: missing file exits 1, prints error to stderr
		{
			Name:      "missing-file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R1.4: missing file among valid files continues processing
		{
			Name:      "missing-among-valid",
			Args:      []string{helloFile, filepath.Join(tmpDir, "nonexistent.txt"), emptyFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R1.1: text mode explicit flag
		{
			Name: "explicit-text-mode",
			Args: []string{"-t", helloFile},
		},
		// R1.2: stdin with newline-terminated content
		{
			Name:  "stdin-with-newline",
			Stdin: []byte("hello world\n"),
		},
		// R2.1, R2.2: --check with valid checksum file
		{
			Name:      "check-valid",
			Args:      []string{"--check", validChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.1, R2.2: --check with multiple valid entries
		{
			Name:      "check-multi-valid",
			Args:      []string{"--check", multiChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.2: --check with mismatched hash exits 1
		{
			Name:      "check-mismatch",
			Args:      []string{"--check", mismatchChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.2: --check with mix of valid and invalid
		{
			Name:      "check-mixed",
			Args:      []string{"--check", mixedChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.1: --check with BSD tag format
		{
			Name:      "check-bsd-tag",
			Args:      []string{"--check", bsdChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --warn with malformed lines
		{
			Name:      "check-warn-malformed",
			Args:      []string{"--check", "--warn", malformedChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.4: --check --quiet suppresses OK lines
		{
			Name:      "check-quiet-all-ok",
			Args:      []string{"--check", "--quiet", multiChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.4: --check --quiet shows only failures
		{
			Name:      "check-quiet-with-fail",
			Args:      []string{"--check", "--quiet", mixedChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.4: --check --status suppresses all output, exit 0
		{
			Name:      "check-status-ok",
			Args:      []string{"--check", "--status", validChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.4: --check --status suppresses all output, exit 1 on failure
		{
			Name:      "check-status-fail",
			Args:      []string{"--check", "--status", mismatchChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.1: --check with short flag -c
		{
			Name:      "check-short-flag",
			Args:      []string{"-c", validChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.1: binary mode with multiple files
		{
			Name: "binary-multiple-files",
			Args: []string{"-b", helloFile, emptyFile},
		},
		// R3.1: binary mode with stdin
		{
			Name:  "binary-stdin",
			Args:  []string{"-b"},
			Stdin: []byte("abc"),
		},
		// R3.1: long form --binary
		{
			Name: "binary-long-flag",
			Args: []string{"--binary", helloFile},
		},
		// R3.2: text mode with multiple files
		{
			Name: "text-multiple-files",
			Args: []string{"-t", helloFile, emptyFile},
		},
		// R3.3: tag format ignores binary flag, multiple files
		{
			Name: "tag-binary-multiple",
			Args: []string{"--tag", "-b", helloFile, emptyFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func md5hex(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
