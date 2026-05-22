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

var discardStderr = testutils.NormalizeFunc(func([]byte) []byte { return nil })

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("god")
	if err != nil {
		t.Skip("reference binary god not found")
	}
	tests := []testutils.DiffTest{
		{Name: "default_octal_dump", Stdin: []byte{0x01, 0x02, 0x03, 0x04}},
		{Name: "default_16_bytes", Stdin: bytes16()},
		{Name: "empty_input", Stdin: []byte{}},
	}
	tests = append(tests, typeSpecTests()...)
	tests = append(tests, multiTypeTests()...)
	tests = append(tests, addressRadixTests()...)
	tests = append(tests, skipAndReadTests(t)...)
	tests = append(tests, stdinAndFileTests(t)...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func bytes16() []byte {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return b
}

func typeSpecTests() []testutils.DiffTest {
	data := bytes16()
	small := []byte{0x01, 0x02, 0x03, 0x04}
	return []testutils.DiffTest{
		{Name: "type_a", Args: []string{"-t", "a"}, Stdin: data},
		{Name: "type_c", Args: []string{"-t", "c"}, Stdin: data},
		{Name: "type_c_escapes", Args: []string{"-t", "c"}, Stdin: []byte("Hello\t\n\000\\")},
		{Name: "type_o1", Args: []string{"-t", "o1"}, Stdin: small},
		{Name: "type_o2", Args: []string{"-t", "o2"}, Stdin: small},
		{Name: "type_o4", Args: []string{"-t", "o4"}, Stdin: small},
		{Name: "type_x1", Args: []string{"-t", "x1"}, Stdin: data},
		{Name: "type_x2", Args: []string{"-t", "x2"}, Stdin: small},
		{Name: "type_x4", Args: []string{"-t", "x4"}, Stdin: small},
		{Name: "type_d1", Args: []string{"-t", "d1"}, Stdin: []byte{0x01, 0x80, 0xff}},
		{Name: "type_d2", Args: []string{"-t", "d2"}, Stdin: small},
		{Name: "type_d4", Args: []string{"-t", "d4"}, Stdin: small},
		{Name: "type_u1", Args: []string{"-t", "u1"}, Stdin: small},
		{Name: "type_u2", Args: []string{"-t", "u2"}, Stdin: []byte{0xff, 0xfe, 0xfd, 0xfc}},
		{Name: "type_u4", Args: []string{"-t", "u4"}, Stdin: []byte{0xff, 0xff, 0xff, 0xff}},
		{Name: "type_f4", Args: []string{"-t", "f4"}, Stdin: []byte{0x00, 0x00, 0x80, 0x3f}},
		{Name: "type_f8", Args: []string{"-t", "f8"}, Stdin: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0, 0x3f}},
		{Name: "format_long", Args: []string{"--format=x1"}, Stdin: small},
		{Name: "partial_word", Stdin: []byte{0x01, 0x02, 0x03}},
		{Name: "type_a_high_bit", Args: []string{"-t", "a"}, Stdin: []byte{0x80, 0x81, 0xff}},
		{Name: "type_c_high_bytes", Args: []string{"-t", "c"}, Stdin: []byte{0x80, 0x81, 0xff}},
	}
}

func multiTypeTests() []testutils.DiffTest {
	data := bytes16()
	small := []byte{0x01, 0x02, 0x03, 0x04}
	return []testutils.DiffTest{
		{Name: "multi_o2_x2", Args: []string{"-t", "o2", "-t", "x2"}, Stdin: small},
		{Name: "multi_o2_c", Args: []string{"-t", "o2", "-t", "c"}, Stdin: data},
		{Name: "multi_o2_x1", Args: []string{"-t", "o2", "-t", "x1"}, Stdin: data},
		{Name: "multi_x1_o2", Args: []string{"-t", "x1", "-t", "o2"}, Stdin: data},
		{Name: "multi_a_c_x1", Args: []string{"-t", "a", "-t", "c", "-t", "x1"}, Stdin: small},
	}
}

func addressRadixTests() []testutils.DiffTest {
	data := []byte("hello")
	return []testutils.DiffTest{
		{Name: "addr_hex", Args: []string{"-A", "x", "-t", "x1"}, Stdin: data},
		{Name: "addr_dec", Args: []string{"-A", "d", "-t", "x1"}, Stdin: data},
		{Name: "addr_none", Args: []string{"-A", "n", "-t", "x1"}, Stdin: data},
		{Name: "addr_octal", Args: []string{"-A", "o", "-t", "x1"}, Stdin: data},
		{Name: "addr_long_hex", Args: []string{"--address-radix=x", "-t", "x1"}, Stdin: data},
	}
}

func skipAndReadTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	bigFile := filepath.Join(dir, "big.bin")
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(bigFile, data, 0o644); err != nil {
		t.Fatal(err)
	}
	small := []byte("hello world, this is a test!")
	return []testutils.DiffTest{
		{Name: "skip_2", Args: []string{"-j", "2", "-t", "x1"}, Stdin: small},
		{Name: "skip_long", Args: []string{"--skip-bytes=3", "-t", "x1"}, Stdin: small},
		{Name: "skip_all", Args: []string{"-j", "100", "-t", "x1"}, Stdin: small, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{discardStderr}},
		{Name: "skip_suffix_b", Args: []string{"-j", "1b", "-t", "x1", bigFile}},
		{Name: "skip_suffix_k", Args: []string{"-j", "1k", "-t", "x1", bigFile}},
		{Name: "read_4", Args: []string{"-N", "4", "-t", "x1"}, Stdin: small},
		{Name: "read_long", Args: []string{"--read-bytes=5", "-t", "x1"}, Stdin: small},
		{Name: "read_more_than_input", Args: []string{"-N", "100", "-t", "x1"}, Stdin: small},
		{Name: "skip_and_read", Args: []string{"-j", "6", "-N", "5", "-t", "c"}, Stdin: small},
		{Name: "skip_read_hex_addr", Args: []string{"-j", "2", "-N", "4", "-A", "x", "-t", "x1"}, Stdin: small},
		{Name: "skip_read_dec_addr", Args: []string{"-j", "2", "-N", "4", "-A", "d", "-t", "x1"}, Stdin: small},
		{Name: "skip_read_no_addr", Args: []string{"-j", "2", "-N", "4", "-A", "n", "-t", "x1"}, Stdin: small},
		{Name: "final_addr_default", Stdin: []byte{0x01, 0x02, 0x03, 0x04}},
		{Name: "final_addr_hex", Args: []string{"-A", "x"}, Stdin: []byte{0x01, 0x02, 0x03, 0x04}},
		{Name: "final_addr_dec", Args: []string{"-A", "d"}, Stdin: []byte{0x01, 0x02, 0x03, 0x04}},
		{Name: "final_addr_none", Args: []string{"-A", "n"}, Stdin: []byte{0x01, 0x02, 0x03, 0x04}},
		{Name: "final_addr_after_skip", Args: []string{"-j", "2", "-A", "x", "-t", "x1"}, Stdin: small},
	}
}

func stdinAndFileTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	f1 := filepath.Join(dir, "file1.bin")
	f2 := filepath.Join(dir, "file2.bin")
	if err := os.WriteFile(f1, []byte("AB"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("CD"), 0o644); err != nil {
		t.Fatal(err)
	}
	return []testutils.DiffTest{
		{Name: "stdin_dash", Args: []string{"-"}, Stdin: []byte("test")},
		{Name: "single_file", Args: []string{f1}},
		{Name: "multi_file", Args: []string{f1, f2}},
	}
}
