// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd113-pee R1.1, R1.2, R1.3, R2.1, R2.2.
package main

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func sortLines(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	sort.Slice(lines, func(i, j int) bool {
		return bytes.Compare(lines[i], lines[j]) < 0
	})
	return bytes.Join(lines, []byte("\n"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("pee")
	if err != nil {
		t.Skip("reference binary pee not found")
	}
	tests := []testutils.DiffTest{
		{
			Name:  "no_args",
			Args:  []string{},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "single_cat",
			Args:  []string{"cat"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:      "cat_and_wc",
			Args:      []string{"cat", "wc -c"},
			Stdin:     []byte("hello\n"),
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "three_commands",
			Args:      []string{"cat", "cat", "cat"},
			Stdin:     []byte("abc\n"),
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:     "failing_command",
			Args:     []string{"false"},
			Stdin:    []byte("hello\n"),
			ExitCode: 1,
		},
		{
			Name:      "mixed_exit_codes",
			Args:      []string{"true", "false"},
			Stdin:     []byte("hello\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:  "empty_stdin",
			Args:  []string{"cat"},
			Stdin: []byte{},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "head -c 1000000 /dev/zero")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = bytes.NewReader(make([]byte, 2000000))
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
		t.Fatal("pee timed out; SIGPIPE handler may not be installed")
	}
}
