// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
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
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skip("reference binary gdu not found")
	}

	dir := makeFixture(t)

	tests := []testutils.DiffTest{
		{Name: "valid_dir", Args: []string{dir}},
		{Name: "summary_valid", Args: []string{"-s", dir}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestErrorExit(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	badPath := filepath.Join(t.TempDir(), "nonexistent")

	cmd := exec.Command(goBin, badPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got: %v", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("nonexistent")) {
		t.Fatalf("stderr should mention nonexistent, got %q", stderr.String())
	}
}

func TestErrorContinue(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := makeFixture(t)
	badPath := filepath.Join(dir, "nonexistent")

	cmd := exec.Command(goBin, "-s", badPath, dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected stdout output for valid directory after error")
	}
	if !bytes.Contains(stdout.Bytes(), []byte(dir)) {
		t.Fatalf("stdout should contain valid dir path %q, got %q", dir, stdout.String())
	}
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := makeLargeFixture(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "-a", dir)
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
		t.Fatal("du timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func makeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeLargeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for i := range 50 {
		sub := filepath.Join(dir, fmt.Sprintf("dir%03d", i))
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		for j := range 5 {
			name := filepath.Join(sub, fmt.Sprintf("f%d.txt", j))
			if err := os.WriteFile(name, []byte("data\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}
