// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "god: " with "od: " so stderr from
// the reference binary matches our binary's program name prefix.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("god: "), []byte("od: "))
}

// TestDiff verifies prd072-od R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4 via differential testing.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("god")
	if err != nil {
		t.Skip("reference binary god not in PATH")
	}

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.bin")
	file2 := filepath.Join(tmpDir, "file2.bin")
	writeTestFile(t, file1, []byte("hello"))
	writeTestFile(t, file2, []byte(" world"))

	tests := buildR1TestCases(file1, file2)
	tests = append(tests, buildR2TestCases(file1)...)
	tests = append(tests, buildR3TestCases()...)
	tests = append(tests, buildR4TestCases(tmpDir)...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func buildR1TestCases(file1, file2 string) []testutils.DiffTest {
	env := []string{"LC_ALL=C"}
	bin := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	text := []byte("hello world\n")
	// Exact float values for deterministic formatting.
	// float32 1.0 = 0x3F800000 LE, float32 2.0 = 0x40000000 LE.
	f4in := []byte{0x00, 0x00, 0x80, 0x3f, 0x00, 0x00, 0x00, 0x40}
	// float64 1.0 = 0x3FF0000000000000 LE.
	f8in := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f}

	return []testutils.DiffTest{
		// R1.1: default octal 2-byte format
		{Name: "default_octal", Stdin: bin, Env: env},
		// R1.2: type specifiers
		{Name: "type_o1", Args: []string{"-t", "o1"}, Stdin: bin, Env: env},
		{Name: "type_o2", Args: []string{"-t", "o2"}, Stdin: bin, Env: env},
		{Name: "type_x1", Args: []string{"-t", "x1"}, Stdin: bin, Env: env},
		{Name: "type_x2", Args: []string{"-t", "x2"}, Stdin: bin, Env: env},
		{Name: "type_x4", Args: []string{"-t", "x4"}, Stdin: bin, Env: env},
		{Name: "type_d2", Args: []string{"-t", "d2"}, Stdin: bin, Env: env},
		{Name: "type_u2", Args: []string{"-t", "u2"}, Stdin: bin, Env: env},
		{Name: "type_d1", Args: []string{"-t", "d1"}, Stdin: bin, Env: env},
		{Name: "type_u1", Args: []string{"-t", "u1"}, Stdin: bin, Env: env},
		{Name: "type_a", Args: []string{"-t", "a"}, Stdin: text, Env: env},
		{Name: "type_c", Args: []string{"-t", "c"}, Stdin: text, Env: env},
		{Name: "type_f4", Args: []string{"-t", "f4"}, Stdin: f4in, Env: env},
		{Name: "type_f8", Args: []string{"-t", "f8"}, Stdin: f8in, Env: env},
		{Name: "format_long", Args: []string{"--format=x1"}, Stdin: bin, Env: env},
		{Name: "combined_flag", Args: []string{"-tx1"}, Stdin: bin, Env: env},
		// R1.3: multiple -t options
		{Name: "multi_type", Args: []string{"-t", "x1", "-t", "c"}, Stdin: text, Env: env},
		// R1.4: file input
		{Name: "file_input", Args: []string{file1}, Env: env},
		{Name: "multi_file", Args: []string{file1, file2}, Env: env},
		{Name: "stdin_dash", Args: []string{"-t", "x1", "-"}, Stdin: bin, Env: env},
		{Name: "empty_input", Stdin: []byte{}, Env: env},
	}
}

func buildR2TestCases(file1 string) []testutils.DiffTest {
	env := []string{"LC_ALL=C"}
	bin := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	text := []byte("hello world\n")

	return []testutils.DiffTest{
		// R2.1: address radix
		{Name: "addr_decimal", Args: []string{"-A", "d"}, Stdin: bin, Env: env},
		{Name: "addr_hex", Args: []string{"-A", "x"}, Stdin: bin, Env: env},
		{Name: "addr_none", Args: []string{"-A", "n"}, Stdin: bin, Env: env},
		{Name: "addr_octal_explicit", Args: []string{"-A", "o"}, Stdin: bin, Env: env},
		{Name: "addr_radix_long", Args: []string{"--address-radix=x"}, Stdin: bin, Env: env},
		{Name: "addr_radix_combined", Args: []string{"-Ax"}, Stdin: bin, Env: env},
		{Name: "addr_hex_with_type", Args: []string{"-A", "x", "-t", "x1"}, Stdin: bin, Env: env},
		// R2.2: skip bytes
		{Name: "skip_3", Args: []string{"-j", "3"}, Stdin: text, Env: env},
		{Name: "skip_long", Args: []string{"--skip-bytes=5"}, Stdin: text, Env: env},
		{Name: "skip_combined", Args: []string{"-j5"}, Stdin: text, Env: env},
		{Name: "skip_file", Args: []string{"-j", "2", file1}, Env: env},
		// R2.3: read bytes
		{Name: "read_4", Args: []string{"-N", "4"}, Stdin: text, Env: env},
		{Name: "read_long", Args: []string{"--read-bytes=5"}, Stdin: text, Env: env},
		{Name: "read_combined", Args: []string{"-N5"}, Stdin: text, Env: env},
		{Name: "read_file", Args: []string{"-N", "3", file1}, Env: env},
		// R2.2 + R2.3 combined
		{Name: "skip_and_read", Args: []string{"-j", "3", "-N", "4"}, Stdin: text, Env: env},
		// R2.4: final address line with different radixes
		{Name: "final_addr_decimal", Args: []string{"-A", "d", "-t", "x1"}, Stdin: bin, Env: env},
		{Name: "final_addr_none", Args: []string{"-A", "n", "-t", "x1"}, Stdin: bin, Env: env},
	}
}

// buildR4TestCases covers R4.1 (exit 0 on success), R4.2 (exit 1 on error),
// R4.3 (differential testing), and R4.4 (error cases: invalid type, missing file).
func buildR4TestCases(tmpDir string) []testutils.DiffTest {
	env := []string{"LC_ALL=C"}
	norm := []testutils.NormalizeFunc{normalizeProgramName}
	missingFile := filepath.Join(tmpDir, "nonexistent.bin")
	return []testutils.DiffTest{
		// R4.1: exit 0 on successful dump
		{Name: "exit_0_success", Args: []string{"-t", "x1"}, Stdin: []byte{0xAB}, Env: env, ExitCode: 0},
		// R4.2: exit 1 on invalid type specifier
		{Name: "err_invalid_type", Args: []string{"-t", "z"}, Stdin: []byte{0x01}, Env: env, ExitCode: 1, Normalize: norm},
		// R4.2: exit 1 on missing file
		{Name: "err_missing_file", Args: []string{missingFile}, Env: env, ExitCode: 1, Normalize: norm},
		// R4.2: exit 1 on invalid address radix
		{Name: "err_invalid_radix", Args: []string{"-A", "q"}, Stdin: []byte{0x01}, Env: env, ExitCode: 1, Normalize: norm},
	}
}

func buildR3TestCases() []testutils.DiffTest {
	env := []string{"LC_ALL=C"}
	bin := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	text := []byte("hello world\n")
	// R3.3/R3.4: 64 bytes = 4 identical 16-byte blocks for duplicate testing.
	block16 := []byte{
		0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
		0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50,
	}
	dup := make([]byte, 0, 64)
	dup = append(dup, block16...)
	dup = append(dup, block16...)
	dup = append(dup, block16...)
	dup = append(dup, block16...)

	return []testutils.DiffTest{
		// R3.1: width control
		{Name: "width_8", Args: []string{"-w8"}, Stdin: bin, Env: env},
		{Name: "width_32_default", Args: []string{"-w"}, Stdin: text, Env: env},
		{Name: "width_long", Args: []string{"--width=4"}, Stdin: bin, Env: env},
		{Name: "width_long_default", Args: []string{"--width"}, Stdin: text, Env: env},
		// R3.2: traditional short options
		{Name: "trad_b", Args: []string{"-b"}, Stdin: bin, Env: env},
		{Name: "trad_c", Args: []string{"-c"}, Stdin: text, Env: env},
		{Name: "trad_d", Args: []string{"-d"}, Stdin: bin, Env: env},
		{Name: "trad_o", Args: []string{"-o"}, Stdin: bin, Env: env},
		{Name: "trad_s", Args: []string{"-s"}, Stdin: bin, Env: env},
		{Name: "trad_x", Args: []string{"-x"}, Stdin: bin, Env: env},
		// R3.3: duplicate suppression with '*'
		{Name: "dup_suppress", Stdin: dup, Env: env},
		{Name: "dup_suppress_hex", Args: []string{"-t", "x1"}, Stdin: dup, Env: env},
		// R3.4: output duplicates (-v disables suppression)
		{Name: "verbose", Args: []string{"-v"}, Stdin: dup, Env: env},
		{Name: "verbose_long", Args: []string{"--output-duplicates"}, Stdin: dup, Env: env},
		{Name: "verbose_hex", Args: []string{"-v", "-t", "x1"}, Stdin: dup, Env: env},
	}
}
