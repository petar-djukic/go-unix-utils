// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []struct {
		name  string
		args  []string
		lines int
	}{
		{"default-y", nil, 5},
		{"single-arg", []string{"hello"}, 5},
		{"multi-args", []string{"hello", "world"}, 5},
		{"dash-dash-arg", []string{"--", "--help"}, 3},
		{"dash-dash-only", []string{"--"}, 3},
		{"empty-string", []string{""}, 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			goOut := captureLines(t, goBin, tc.args, tc.lines)
			refOut := captureLines(t, refBin, tc.args, tc.lines)
			if !bytes.Equal(goOut, refOut) {
				t.Errorf("output mismatch\ngo:  %q\nref: %q", goOut, refOut)
			}
		})
	}
}

func TestExitCodeOnSIGPIPE(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		args []string
	}{
		{"default", nil},
		{"with-arg", []string{"hello"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			goCode, goStderr := captureExitAndStderr(t, goBin, tc.args)
			if goCode != 0 {
				t.Errorf("expected exit code 0 on broken pipe, got %d", goCode)
			}
			if len(goStderr) > 0 {
				t.Errorf("unexpected stderr on SIGPIPE: %q", goStderr)
			}
		})
	}
}

func captureExitAndStderr(t *testing.T, binary string, args []string) (int, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		_ = scanner.Text()
	}
	stdout.Close()

	err = cmd.Wait()
	if err == nil {
		return 0, stderrBuf.Bytes()
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stderrBuf.Bytes()
	}
	t.Fatalf("unexpected wait error: %v", err)
	return -1, nil
}

func captureLines(t *testing.T, binary string, args []string, n int) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var buf bytes.Buffer
	scanner := bufio.NewScanner(stdout)
	for i := 0; i < n && scanner.Scan(); i++ {
		buf.Write(scanner.Bytes())
		buf.WriteByte('\n')
	}

	stdout.Close()
	_ = cmd.Wait()
	return buf.Bytes()
}
