// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for sponge covering stdin soaking, file write, append
// mode, and passthrough mode.
//
// Implements prd007-sponge R1.1, R2.1, R3.1, R4.1.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the compiled Go sponge binary. Set by TestMain.
var goBinary string

// refBinary is the path to the moreutils sponge reference binary. Set by TestMain.
var refBinary string

// TestMain builds the Go sponge binary and locates the sponge reference binary.
// D1: skip all tests if sponge is not on PATH.
// D2: moreutils binaries have no g- prefix; use exec.LookPath("sponge").
func TestMain(m *testing.M) {
	ref, err := exec.LookPath("sponge")
	if err != nil {
		fmt.Fprintln(os.Stderr, "sponge not found on PATH; skipping sponge differential tests")
		os.Exit(0)
	}
	refBinary = ref

	binDir, err := os.MkdirTemp("", "sponge-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "sponge")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go sponge binary: %v\n%s", err, out)
		os.RemoveAll(binDir) // best-effort cleanup
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(binDir) // best-effort cleanup
	os.Exit(code)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}

// TestSpongePassthrough verifies R4.1: when no file argument is given, sponge
// writes soaked stdin to stdout (passthrough mode).
func TestSpongePassthrough(t *testing.T) {
	t.Parallel()

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "passthrough-stdout",
			Stdin:    []byte("hello world\n"),
			ExitCode: 0,
		},
		{
			Name:     "passthrough-multiline",
			Stdin:    []byte("line1\nline2\nline3\n"),
			ExitCode: 0,
		},
		{
			Name:     "passthrough-empty",
			Stdin:    []byte{},
			ExitCode: 0,
		},
	})
}

// TestSpongeWriteToFile verifies R1.1 and R2.1: sponge reads all of stdin
// before opening the output file and writes to a named output file atomically.
func TestSpongeWriteToFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "small-stdin-to-file",
			Args:     []string{filepath.Join(dir, "out1.txt")},
			Stdin:    []byte("hello\nworld\n"),
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				filepath.Join(dir, "out1.txt"): []byte("hello\nworld\n"),
			},
		},
		{
			Name:     "empty-stdin-to-file",
			Args:     []string{filepath.Join(dir, "empty.txt")},
			Stdin:    []byte{},
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				filepath.Join(dir, "empty.txt"): []byte{},
			},
		},
	})
}

// TestSpongeOverwriteExisting verifies R1.1 and R2.1: sponge overwrites an
// existing file with new stdin content.
func TestSpongeOverwriteExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "existing.txt", "old content\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "overwrite-existing-file",
			Args:     []string{filepath.Join(dir, "existing.txt")},
			Stdin:    []byte("new content\n"),
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				filepath.Join(dir, "existing.txt"): []byte("new content\n"),
			},
		},
	})
}

// TestSpongeAppendMode verifies R3.1: sponge -a appends stdin to an existing
// file rather than truncating it. The result is [original][stdin].
func TestSpongeAppendMode(t *testing.T) {
	t.Parallel()

	t.Run("append-to-existing", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeTestFile(t, dir, "existing.txt", "original line\n")

		testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
			{
				Name:     "append-mode",
				Args:     []string{"-a", filepath.Join(dir, "existing.txt")},
				Stdin:    []byte("appended line\n"),
				ExitCode: 0,
				ExpectedFiles: map[string][]byte{
					filepath.Join(dir, "existing.txt"): []byte("original line\nappended line\n"),
				},
			},
		})
	})

	t.Run("append-no-existing-file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
			{
				Name:     "append-new-file",
				Args:     []string{"-a", filepath.Join(dir, "newfile.txt")},
				Stdin:    []byte("new content\n"),
				ExitCode: 0,
				ExpectedFiles: map[string][]byte{
					filepath.Join(dir, "newfile.txt"): []byte("new content\n"),
				},
			},
		})
	})
}

// TestSpongeSoakBeforeWrite verifies R1.1: sponge reads all stdin before
// writing. We simulate 'cat file | sponge file' by providing file content as
// stdin and the same file as the output argument.
func TestSpongeSoakBeforeWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "line1\nline2\nline3\n"
	writeTestFile(t, dir, "data.txt", content)

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "soak-before-write",
			Args:     []string{filepath.Join(dir, "data.txt")},
			Stdin:    []byte(content),
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				filepath.Join(dir, "data.txt"): []byte(content),
			},
		},
	})
}

// TestSpongeLargeStdin verifies R1.1 with a larger input to exercise the
// buffer growth path. We use a payload over 1 MB.
func TestSpongeLargeStdin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Build a payload > 1 MB by repeating lines.
	var sb strings.Builder
	for i := 1; i <= 50000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	largeInput := []byte(sb.String())

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "large-stdin-to-file",
			Args:     []string{filepath.Join(dir, "large_out.txt")},
			Stdin:    largeInput,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				filepath.Join(dir, "large_out.txt"): largeInput,
			},
		},
	})
}
