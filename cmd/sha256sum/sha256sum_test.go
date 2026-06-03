// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
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

	helloHash := sha256hex([]byte("hello\n"))
	emptyHash := sha256hex([]byte{})

	validChecksum := filepath.Join(tmpDir, "valid.sha256")
	writeFile(t, validChecksum, fmt.Sprintf("%s  %s\n", helloHash, helloFile))

	multiChecksum := filepath.Join(tmpDir, "multi.sha256")
	writeFile(t, multiChecksum, fmt.Sprintf("%s  %s\n%s  %s\n", helloHash, helloFile, emptyHash, emptyFile))

	mismatchChecksum := filepath.Join(tmpDir, "mismatch.sha256")
	writeFile(t, mismatchChecksum, fmt.Sprintf("%s  %s\n", "0000000000000000000000000000000000000000000000000000000000000000", helloFile))

	mixedChecksum := filepath.Join(tmpDir, "mixed.sha256")
	writeFile(t, mixedChecksum, fmt.Sprintf("%s  %s\n%s  %s\n", helloHash, helloFile, "0000000000000000000000000000000000000000000000000000000000000000", emptyFile))

	malformedChecksum := filepath.Join(tmpDir, "malformed.sha256")
	writeFile(t, malformedChecksum, fmt.Sprintf("this is not a checksum\n%s  %s\n", helloHash, helloFile))

	bsdChecksum := filepath.Join(tmpDir, "bsd.sha256")
	writeFile(t, bsdChecksum, fmt.Sprintf("SHA256 (%s) = %s\n", helloFile, helloHash))

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
		// R2.3: --check --quiet suppresses OK lines
		{
			Name:      "check-quiet-all-ok",
			Args:      []string{"--check", "--quiet", multiChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --quiet shows only failures
		{
			Name:      "check-quiet-with-fail",
			Args:      []string{"--check", "--quiet", mixedChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --status suppresses all output, exit 0
		{
			Name:      "check-status-ok",
			Args:      []string{"--check", "--status", validChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --status suppresses all output, exit 1 on failure
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
		// R3.2: tag format ignores binary flag, multiple files
		{
			Name: "tag-binary-multiple",
			Args: []string{"--tag", "-b", helloFile, emptyFile},
		},
		// R4.1: exit 0 when all files processed successfully
		{
			Name: "exit-0-single-file",
			Args: []string{helloFile},
		},
		// R4.1: exit 0 when all verified digests match
		{
			Name:      "exit-0-check-valid",
			Args:      []string{"--check", validChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: exit 1 when a named file cannot be opened
		{
			Name:      "exit-1-missing-file",
			Args:      []string{filepath.Join(tmpDir, "no-such-file.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: exit 1 when a digest fails verification
		{
			Name:      "exit-1-check-mismatch",
			Args:      []string{"--check", mismatchChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: exit 1 when the checksum file is unreadable
		{
			Name:      "exit-1-check-unreadable",
			Args:      []string{"--check", filepath.Join(tmpDir, "nonexistent.sha256")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	if err := os.WriteFile(largePath, bytes.Repeat([]byte("x\n"), 500000), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, largePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("sha256sum timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
