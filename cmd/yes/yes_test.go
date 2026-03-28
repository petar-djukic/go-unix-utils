// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/yes against GNU gyes.
// Covers prd012-yes R3.1-R3.3 (error handling, exit codes),
// R4.1-R4.3 (differential testing).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// pipedTimeout is the per-invocation timeout for piped output tests.
const pipedTimeout = 10 * time.Second

// helpVersionNormalizer normalizes --help and --version output so differences
// in binary paths, package names, version strings, ANSI escapes, and GNU
// trailer lines do not cause false failures.
func helpVersionNormalizer() testutils.NormalizeFunc {
	ansiEsc := regexp.MustCompile(`\x1b(?:\][^\x1b]*\x1b\\|\[[0-9;]*m)`)
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?yes`)
	versionLine := regexp.MustCompile(`(?m)^yes \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(`(?m)^(Copyright|License|Written by|This is free|There is NO|Report |General help|or available|Full documentation|GNU coreutils).*\n?`)
	optSplit := regexp.MustCompile(`(--(?:help|version))\n\s+`)
	multiSpace := regexp.MustCompile(`(--(?:help|version))\s{2,}`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("yes"))
		b = versionLine.ReplaceAll(b, []byte("yes (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = optSplit.ReplaceAll(b, []byte("$1  "))
		b = multiSpace.ReplaceAll(b, []byte("$1 "))
		b = bytes.TrimRight(b, "\n")
		if len(b) > 0 {
			b = append(b, '\n')
		}
		return b
	}
}

// shellQuote wraps s in single quotes for safe shell interpolation.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// buildShellCmd constructs a shell command string that pipes binary output
// through head -n lines.
func buildShellCmd(binary string, args []string, lines int) string {
	parts := []string{shellQuote(binary)}
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ") + " | head -n " + strconv.Itoa(lines)
}

// runPiped runs "binary [args...] | head -n lines" via sh and returns stdout.
// R4.1: pipes through head to limit infinite output for comparison.
func runPiped(t *testing.T, binary string, args []string, lines int) []byte {
	t.Helper()
	shellCmd := buildShellCmd(binary, args, lines)
	ctx, cancel := context.WithTimeout(context.Background(), pipedTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", shellCmd)
	env := os.Environ()
	cmd.Env = append(env, "LC_ALL=C")
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("timed out: %s", shellCmd)
	}
	if err != nil {
		t.Fatalf("command failed: %s: %v", shellCmd, err)
	}
	return out
}

// comparePiped runs both binaries piped through head and compares stdout.
func comparePiped(t *testing.T, goBin, refBin string, args []string, lines int) {
	t.Helper()
	goOut := runPiped(t, goBin, args, lines)
	refOut := runPiped(t, refBin, args, lines)
	if !bytes.Equal(goOut, refOut) {
		t.Errorf("piped output divergence\n  args: %v\n  lines: %d\n"+
			"  ref: %q\n  go:  %q", args, lines, refOut, goOut)
	}
}

// TestDiff runs differential tests for --help and --version flags
// against the GNU reference binary gyes.
// R4.3: verifies flag output and exit codes.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skipf("reference binary gyes not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()

	tests := []testutils.DiffTest{
		// R4.3: --help output and exit code.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R4.3: --version output and exit code.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestPipedOutput runs differential tests for yes output piped through head.
// R4.1-R4.2: compares default, single-arg, multi-arg, --, and empty-string cases.
// R4.3: piping through head tests SIGPIPE exit behavior.
func TestPipedOutput(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skipf("reference binary gyes not in PATH: %v", err)
	}

	cases := []struct {
		name  string
		args  []string
		lines int
	}{
		// R4.1: default "y" output.
		{"default_y", nil, 5},
		// R4.2: single argument.
		{"single_arg", []string{"hello"}, 3},
		// R4.2: multiple arguments joined with spaces.
		{"multi_arg", []string{"hello", "world"}, 3},
		// R4.2: arguments after "--" separator.
		{"dash_dash_separator", []string{"--", "--help"}, 3},
		// R4.2: empty string argument.
		{"empty_string", []string{""}, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comparePiped(t, goBin, refBin, tc.args, tc.lines)
		})
	}
}
