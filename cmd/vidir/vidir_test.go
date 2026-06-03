// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd114-vidir R2.1, R2.2.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func testEnv(overrides ...string) []string {
	skip := make(map[string]bool)
	for _, e := range overrides {
		k, _, _ := strings.Cut(e, "=")
		skip[k] = true
	}
	var env []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if !skip[k] {
			env = append(env, e)
		}
	}
	return append(env, overrides...)
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("vidir")
	if err != nil {
		t.Skip("reference binary vidir not found")
	}
	tests := []testutils.DiffTest{
		{
			Name:  "empty_stdin",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"EDITOR=true"},
		},
		{
			Name:  "editor_noop",
			Args:  []string{},
			Stdin: []byte("a\n"),
			Env:   []string{"EDITOR=true"},
		},
		{
			Name:  "multiple_files_noop",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"EDITOR=true"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestEditorFailure(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin)
	cmd.Stdin = bytes.NewReader([]byte("a\n"))
	cmd.Env = testEnv("EDITOR=false", "LC_ALL=C")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1 for editor failure, got: %v", err)
	}
}

func TestDeleteFailure(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	scriptDir := t.TempDir()
	truncScript := filepath.Join(scriptDir, "trunc.sh")
	if err := os.WriteFile(truncScript, []byte("#!/bin/sh\n: > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goBin)
	cmd.Stdin = bytes.NewReader([]byte("nonexistent.txt\n"))
	cmd.Env = testEnv("EDITOR="+truncScript, "LC_ALL=C")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1 for delete failure, got: %v", err)
	}
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = bytes.NewReader([]byte("a\nb\nc\n"))
	cmd.Env = testEnv("EDITOR=cat", "LC_ALL=C")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	_ = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("vidir timed out; SIGPIPE handler may not be installed")
	}
}
