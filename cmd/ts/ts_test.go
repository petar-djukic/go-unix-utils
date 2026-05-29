// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var subsecRe = regexp.MustCompile(`\d+\.\d{6}`)

var subsecondNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return subsecRe.ReplaceAll(b, []byte("<SUBSEC>"))
}

var elapsedRe = regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)

var elapsedNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return elapsedRe.ReplaceAll(b, []byte("<ELAPSED>"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ts")
	if err != nil {
		t.Skip("reference binary ts not found")
	}
	norm := []testutils.NormalizeFunc{testutils.TimestampNormalizer}
	subsecNorm := []testutils.NormalizeFunc{subsecondNormalizer}
	elapsedNorm := []testutils.NormalizeFunc{elapsedNormalizer}
	tests := []testutils.DiffTest{
		{Name: "default_multiline", Stdin: []byte("alpha\nbeta\ngamma\n"), Normalize: norm},
		{Name: "single_line", Stdin: []byte("hello\n"), Normalize: norm},
		{Name: "partial_last_line", Stdin: []byte("complete\npartial"), Normalize: norm},
		{Name: "empty_stdin", Stdin: []byte{}},
		{Name: "custom_format_iso", Args: []string{"%Y-%m-%d %H:%M:%S"}, Stdin: []byte("test\n"), Normalize: norm},
		{Name: "custom_format_time", Args: []string{"%H:%M:%S"}, Stdin: []byte("test\n"), Normalize: norm},
		{Name: "custom_format_T", Args: []string{"%T"}, Stdin: []byte("test\n"), Normalize: norm},
		{Name: "subsecond_S", Args: []string{"%.S"}, Stdin: []byte("test\n"), Normalize: subsecNorm},
		{Name: "subsecond_s", Args: []string{"%.s"}, Stdin: []byte("test\n"), Normalize: subsecNorm},
		{Name: "subsecond_T", Args: []string{"%.T"}, Stdin: []byte("test\n"), Normalize: subsecNorm},
		{Name: "incremental_default", Args: []string{"-i"}, Stdin: []byte("a\nb\nc\n"), Normalize: elapsedNorm},
		{Name: "incremental_custom_format", Args: []string{"-i", "%.S"}, Stdin: []byte("x\ny\n"), Normalize: subsecNorm},
		{Name: "sincestart_default", Args: []string{"-s"}, Stdin: []byte("a\nb\nc\n"), Normalize: elapsedNorm},
		{Name: "sincestart_custom_format", Args: []string{"-s", "%.S"}, Stdin: []byte("x\ny\n"), Normalize: subsecNorm},
		{Name: "sincestart_empty_stdin", Args: []string{"-s"}, Stdin: []byte{}},
		{Name: "sincestart_custom_T", Args: []string{"-s", "%T"}, Stdin: []byte("a\nb\n"), Normalize: elapsedNorm},
		{Name: "monotonic_default", Args: []string{"-m"}, Stdin: []byte("a\nb\n"), Normalize: norm},
		{Name: "monotonic_incremental", Args: []string{"-m", "-i"}, Stdin: []byte("a\nb\nc\n"), Normalize: elapsedNorm},
		{Name: "monotonic_sincestart", Args: []string{"-m", "-s"}, Stdin: []byte("a\nb\nc\n"), Normalize: elapsedNorm},
		{Name: "monotonic_custom_format", Args: []string{"-m", "%.S"}, Stdin: []byte("test\n"), Normalize: subsecNorm},
		{Name: "monotonic_sincestart_custom", Args: []string{"-m", "-s", "%.S"}, Stdin: []byte("x\ny\n"), Normalize: subsecNorm},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestMutualExclusive(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "-i", "-s")
	cmd.Stdin = strings.NewReader("x\n")
	stderr, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit when -i and -s are both given")
	}
	if !strings.Contains(string(stderr), "usage") {
		t.Fatalf("expected usage message on stderr, got: %s", stderr)
	}
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
