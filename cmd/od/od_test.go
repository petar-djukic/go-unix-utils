// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go od against GNU god.
// Covers srd072-od R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
