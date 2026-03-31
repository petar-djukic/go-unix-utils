// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/yes against gyes (GNU coreutils).
//
// Covers prd012-yes R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2, R3.3,
// R4.1, R4.2, R4.3.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// pipeHeadTimeout is the timeout for pipe-through-head tests.
const pipeHeadTimeout = 10 * time.Second

// discardStdout blanks stdout so that tests comparing --help or --version
// check only exit code and stderr. GNU yes's output includes paths and
// boilerplate that cannot be reproduced exactly.
func discardStdout(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skip("reference binary gyes not in PATH")
	}

	// --help and --version use RunDiffTests with discardStdout.
	tests := []testutils.DiffTest{
		// R1.3: --help prints usage and exits 0
		{
			Name:      "R1.3_help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
		// R2.1: --version prints version info and exits 0
		{
			Name:      "R2.1_version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStdout},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Differential output tests pipe through head to capture finite output.
	// These cases match GNU yes behavior byte-for-byte.
	diffOutputTests := []struct {
		name  string
		args  []string
		lines int
		want  string
	}{
		// R1.1: no arguments — outputs "y" lines
		{
			name:  "R1.1_default_y",
			args:  nil,
			lines: 5,
			want:  "y\ny\ny\ny\ny\n",
		},
		// R1.2: single argument
		{
			name:  "R1.2_single_arg",
			args:  []string{"hello"},
			lines: 3,
			want:  "hello\nhello\nhello\n",
		},
		// R1.2: multiple arguments joined by spaces
		{
			name:  "R1.2_multi_arg",
			args:  []string{"hello", "world"},
			lines: 3,
			want:  "hello world\nhello world\nhello world\n",
		},
		// R4.2: empty string argument outputs empty lines
		{
			name:  "R4.2_empty_string",
			args:  []string{""},
			lines: 3,
			want:  "\n\n\n",
		},
		// R4.2: multiple arguments with empty strings preserving positions
		{
			name:  "R4.2_multi_with_empty",
			args:  []string{"", "hello", ""},
			lines: 3,
			want:  " hello \n hello \n hello \n",
		},
		// R4.3: argument containing tab character
		{
			name:  "R4.3_tab_char",
			args:  []string{"hello\tworld"},
			lines: 3,
			want:  "hello\tworld\nhello\tworld\nhello\tworld\n",
		},
		// R4.3: argument containing quotes
		{
			name:  "R4.3_quotes",
			args:  []string{`he said "hi"`},
			lines: 3,
			want:  "he said \"hi\"\nhe said \"hi\"\nhe said \"hi\"\n",
		},
	}

	for _, tc := range diffOutputTests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			goOut := runPipedHead(t, goBin, tc.args, tc.lines)
			refOut := runPipedHead(t, refBin, tc.args, tc.lines)
			if !bytes.Equal(goOut, refOut) {
				t.Errorf("output mismatch args=%v\n  ref: %q\n  go:  %q",
					tc.args, refOut, goOut)
			}
			if string(goOut) != tc.want {
				t.Errorf("unexpected output args=%v\n  want: %q\n  got:  %q",
					tc.args, tc.want, goOut)
			}
		})
	}
}

// TestUnrecognizedFlags verifies that unrecognized options are output as
// literal strings rather than causing an error (prd012 R3.3, D1).
// GNU yes uses getopt and rejects unknown flags, but our implementation
// intentionally treats all non --help/--version arguments as output strings.
func TestUnrecognizedFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	flagTests := []struct {
		name string
		args []string
		want string
	}{
		// R3.3: long unrecognized option treated as literal string
		{
			name: "R3.3_unrecognized_long_flag",
			args: []string{"--foo"},
			want: "--foo\n--foo\n--foo\n",
		},
		// R3.3: short unrecognized option treated as literal string
		{
			name: "R3.3_unrecognized_short_flag",
			args: []string{"-x"},
			want: "-x\n-x\n-x\n",
		},
		// R3.3: multiple unrecognized options joined with spaces
		{
			name: "R3.3_multi_flags",
			args: []string{"-a", "-b"},
			want: "-a -b\n-a -b\n-a -b\n",
		},
	}

	for _, tc := range flagTests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runPipedHead(t, goBin, tc.args, 3)
			if string(got) != tc.want {
				t.Errorf("unexpected output args=%v\n  want: %q\n  got:  %q",
					tc.args, tc.want, got)
			}
		})
	}
}

// TestVersionFormat verifies the --version output matches the expected pattern.
// R2.2: version output format "yes (go-unix-utils) VERSION".
func TestVersionFormat(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}

	// Version output must match "yes (go-unix-utils) <version>\n".
	versionRe := regexp.MustCompile(`^yes \(go-unix-utils\) .+\n$`)
	if !versionRe.Match(out) {
		t.Errorf("version output does not match pattern: %q", out)
	}
}

// TestSIGPIPEExitZero verifies that yes exits 0 when SIGPIPE is received.
// R3.1, R4.3: exit code is 0 on normal termination (SIGPIPE).
func TestSIGPIPEExitZero(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), pipeHeadTimeout)
	defer cancel()

	yesCmd := exec.CommandContext(ctx, goBin)
	yesCmd.Env = append(yesCmd.Environ(), "LC_ALL=C")

	headCmd := exec.CommandContext(ctx, "head", "-n", "1")

	pipe, err := yesCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	headCmd.Stdin = pipe

	var out bytes.Buffer
	headCmd.Stdout = &out

	// Capture yes's stderr to verify it writes nothing (R3.2, R3.3).
	var yesStderr bytes.Buffer
	yesCmd.Stderr = &yesStderr

	if err := yesCmd.Start(); err != nil {
		t.Fatalf("start yes: %v", err)
	}
	if err := headCmd.Start(); err != nil {
		t.Fatalf("start head: %v", err)
	}

	// Wait for head to finish, then close the pipe read end so yes gets SIGPIPE.
	if err := headCmd.Wait(); err != nil {
		t.Fatalf("head wait: %v", err)
	}
	pipe.Close() // triggers SIGPIPE on yes's next write

	// R3.1: yes should exit 0 on SIGPIPE.
	err = yesCmd.Wait()
	if err != nil {
		t.Errorf("yes should exit 0 on SIGPIPE, got: %v", err)
	}

	// R3.2, R3.3: yes must not write to stderr on write failure.
	if yesStderr.Len() > 0 {
		t.Errorf("yes wrote to stderr on SIGPIPE: %q", yesStderr.String())
	}
}

// runPipedHead runs binary with args, piping stdout through head -n N.
// Returns the captured output from head.
func runPipedHead(t *testing.T, bin string, args []string, lines int) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), pipeHeadTimeout)
	defer cancel()

	yesCmd := exec.CommandContext(ctx, bin, args...)
	yesCmd.Env = append(yesCmd.Environ(), "LC_ALL=C")

	headCmd := exec.CommandContext(ctx, "head", "-n", headArg(lines))

	pipe, err := yesCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	headCmd.Stdin = pipe

	var out bytes.Buffer
	headCmd.Stdout = &out

	if err := yesCmd.Start(); err != nil {
		t.Fatalf("start %s: %v", bin, err)
	}
	if err := headCmd.Start(); err != nil {
		t.Fatalf("start head: %v", err)
	}

	// Wait for head to finish (it closes stdin when done).
	if err := headCmd.Wait(); err != nil {
		t.Fatalf("head wait: %v", err)
	}
	// yes will get SIGPIPE or be killed by context; ignore its error.
	_ = yesCmd.Wait() // best-effort: yes exits via SIGPIPE

	return out.Bytes()
}

// headArg formats the line count as a string for head -n.
func headArg(n int) string {
	return fmt.Sprintf("%d", n)
}
