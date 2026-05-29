// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skip("reference binary ts not found")
	}
	norm := []testutils.NormalizeFunc{testutils.TimestampNormalizer}
	tests := []testutils.DiffTest{
		{Name: "default_multiline", Stdin: []byte("alpha\nbeta\ngamma\n"), Normalize: norm},
		{Name: "single_line", Stdin: []byte("hello\n"), Normalize: norm},
		{Name: "partial_last_line", Stdin: []byte("complete\npartial"), Normalize: norm},
		{Name: "empty_stdin", Stdin: []byte{}},
		{Name: "custom_format_iso", Args: []string{"%Y-%m-%d %H:%M:%S"}, Stdin: []byte("test\n"), Normalize: norm},
		{Name: "custom_format_time", Args: []string{"%H:%M:%S"}, Stdin: []byte("test\n"), Normalize: norm},
		{Name: "custom_format_T", Args: []string{"%T"}, Stdin: []byte("test\n"), Normalize: norm},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer stdin.Close()
		for i := range 500000 {
			if _, err := fmt.Fprintf(stdin, "line %d\n", i); err != nil {
				return
			}
		}
	}()
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("ts timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}
