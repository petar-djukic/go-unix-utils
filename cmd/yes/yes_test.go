// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skipf("reference binary gyes not in PATH: %v", err)
	}

	// R4.1: Pipe through "head -n N" to capture finite output.
	// Since DiffTest doesn't support pipe_through, we run yes via shell.
	tests := []struct {
		name    string
		args    string
		headN   int
		want    string
	}{
		{"default_output", "", 5, "y\ny\ny\ny\ny\n"},
		{"single_arg", "hello", 3, "hello\nhello\nhello\n"},
		{"multi_arg", "hello world", 2, "hello world\nhello world\n"},
		{"double_dash", "-- --help", 1, "--help\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refOut := runYesWithHead(t, refBin, tc.args, tc.headN)
			goOut := runYesWithHead(t, goBin, tc.args, tc.headN)

			if !bytes.Equal(refOut, goOut) {
				t.Errorf("divergence detected\n"+
					"args:       %s\n"+
					"ref stdout: %q\n"+
					"go  stdout: %q",
					tc.args, refOut, goOut)
			}
		})
	}
}

// runYesWithHead runs the binary with args piped through head -n N via shell.
func runYesWithHead(t *testing.T, binary, args string, headN int) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build shell command: <binary> <args> | head -n <N>
	shellCmd := binary
	if args != "" {
		shellCmd += " " + args
	}
	shellCmd += " | head -n " + strings.Repeat("", 0) + itoa(headN)

	cmd := exec.CommandContext(ctx, "sh", "-c", shellCmd)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	out, err := cmd.Output()
	if err != nil {
		// head closes the pipe, causing yes to get SIGPIPE.
		// This is expected; we only care about the output.
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("timed out running %s", shellCmd)
		}
	}
	return out
}

// itoa converts an int to its string representation.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
