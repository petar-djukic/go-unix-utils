// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gptx")
	if err != nil {
		t.Skip("reference binary gptx not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "basic kwic index from stdin",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "empty input produces no output",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			Name:  "single word",
			Args:  []string{},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "three words",
			Args:  []string{},
			Stdin: []byte("a b c\n"),
		},

		// R2.1: width and gap-size flags
		{
			Name:  "width short flag -w",
			Args:  []string{"-w", "50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width long flag --width=N",
			Args:  []string{"--width=50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width long flag --width N",
			Args:  []string{"--width", "50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width attached -wN",
			Args:  []string{"-w50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "gap-size short flag -g",
			Args:  []string{"-g", "5"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "gap-size long flag --gap-size=N",
			Args:  []string{"--gap-size=5"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "gap-size attached -gN",
			Args:  []string{"-g5"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width and gap-size combined",
			Args:  []string{"-w", "50", "-g", "5"},
			Stdin: []byte("hello world\n"),
		},

		// R2.2: ignore-case flag
		{
			Name:  "ignore-case short flag -f",
			Args:  []string{"-f"},
			Stdin: []byte("Beta alpha\n"),
		},
		{
			Name:  "ignore-case long flag --ignore-case",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("Beta alpha\n"),
		},
		{
			Name:  "case-sensitive sort (no -f)",
			Args:  []string{},
			Stdin: []byte("Beta alpha\n"),
		},
		{
			Name:  "ignore-case with mixed words",
			Args:  []string{"-f"},
			Stdin: []byte("x Y z\n"),
		},
		{
			Name:  "combined -f and -w flags",
			Args:  []string{"-fw", "50"},
			Stdin: []byte("Beta alpha\n"),
		},

		// R2.3: file and stdin reading
		{
			Name:  "read from explicit stdin dash",
			Args:  []string{"-"},
			Stdin: []byte("hello world\n"),
		},

		// R5.1: exit codes
		{
			Name:  "r5_1 exit 0 on success",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:      "r5_1 exit 1 nonexistent file",
			Args:      []string{filepath.Join(t.TempDir(), "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		{
			Name:      "r5_1 exit 1 invalid option",
			Args:      []string{"--bogus-flag"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin)
	cmd.Stdin = strings.NewReader("hello world\n")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	stdout.Read(buf)
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("ptx timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

var discardStderr = testutils.NormalizeFunc(func([]byte) []byte { return nil })
