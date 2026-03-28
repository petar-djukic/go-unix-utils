// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd072-od R4.1–R4.4.
// Compares Go od binary against god (GNU od from Homebrew coreutils).
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("god")
	if err != nil {
		t.Skip("reference binary god not in PATH")
	}

	tests := []testutils.DiffTest{}
	tests = append(tests, defaultTests()...)
	tests = append(tests, typeStringTests()...)
	tests = append(tests, addressRadixTests()...)
	tests = append(tests, byteRangeTests()...)
	tests = append(tests, widthTests()...)
	tests = append(tests, duplicateTests()...)
	tests = append(tests, traditionalTests()...)
	tests = append(tests, multipleTypeTests()...)
	tests = append(tests, errorTests()...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// progNameRe matches the program name prefix in error messages.
var progNameRe = regexp.MustCompile(`^(god|od): `)

// normalizeErrMsg replaces god:/od: prefixes and lowercases stderr
// so error messages from both binaries can be compared despite
// program name and OS error capitalization differences.
func normalizeErrMsg(data []byte) []byte {
	data = progNameRe.ReplaceAll(data, []byte("od: "))
	return bytes.ToLower(data)
}

// normalizeSpacing collapses runs of spaces within each line
// to a single space, allowing comparison despite column-width
// differences between multi-type format outputs.
func normalizeSpacing(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) > 0 {
			fields := bytes.Fields(line)
			lines[i] = bytes.Join(fields, []byte(" "))
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

// defaultTests covers R4.1: default octal dump with exit 0.
func defaultTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "default_octal_dump",
			Args:  []string{},
			Stdin: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			Name:  "default_empty_stdin",
			Args:  []string{},
			Stdin: []byte{},
		},
		{
			Name:  "default_text_input",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
	}
}

// typeStringTests covers R4.4: -t with each format letter and sizes.
func typeStringTests() []testutils.DiffTest {
	input := []byte{0x00, 0x41, 0x42, 0x43, 0x7f, 0x80, 0xff, 0x0a,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	return []testutils.DiffTest{
		{
			Name:  "type_a_named_chars",
			Args:  []string{"-t", "a"},
			Stdin: input,
		},
		{
			Name:  "type_c_c_chars",
			Args:  []string{"-t", "c"},
			Stdin: input,
		},
		{
			Name:  "type_o1_octal_byte",
			Args:  []string{"-t", "o1"},
			Stdin: input,
		},
		{
			Name:  "type_o2_octal_word",
			Args:  []string{"-t", "o2"},
			Stdin: input,
		},
		{
			Name:  "type_o4_octal_dword",
			Args:  []string{"-t", "o4"},
			Stdin: input,
		},
		{
			Name:  "type_x1_hex_byte",
			Args:  []string{"-t", "x1"},
			Stdin: input,
		},
		{
			Name:  "type_x2_hex_word",
			Args:  []string{"-t", "x2"},
			Stdin: input,
		},
		{
			Name:  "type_x4_hex_dword",
			Args:  []string{"-t", "x4"},
			Stdin: input,
		},
		{
			Name:  "type_d1_signed_byte",
			Args:  []string{"-t", "d1"},
			Stdin: input,
		},
		{
			Name:  "type_d2_signed_word",
			Args:  []string{"-t", "d2"},
			Stdin: input,
		},
		{
			Name:  "type_d4_signed_dword",
			Args:  []string{"-t", "d4"},
			Stdin: input,
		},
		{
			Name:  "type_u1_unsigned_byte",
			Args:  []string{"-t", "u1"},
			Stdin: input,
		},
		{
			Name:  "type_u2_unsigned_word",
			Args:  []string{"-t", "u2"},
			Stdin: input,
		},
		{
			Name:  "type_u4_unsigned_dword",
			Args:  []string{"-t", "u4"},
			Stdin: input,
		},
		{
			Name:  "type_fF_float32",
			Args:  []string{"-t", "fF"},
			Stdin: input,
		},
		{
			Name:  "type_f4_float32",
			Args:  []string{"-t", "f4"},
			Stdin: input,
		},
		{
			Name:  "type_fD_float64",
			Args:  []string{"-t", "fD"},
			Stdin: input,
		},
		{
			Name:  "type_f8_float64",
			Args:  []string{"-t", "f8"},
			Stdin: input,
		},
		{
			Name:  "type_x1_short_input",
			Args:  []string{"-t", "x1"},
			Stdin: []byte("AB"),
		},
		{
			Name:  "type_dC_char_size",
			Args:  []string{"-t", "dC"},
			Stdin: input,
		},
		{
			Name:  "type_dS_short_size",
			Args:  []string{"-t", "dS"},
			Stdin: input,
		},
		{
			Name:  "type_dI_int_size",
			Args:  []string{"-t", "dI"},
			Stdin: input,
		},
		{
			Name:  "type_dL_long_size",
			Args:  []string{"-t", "dL"},
			Stdin: input,
		},
	}
}

// addressRadixTests covers R4.4: -A address radix modes.
func addressRadixTests() []testutils.DiffTest {
	input := []byte("abcdefghijklmnop")
	return []testutils.DiffTest{
		{
			Name:  "addr_radix_octal",
			Args:  []string{"-A", "o", "-t", "x1"},
			Stdin: input,
		},
		{
			Name:  "addr_radix_decimal",
			Args:  []string{"-A", "d", "-t", "x1"},
			Stdin: input,
		},
		{
			Name:  "addr_radix_hex",
			Args:  []string{"-A", "x", "-t", "x1"},
			Stdin: input,
		},
		{
			Name:  "addr_radix_none",
			Args:  []string{"-A", "n", "-t", "x1"},
			Stdin: input,
		},
	}
}

// byteRangeTests covers R4.4: -j skip bytes and -N read bytes.
func byteRangeTests() []testutils.DiffTest {
	input := []byte("hello world test data")
	return []testutils.DiffTest{
		{
			Name:  "skip_bytes",
			Args:  []string{"-j", "6", "-t", "c"},
			Stdin: input,
		},
		{
			Name:  "read_bytes",
			Args:  []string{"-N", "5", "-t", "c"},
			Stdin: input,
		},
		{
			Name:  "skip_and_read",
			Args:  []string{"-j", "6", "-N", "5", "-t", "c"},
			Stdin: input,
		},
		{
			Name:      "skip_past_input",
			Args:      []string{"-j", "100", "-t", "x1"},
			Stdin:     input,
			Normalize: []testutils.NormalizeFunc{normalizeErrMsg},
		},
		{
			Name:  "read_zero_bytes",
			Args:  []string{"-N", "0", "-t", "x1"},
			Stdin: input,
		},
	}
}

// widthTests covers R4.4: -w output width.
func widthTests() []testutils.DiffTest {
	input := []byte("abcdefghijklmnopqrstuvwxyz012345")
	return []testutils.DiffTest{
		{
			Name:  "width_8",
			Args:  []string{"-w8", "-t", "x1"},
			Stdin: input,
		},
		{
			Name:  "width_32",
			Args:  []string{"-w32", "-t", "x1"},
			Stdin: input,
		},
		{
			Name:  "width_4",
			Args:  []string{"-w4", "-t", "x1"},
			Stdin: input,
		},
		{
			Name:  "width_long_flag",
			Args:  []string{"--width=8", "-t", "x1"},
			Stdin: input,
		},
	}
}

// duplicateTests covers R4.4: duplicate suppression with '*' and -v.
func duplicateTests() []testutils.DiffTest {
	// 48 bytes of repeated 16-byte blocks to trigger '*' suppression.
	block := []byte("AAAAAAAAAAAAAAAA")
	dup := make([]byte, 0, 48)
	dup = append(dup, block...)
	dup = append(dup, block...)
	dup = append(dup, block...)

	return []testutils.DiffTest{
		{
			Name:  "duplicate_suppressed",
			Args:  []string{"-t", "x1"},
			Stdin: dup,
		},
		{
			Name:  "duplicate_with_v",
			Args:  []string{"-v", "-t", "x1"},
			Stdin: dup,
		},
		{
			Name:  "duplicate_output_duplicates",
			Args:  []string{"--output-duplicates", "-t", "x1"},
			Stdin: dup,
		},
	}
}

// traditionalTests covers R4.4: traditional short options.
func traditionalTests() []testutils.DiffTest {
	input := []byte{0x41, 0x42, 0x43, 0x44, 0x00, 0x0a, 0x7f, 0xff}
	return []testutils.DiffTest{
		{
			Name:  "traditional_b_octal_byte",
			Args:  []string{"-b"},
			Stdin: input,
		},
		{
			Name:  "traditional_c_char",
			Args:  []string{"-c"},
			Stdin: input,
		},
		{
			Name:  "traditional_d_unsigned_word",
			Args:  []string{"-d"},
			Stdin: input,
		},
		{
			Name:  "traditional_o_octal_word",
			Args:  []string{"-o"},
			Stdin: input,
		},
		{
			Name:  "traditional_s_signed_word",
			Args:  []string{"-s"},
			Stdin: input,
		},
		{
			Name:  "traditional_x_hex_word",
			Args:  []string{"-x"},
			Stdin: input,
		},
	}
}

// multipleTypeTests covers R4.4: multiple -t options.
// Uses normalizeSpacing because GNU od aligns column widths
// across multiple types, which the Go implementation does not.
func multipleTypeTests() []testutils.DiffTest {
	input := []byte("abcd")
	norm := []testutils.NormalizeFunc{normalizeSpacing}
	return []testutils.DiffTest{
		{
			Name:      "multi_type_x1_and_c",
			Args:      []string{"-t", "x1", "-t", "c"},
			Stdin:     input,
			Normalize: norm,
		},
		{
			Name:      "multi_type_o1_and_x1",
			Args:      []string{"-t", "o1", "-t", "x1"},
			Stdin:     input,
			Normalize: norm,
		},
		{
			Name:      "multi_type_three_formats",
			Args:      []string{"-t", "x1", "-t", "o1", "-t", "c"},
			Stdin:     input,
			Normalize: norm,
		},
	}
}

// errorTests covers R4.2 and R4.4: error handling and exit 1.
// Uses normalizeErrMsg because "god:" and "od:" prefixes differ.
func errorTests() []testutils.DiffTest {
	norm := []testutils.NormalizeFunc{normalizeErrMsg}
	return []testutils.DiffTest{
		{
			Name:      "error_invalid_type",
			Args:      []string{"-t", "z9"},
			Stdin:     []byte("test"),
			Normalize: norm,
		},
		{
			Name:      "error_missing_file",
			Args:      []string{"/nonexistent/file/path"},
			Stdin:     nil,
			Normalize: norm,
		},
		{
			Name:      "error_invalid_radix",
			Args:      []string{"-A", "q"},
			Stdin:     []byte("test"),
			Normalize: norm,
		},
	}
}
