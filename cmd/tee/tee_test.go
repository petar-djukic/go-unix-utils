// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee against GNU gtee.
// Covers prd017-tee R4.1-R4.3 (differential testing).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// helpVersionNormalizer normalizes --help and --version output so differences
// in binary paths, package names, version strings, and GNU trailer lines
// do not cause false failures.
func helpVersionNormalizer() testutils.NormalizeFunc {
	ansiEsc := regexp.MustCompile(`\x1b(?:\][^\x1b]*\x1b\\|\[[0-9;]*m)`)
	binPath := regexp.MustCompile(`(?m)/[^\s]+/g?tee`)
	versionLine := regexp.MustCompile(`(?m)^tee \([^)]+\) .+$`)
	gnuTrailer := regexp.MustCompile(
		`(?m)^(Copyright|License|Written by|This is free|` +
			`There is NO|Report |General help|or available|` +
			`Full documentation|GNU coreutils).*\n?`)
	// Strip all lines related to GNU-only features (-p, --output-error,
	// MODE descriptions) and explanatory paragraphs.
	gnuOnly := regexp.MustCompile(
		`(?m)^.*(-p\b|--output-error|MODE|` +
			`operate in|write error|nopipe|` +
			`If a FILE|With no FILE|The default|` +
			`With "nopipe"|writing to).*\n?`)
	// Collapse whitespace differences in option descriptions.
	optWrap := regexp.MustCompile(
		`(--(?:help|version|append|ignore-interrupts))\n\s+`)
	optSpace := regexp.MustCompile(
		`(--(?:help|version|append|ignore-interrupts))\s{2,}`)
	multiBlank := regexp.MustCompile(`\n{2,}`)
	return func(b []byte) []byte {
		b = ansiEsc.ReplaceAll(b, nil)
		b = binPath.ReplaceAll(b, []byte("tee"))
		b = versionLine.ReplaceAll(b, []byte("tee (NORMALIZED) VERSION"))
		b = gnuTrailer.ReplaceAll(b, nil)
		b = gnuOnly.ReplaceAll(b, nil)
		b = optWrap.ReplaceAll(b, []byte("$1 "))
		b = optSpace.ReplaceAll(b, []byte("$1 "))
		b = multiBlank.ReplaceAll(b, []byte("\n"))
		b = bytes.TrimRight(b, "\n")
		if len(b) > 0 {
			b = append(b, '\n')
		}
		return b
	}
}

// stderrNormalizer normalizes error messages between GNU and Go binaries.
func stderrNormalizer() testutils.NormalizeFunc {
	// GNU: "tee: /path: reason" vs Go: "tee: open /path: reason"
	binPath := regexp.MustCompile(`/[^\s:]+/g?tee|gtee`)
	goOpen := regexp.MustCompile(`tee: open `)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	// Normalize "No such file or directory" casing.
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("tee"))
		b = goOpen.ReplaceAll(b, []byte("tee: "))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	norm := helpVersionNormalizer()
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R4.2: passthrough — stdin to stdout only, no files.
		{
			Name:  "passthrough_no_files",
			Stdin: []byte("hello\nworld\n"),
		},
		// R4.2: single file — stdin to stdout and one file.
		{
			Name:  "single_file",
			Args:  []string{"out.txt"},
			Stdin: []byte("hello\nworld\n"),
			ExpectedFiles: map[string][]byte{
				"out.txt": []byte("hello\nworld\n"),
			},
		},
		// R4.2: multiple files — stdin to stdout and two files.
		{
			Name:  "multiple_files",
			Args:  []string{"a.txt", "b.txt"},
			Stdin: []byte("line1\nline2\n"),
			ExpectedFiles: map[string][]byte{
				"a.txt": []byte("line1\nline2\n"),
				"b.txt": []byte("line1\nline2\n"),
			},
		},
		// R4.3: empty stdin — should produce empty stdout and file.
		{
			Name:  "empty_stdin",
			Args:  []string{"empty.txt"},
			Stdin: []byte{},
			ExpectedFiles: map[string][]byte{
				"empty.txt": {},
			},
		},
		// R4.2: -i flag — ignore interrupts, still copies normally.
		{
			Name:  "ignore_interrupts_flag",
			Args:  []string{"-i", "out.txt"},
			Stdin: []byte("data\n"),
			ExpectedFiles: map[string][]byte{
				"out.txt": []byte("data\n"),
			},
		},
		// R4.3: write error on invalid path — exit 1 with stderr.
		{
			Name:      "write_error_invalid_path",
			Args:      []string{"/nonexistent-dir/nofile.txt"},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: --help output.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R4.3: --version output.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R4.2: -- separator before filenames.
		{
			Name:  "double_dash_separator",
			Args:  []string{"--", "out.txt"},
			Stdin: []byte("separated\n"),
			ExpectedFiles: map[string][]byte{
				"out.txt": []byte("separated\n"),
			},
		},
		// R4.2: binary data passthrough.
		{
			Name:  "binary_data",
			Args:  []string{"bin.dat"},
			Stdin: []byte{0x00, 0x01, 0xFF, 0xFE, 0x0A, 0x0D},
			ExpectedFiles: map[string][]byte{
				"bin.dat": {0x00, 0x01, 0xFF, 0xFE, 0x0A, 0x0D},
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAppendMode tests append mode separately because it requires
// pre-existing file content before tee runs. Both binaries run in
// isolated directories to avoid shared-workdir interference.
func TestDiffAppendMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	// R4.2: append mode with pre-existing file (-a short form).
	t.Run("append_to_existing", func(t *testing.T) {
		verifyAppend(t, goBin, refBin, []string{"-a"})
	})

	// R4.2: long form --append.
	t.Run("append_long_form", func(t *testing.T) {
		verifyAppend(t, goBin, refBin, []string{"--append"})
	})

	// R4.2: combined -ai flags with append on pre-existing file.
	t.Run("combined_ai_flags", func(t *testing.T) {
		verifyAppend(t, goBin, refBin, []string{"-ai"})
	})
}

// verifyAppend runs both binaries with append mode on a pre-seeded file
// and compares the resulting file content and stdout.
func verifyAppend(
	t *testing.T, goBin, refBin string, flags []string,
) {
	t.Helper()
	stdin := []byte("new\n")
	existing := []byte("old\n")

	refDir := t.TempDir()
	goDir := t.TempDir()

	seedFile(t, refDir, "out.txt", existing)
	seedFile(t, goDir, "out.txt", existing)

	args := append(flags, "out.txt")

	refOut := runCapture(t, refBin, args, stdin, refDir)
	goOut := runCapture(t, goBin, args, stdin, goDir)

	if !bytes.Equal(refOut.stdout, goOut.stdout) {
		t.Errorf("stdout divergence\n  ref: %q\n  go:  %q",
			refOut.stdout, goOut.stdout)
	}
	if refOut.exitCode != goOut.exitCode {
		t.Errorf("exit code divergence  ref=%d  go=%d",
			refOut.exitCode, goOut.exitCode)
	}

	refFile := readFileBytes(t, filepath.Join(refDir, "out.txt"))
	goFile := readFileBytes(t, filepath.Join(goDir, "out.txt"))

	// R4.3: file content must match byte-for-byte between binaries.
	if !bytes.Equal(refFile, goFile) {
		t.Errorf("file content divergence\n  ref: %q\n  go:  %q",
			refFile, goFile)
	}

	// Verify append actually appended.
	want := append(existing, stdin...)
	if !bytes.Equal(goFile, want) {
		t.Errorf("append failed\n  want: %q\n  got:  %q",
			want, goFile)
	}
}

// captureResult holds output from a binary invocation.
type captureResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runCapture executes a binary and captures stdout, stderr, exit code.
func runCapture(
	t *testing.T, bin string, args []string,
	stdin []byte, workDir string,
) captureResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}

	return captureResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// seedFile creates a file with the given content in dir.
func seedFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("seed file %s: %v", path, err)
	}
}

// readFileBytes reads a file and returns its content.
func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return data
}
