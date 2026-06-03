// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"io"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpathchk")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?pathchk`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("pathchk"))
	})

	tests := []testutils.DiffTest{
		{
			Name: "valid-path",
			Args: []string{"validpath"},
		},
		{
			Name: "valid-absolute-path",
			Args: []string{"/tmp"},
		},
		{
			Name: "valid-nested-path",
			Args: []string{"/tmp/foo/bar"},
		},
		{
			Name: "posix-invalid-at",
			Args:      []string{"-p", "invalid@path"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name: "posix-valid",
			Args: []string{"-p", "valid_path"},
		},
		{
			Name: "posix-valid-dotfile",
			Args: []string{"-p", ".hidden"},
		},
		{
			Name:      "posix-component-too-long",
			Args:      []string{"-p", "aaaaaaaaaaaaaaa"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name: "posix-component-at-limit",
			Args: []string{"-p", "aaaaaaaaaaaaaa"},
		},
		{
			Name:      "lead-hyphen",
			Args:      []string{"-P", "--", "-file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name: "lead-hyphen-ok",
			Args: []string{"-P", "normal"},
		},
		{
			Name:      "lead-hyphen-in-component",
			Args:      []string{"-P", "dir/-file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "empty-name-P",
			Args:      []string{"-P", ""},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "empty-name-default",
			Args:      []string{""},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "empty-name-posix",
			Args:      []string{"-p", ""},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "missing-operand",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name: "multiple-valid",
			Args: []string{"foo", "bar", "baz"},
		},
		{
			Name:      "multiple-mixed",
			Args:      []string{"-p", "valid", "invalid@path"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "portability-flag",
			Args:      []string{"--portability", "valid"},
		},
		{
			Name:      "posix-space-in-name",
			Args:      []string{"-p", "a b"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "help",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		{
			Name:      "version",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "--help")
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
		t.Fatal("pathchk timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}
