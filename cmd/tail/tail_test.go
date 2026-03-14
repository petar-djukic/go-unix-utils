// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd055-tail R4.3 -- differential tests against gtail reference binary
// R1.1–R1.4, R2.1–R2.3, R3.1–R3.4, R4.1–R4.4
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

// stderrNormalizer normalizes the binary name prefix (gtail: vs tail:) and
// platform-specific error message casing differences in stderr output.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "gtail:", "tail:")
	// macOS Go returns lowercase "no such file or directory" while GNU uses uppercase.
	s = strings.ReplaceAll(s, "No such file or directory", "no such file or directory")
	// Normalize directory path differences in multi-file output.
	// GNU outputs "error reading '...'" and our code matches this format.
	return []byte(s)
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skipf("reference binary gtail not in PATH: %v", err)
	}

	// Create temp files for file-based tests.
	tmpDir := t.TempDir()

	// 20-line file for line tests.
	lines20 := ""
	for i := 1; i <= 20; i++ {
		lines20 += fmt.Sprintf("%d\n", i)
	}
	file20 := filepath.Join(tmpDir, "20lines.txt")
	if err := os.WriteFile(file20, []byte(lines20), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// 5-line file for small file tests.
	lines5 := "a\nb\nc\nd\ne\n"
	file5 := filepath.Join(tmpDir, "5lines.txt")
	if err := os.WriteFile(file5, []byte(lines5), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// Binary content file for byte tests.
	byteContent := "abcdefghijklmnopqrstuvwxyz"
	fileBytes := filepath.Join(tmpDir, "bytes.txt")
	if err := os.WriteFile(fileBytes, []byte(byteContent), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// R2.3: larger file for suffix multiplier tests (2048 bytes).
	largeBuf := make([]byte, 2048)
	for i := range largeBuf {
		largeBuf[i] = byte('A' + (i % 26))
	}
	fileLarge := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(fileLarge, largeBuf, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// Second file for multi-file tests.
	file2 := filepath.Join(tmpDir, "second.txt")
	if err := os.WriteFile(file2, []byte("x\ny\nz\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// Non-existent file path.
	nonExistent := filepath.Join(tmpDir, "no-such-file.txt")

	tests := []testutils.DiffTest{
		// R1.1: default 10 lines from stdin
		{
			Name:  "default-10-lines-stdin",
			Stdin: []byte(lines20),
		},
		// R1.2: explicit -n count
		{
			Name:  "n-5-lines-stdin",
			Args:  []string{"-n", "5"},
			Stdin: []byte(lines20),
		},
		// R1.2: -nN form (no space)
		{
			Name:  "n5-lines-stdin-no-space",
			Args:  []string{"-n5"},
			Stdin: []byte(lines20),
		},
		// R1.3: +N from start
		{
			Name:  "n-plus-3-stdin",
			Args:  []string{"-n", "+3"},
			Stdin: []byte(lines20),
		},
		// R1.3: +N with no space
		{
			Name:  "n-plus-5-stdin-no-space",
			Args:  []string{"-n+5"},
			Stdin: []byte(lines20),
		},
		// R1.1: default from file
		{
			Name: "default-10-lines-file",
			Args: []string{file20},
		},
		// R1.2: -n from file
		{
			Name: "n-5-lines-file",
			Args: []string{"-n", "5", file20},
		},
		// R1.3: +N from file
		{
			Name: "n-plus-3-file",
			Args: []string{"-n", "+3", file20},
		},
		// R1.1: file with fewer than 10 lines
		{
			Name: "fewer-than-10-lines",
			Args: []string{file5},
		},
		// R2.1: -c byte count
		{
			Name: "c-5-bytes-file",
			Args: []string{"-c", "5", fileBytes},
		},
		// R2.1: -cN form (no space)
		{
			Name: "c5-bytes-file-no-space",
			Args: []string{"-c5", fileBytes},
		},
		// R2.2: -c +N from start
		{
			Name: "c-plus-10-bytes-file",
			Args: []string{"-c", "+10", fileBytes},
		},
		// R2.1: -c from stdin
		{
			Name:  "c-5-bytes-stdin",
			Args:  []string{"-c", "5"},
			Stdin: []byte(byteContent),
		},
		// R2.2: -c +N from stdin
		{
			Name:  "c-plus-10-bytes-stdin",
			Args:  []string{"-c", "+10"},
			Stdin: []byte(byteContent),
		},
		// R3.1: multi-file headers
		{
			Name: "multi-file-headers",
			Args: []string{file20, file5},
		},
		// R3.1: multi-file with -n
		{
			Name: "multi-file-n-3",
			Args: []string{"-n", "3", file20, file5},
		},
		// R3.3: -q suppresses headers
		{
			Name: "quiet-multi-file",
			Args: []string{"-q", file20, file5},
		},
		// R3.4: -v forces header on single file
		{
			Name: "verbose-single-file",
			Args: []string{"-v", file20},
		},
		// R3.2: single file no header
		{
			Name: "single-file-no-header",
			Args: []string{file20},
		},
		// R4.2, R4.4: non-existent file
		{
			Name:      "nonexistent-file",
			Args:      []string{nonExistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.4: mix of valid and invalid files
		{
			Name:      "valid-and-invalid-files",
			Args:      []string{file5, nonExistent, file20},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.4: stdin via -
		{
			Name:  "stdin-via-dash",
			Args:  []string{"-"},
			Stdin: []byte(lines5),
		},
		// Edge: -n 0 (no output)
		{
			Name:  "n-0-no-output",
			Args:  []string{"-n", "0"},
			Stdin: []byte(lines20),
		},
		// Edge: -c 0 (no output)
		{
			Name:  "c-0-no-output",
			Args:  []string{"-c", "0"},
			Stdin: []byte(byteContent),
		},
		// Edge: empty stdin
		{
			Name:  "empty-stdin",
			Stdin: []byte(""),
		},
		// Edge: single line no trailing newline
		{
			Name:  "single-line-no-newline",
			Stdin: []byte("hello"),
		},
		// R1.3: +1 means output everything
		{
			Name:  "n-plus-1-all",
			Args:  []string{"-n", "+1"},
			Stdin: []byte(lines5),
		},
		// R2.2: +1 byte means output everything
		{
			Name:  "c-plus-1-all",
			Args:  []string{"-c", "+1"},
			Stdin: []byte(byteContent),
		},
		// R2.1: -c with --bytes= long form
		{
			Name: "bytes-long-form",
			Args: []string{"--bytes=5", fileBytes},
		},
		// R2.2: --bytes=+N from beginning
		{
			Name: "bytes-long-form-plus",
			Args: []string{"--bytes=+10", fileBytes},
		},
		// R2.2: -c +N with larger offset than file size
		{
			Name: "c-plus-beyond-end",
			Args: []string{"-c", "+100", fileBytes},
		},
		// R2.2: -n +N with offset beyond file line count
		{
			Name:  "n-plus-beyond-end",
			Args:  []string{"-n", "+100"},
			Stdin: []byte(lines5),
		},
		// R2.3: -c with suffix b (512 bytes)
		{
			Name:  "c-suffix-b",
			Args:  []string{"-c", "1b"},
			Stdin: []byte(largeBuf),
		},
		// R2.3: -c with suffix K (1024 bytes)
		{
			Name:  "c-suffix-K",
			Args:  []string{"-c", "1K"},
			Stdin: []byte(largeBuf),
		},
		// R2.3: -c with suffix KiB (1024 bytes, same as K)
		{
			Name:  "c-suffix-KiB",
			Args:  []string{"-c", "1KiB"},
			Stdin: []byte(largeBuf),
		},
		// R2.3: -c with suffix kB (1000 bytes)
		{
			Name:  "c-suffix-kB",
			Args:  []string{"-c", "1kB"},
			Stdin: []byte(largeBuf),
		},
		// R2.3: -c with suffix b combined with +N
		{
			Name:  "c-plus-suffix-b",
			Args:  []string{"-c", "+1b"},
			Stdin: []byte(largeBuf),
		},
		// R2.1: -c with byte count larger than input
		{
			Name:  "c-larger-than-input",
			Args:  []string{"-c", "100"},
			Stdin: []byte(byteContent),
		},
		// R2.1: -c with multi-file
		{
			Name: "c-multi-file",
			Args: []string{"-c", "5", fileBytes, file5},
		},
		// R2.2: -c +N with multi-file
		{
			Name: "c-plus-multi-file",
			Args: []string{"-c", "+10", fileBytes, file5},
		},
		// R2.1: -c with -q on multi-file
		{
			Name: "c-quiet-multi-file",
			Args: []string{"-q", "-c", "5", fileBytes, file5},
		},
		// R2.1: -c with -v on single file
		{
			Name: "c-verbose-single-file",
			Args: []string{"-v", "-c", "5", fileBytes},
		},
		// R2.3: -c with suffix K and --bytes= form
		{
			Name:  "bytes-long-form-suffix-K",
			Args:  []string{"--bytes=1K"},
			Stdin: []byte(largeBuf),
		},
		// R2.3: -c with suffix kB via short form
		{
			Name:  "c-suffix-kB-short",
			Args:  []string{"-c1kB"},
			Stdin: []byte(largeBuf),
		},
		// R2.3: -c with suffix M (larger than input, outputs all)
		{
			Name:  "c-suffix-M-larger-than-input",
			Args:  []string{"-c", "1M"},
			Stdin: []byte(largeBuf),
		},
		// R2.3: suffix with from-start via --bytes=+1K
		{
			Name:  "bytes-long-form-plus-suffix-K",
			Args:  []string{"--bytes=+1K"},
			Stdin: []byte(largeBuf),
		},
		// R3.1: multi-file with stdin via dash shows "standard input" header
		{
			Name:  "multi-file-with-stdin-dash",
			Args:  []string{file5, "-"},
			Stdin: []byte("stdin line\n"),
		},
		// R3.2: single file no header (--lines form)
		{
			Name: "single-file-no-header-lines",
			Args: []string{"--lines=3", file5},
		},
		// R3.3: --quiet long form suppresses headers
		{
			Name: "quiet-long-form-multi-file",
			Args: []string{"--quiet", file20, file5},
		},
		// R3.3: --silent long form suppresses headers
		{
			Name: "silent-long-form-multi-file",
			Args: []string{"--silent", file20, file5},
		},
		// R3.3: -q with single file (no-op, but must not error)
		{
			Name: "quiet-single-file",
			Args: []string{"-q", file5},
		},
		// R3.4: --verbose long form forces header on single file
		{
			Name: "verbose-long-form-single-file",
			Args: []string{"--verbose", file5},
		},
		// R3.4: -v with multiple files (headers still shown)
		{
			Name: "verbose-multi-file",
			Args: []string{"-v", file20, file5},
		},
		// R3.4: -v with stdin (no files) shows "standard input" header
		{
			Name:  "verbose-stdin-no-files",
			Args:  []string{"-v"},
			Stdin: []byte(lines5),
		},
		// R3.1: multi-file with -n +N from start
		{
			Name: "multi-file-n-plus-3",
			Args: []string{"-n", "+3", file20, file5},
		},
		// R3.1: multi-file with -c byte mode
		{
			Name: "multi-file-c-3",
			Args: []string{"-c", "3", fileBytes, file5},
		},
		// R3.1, R4.4: non-existent between valid files with headers
		{
			Name:      "multi-file-error-middle-headers",
			Args:      []string{"-n", "2", file5, nonExistent, file2},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.1: exit 0 on successful single file read
		{
			Name: "exit-0-single-file",
			Args: []string{"-n", "3", file5},
		},
		// R4.1: exit 0 on successful multi-file read
		{
			Name: "exit-0-multi-file",
			Args: []string{"-n", "2", file5, file2},
		},
		// R4.1: exit 0 on successful stdin read
		{
			Name:  "exit-0-stdin",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		// R4.2: exit 1 when only file is non-existent
		{
			Name:      "exit-1-only-nonexistent",
			Args:      []string{nonExistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2: exit 1 with multiple non-existent files
		{
			Name:      "exit-1-multiple-nonexistent",
			Args:      []string{nonExistent, filepath.Join(tmpDir, "also-missing.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2, R4.4: non-existent file at end, valid file still produces output
		{
			Name:      "valid-then-nonexistent",
			Args:      []string{file5, nonExistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2, R4.4: non-existent file at start, valid file still produces output
		{
			Name:      "nonexistent-then-valid",
			Args:      []string{nonExistent, file5},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2, R4.4: non-existent with byte mode
		{
			Name:      "nonexistent-byte-mode",
			Args:      []string{"-c", "5", nonExistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2, R4.4: non-existent with -q (quiet still reports errors to stderr)
		{
			Name:      "nonexistent-quiet",
			Args:      []string{"-q", file5, nonExistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2, R4.4: non-existent with -v (verbose still reports errors)
		{
			Name:      "nonexistent-verbose",
			Args:      []string{"-v", nonExistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.3: directory as input (cannot read)
		{
			Name:      "directory-as-input",
			Args:      []string{tmpDir},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.3, R4.4: directory mixed with valid file
		{
			Name:      "directory-mixed-with-valid",
			Args:      []string{file5, tmpDir},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
