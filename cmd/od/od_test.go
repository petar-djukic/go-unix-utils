// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearData is a normalizer that clears all output. Used for error tests
// where stderr messages differ between Go and GNU implementations but
// exit codes should match.
func clearData(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests comparing the Go od against GNU god.
// Covers srd072-od R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("god")
	if err != nil {
		t.Skip("reference binary god not in PATH")
	}
	tests := []testutils.DiffTest{
		// R1.1, R1.4, R2.4: default octal dump from stdin with final address.
		{
			Name:  "default_octal",
			Stdin: []byte("\x01\x02\x03\x04"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -t x1 hex byte format.
		{
			Name:  "type_hex_byte",
			Args:  []string{"-t", "x1"},
			Stdin: []byte("hello"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -t c C-style character display.
		{
			Name:  "type_c_chars",
			Args:  []string{"-t", "c"},
			Stdin: []byte("hello\nworld"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -t a named character display.
		{
			Name:  "type_named_chars",
			Args:  []string{"-t", "a"},
			Stdin: []byte("AB\x00\x1f"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -t d2 signed decimal 2-byte.
		{
			Name:  "type_signed_d2",
			Args:  []string{"-t", "d2"},
			Stdin: []byte("\x01\x02\x03\x04"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -t u2 unsigned decimal 2-byte.
		{
			Name:  "type_unsigned_u2",
			Args:  []string{"-t", "u2"},
			Stdin: []byte("\x01\x02\x03\x04"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -t f4 float.
		{
			Name:  "type_float_f4",
			Args:  []string{"-t", "f4"},
			Stdin: []byte("\x00\x00\x80\x3f\x00\x00\x00\x40"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: multiple -t options produce multiple output lines per block.
		{
			Name:  "multiple_types_x1_c",
			Args:  []string{"-t", "x1", "-t", "c"},
			Stdin: []byte("abcd"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -A d decimal address radix.
		{
			Name:  "addr_radix_decimal",
			Args:  []string{"-A", "d"},
			Stdin: []byte("\x01\x02\x03\x04"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -A x hex address radix with -t x1.
		{
			Name:  "addr_radix_hex",
			Args:  []string{"-A", "x", "-t", "x1"},
			Stdin: []byte("hello"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -A n no addresses.
		{
			Name:  "addr_radix_none",
			Args:  []string{"-A", "n"},
			Stdin: []byte("\x01\x02\x03\x04"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -A o octal address (default, explicit).
		{
			Name:  "addr_radix_octal_explicit",
			Args:  []string{"-A", "o"},
			Stdin: []byte("ABCD"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -j skip bytes.
		{
			Name:  "skip_bytes",
			Args:  []string{"-j", "6", "-t", "c"},
			Stdin: []byte("hello world"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -j with multiplier suffix b (512).
		{
			Name:  "skip_bytes_suffix_b",
			Args:  []string{"-j", "3b", "-t", "x1"},
			Stdin: make([]byte, 2048),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -N read bytes limit.
		{
			Name:  "read_bytes_limit",
			Args:  []string{"-N", "5", "-t", "c"},
			Stdin: []byte("hello world"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -N with small limit.
		{
			Name:  "read_bytes_small",
			Args:  []string{"-N", "2"},
			Stdin: []byte("ABCDEFGH"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2 + R2.3: skip and limit combined.
		{
			Name:  "skip_and_limit",
			Args:  []string{"-j", "6", "-N", "5", "-t", "c"},
			Stdin: []byte("hello world"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: final address line with hex radix.
		{
			Name:  "final_addr_hex",
			Args:  []string{"-A", "x", "-t", "x1"},
			Stdin: []byte("ABCDEFGHIJKLMNOP"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: final address with decimal radix.
		{
			Name:  "final_addr_decimal",
			Args:  []string{"-A", "d", "-t", "x1"},
			Stdin: []byte("ABCDEF"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: traditional -x option (hex 2-byte).
		{
			Name:  "traditional_x",
			Args:  []string{"-x"},
			Stdin: []byte("AB"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: traditional -b option (octal byte).
		{
			Name:  "traditional_b",
			Args:  []string{"-b"},
			Stdin: []byte("AB"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: traditional -c option (C chars).
		{
			Name:  "traditional_c",
			Args:  []string{"-c"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: traditional -d option (unsigned decimal 2-byte).
		{
			Name:  "traditional_d",
			Args:  []string{"-d"},
			Stdin: []byte("AB"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: traditional -s option (signed decimal 2-byte).
		{
			Name:  "traditional_s",
			Args:  []string{"-s"},
			Stdin: []byte("\xff\xfe"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: traditional -o option (octal 2-byte).
		{
			Name:  "traditional_o",
			Args:  []string{"-o"},
			Stdin: []byte("AB"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty input: just final address.
		{
			Name:  "empty_input",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -w8 explicit narrow width.
		{
			Name:  "width_8",
			Args:  []string{"-w8", "-t", "x1"},
			Stdin: []byte("ABCDEFGHIJKLMNOP"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: -w without value defaults to 32.
		{
			Name:  "width_no_value",
			Args:  []string{"-w", "-t", "x1"},
			Stdin: []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: --width=8 long form with value.
		{
			Name:  "width_long_eq",
			Args:  []string{"--width=8", "-t", "x1"},
			Stdin: []byte("ABCDEFGHIJKLMNOP"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: --width long form without value defaults to 32.
		{
			Name:  "width_long_no_value",
			Args:  []string{"--width", "-t", "x1"},
			Stdin: []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: duplicate suppression replaces repeated lines with '*'.
		{
			Name:  "duplicate_suppression",
			Args:  []string{"-t", "x1"},
			Stdin: make([]byte, 64),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.4: -v disables duplicate suppression, all lines printed.
		{
			Name:  "output_duplicates_v",
			Args:  []string{"-v", "-t", "x1"},
			Stdin: make([]byte, 64),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.4: --output-duplicates long form.
		{
			Name:  "output_duplicates_long",
			Args:  []string{"--output-duplicates", "-t", "x1"},
			Stdin: make([]byte, 64),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: combined traditional options -bc.
		{
			Name:  "traditional_combined_bc",
			Args:  []string{"-b", "-c"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: duplicate suppression with non-zero repeated data.
		{
			Name:  "duplicate_suppression_nonzero",
			Args:  []string{"-t", "o1"},
			Stdin: func() []byte {
				b := make([]byte, 80)
				for i := range b {
					b[i] = 0xAA
				}
				return b
			}(),
			Env: []string{"LC_ALL=C"},
		},
		// R3.1 + R3.3: width with duplicate suppression.
		{
			Name:  "width_with_dup_suppress",
			Args:  []string{"-w8", "-t", "x1"},
			Stdin: make([]byte, 40),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1 + R3.4: width with -v shows all lines.
		{
			Name:  "width_with_v",
			Args:  []string{"-w8", "-v", "-t", "x1"},
			Stdin: make([]byte, 40),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1, R4.4: exit 0 on successful dump.
		{
			Name:     "exit_zero_success",
			Args:     []string{"-t", "x1"},
			Stdin:    []byte("test"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R4.4: -t d1 signed decimal 1-byte size variant.
		{
			Name:  "type_signed_d1",
			Args:  []string{"-t", "d1"},
			Stdin: []byte("\x80\x7f\x00\xff"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t d4 signed decimal 4-byte size variant.
		{
			Name:  "type_signed_d4",
			Args:  []string{"-t", "d4"},
			Stdin: []byte("\x01\x02\x03\x04\x05\x06\x07\x08"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t o4 octal 4-byte size variant.
		{
			Name:  "type_octal_o4",
			Args:  []string{"-t", "o4"},
			Stdin: []byte("\x01\x02\x03\x04\x05\x06\x07\x08"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t u1 unsigned decimal 1-byte size variant.
		{
			Name:  "type_unsigned_u1",
			Args:  []string{"-t", "u1"},
			Stdin: []byte("\x00\x7f\x80\xff"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t u4 unsigned decimal 4-byte size variant.
		{
			Name:  "type_unsigned_u4",
			Args:  []string{"-t", "u4"},
			Stdin: []byte("\x01\x02\x03\x04\x05\x06\x07\x08"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t x4 hexadecimal 4-byte size variant.
		{
			Name:  "type_hex_x4",
			Args:  []string{"-t", "x4"},
			Stdin: []byte("\x01\x02\x03\x04\x05\x06\x07\x08"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t f8 double float size variant.
		{
			Name:  "type_float_f8",
			Args:  []string{"-t", "f8"},
			Stdin: []byte("\x00\x00\x00\x00\x00\x00\xf0\x3f\x00\x00\x00\x00\x00\x00\x00\x40"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t o1 octal 1-byte size variant.
		{
			Name:  "type_octal_o1",
			Args:  []string{"-t", "o1"},
			Stdin: []byte("\x00\x7f\x80\xff"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: -t x2 hexadecimal 2-byte size variant.
		{
			Name:  "type_hex_x2",
			Args:  []string{"-t", "x2"},
			Stdin: []byte("\x01\x02\x03\x04"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.4: multiple -t options with different types (o2 + x1).
		{
			Name:  "multiple_types_o2_x1",
			Args:  []string{"-t", "o2", "-t", "x1"},
			Stdin: []byte("ABCDEFGH"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFiles tests file-based inputs including multi-file and error cases.
// Covers srd072-od R1.4 (multi-file), R4.2 (error exits), R4.3, R4.4.
func TestDiffFiles(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("god")
	if err != nil {
		t.Skip("reference binary god not in PATH")
	}

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "file1.txt"), []byte("hello"))
	writeTestFile(t, filepath.Join(dir, "file2.txt"), []byte(" world"))

	tests := []testutils.DiffTest{
		// R1.4, R4.4: multi-file input reads files in order.
		{
			Name:    "multi_file_input",
			Args:    []string{"-t", "c", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4, R4.4: single file input.
		{
			Name:    "single_file_input",
			Args:    []string{"-t", "x1", "file1.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R4.2, R4.4: missing file causes exit 1.
		{
			Name:      "error_missing_file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearData},
		},
		// R4.2, R4.4: invalid type causes exit 1.
		{
			Name:      "error_invalid_type",
			Args:      []string{"-t", "z"},
			Stdin:     []byte("test"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearData},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
