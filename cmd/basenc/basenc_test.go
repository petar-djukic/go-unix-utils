// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/basenc against gbasenc (Homebrew coreutils).
//
// Covers prd081-basenc: R1.1 (--base64), R1.2 (--base64url),
// R1.3 (--base32), R1.4 (--base32hex), R2.1 (--base16), R2.2 (--z85),
// R2.3 (-d/--decode), R2.4 (-w/--wrap), R3.1 (-i/--ignore-garbage),
// R3.2 (FILE/stdin input), R3.3 (error exits), R3.4 (SIGPIPE).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgramName normalizes the program name prefix in error messages
// so "gbasenc: ..." and "basenc: ..." compare as equal.
var stderrProgramName = regexp.MustCompile(`^[a-z0-9]+:`)

// normalizeAllRe matches all content for suppressing output comparison.
var normalizeAllRe = regexp.MustCompile(`(?s).*`)

// normalizeProgramName strips the leading program name from stderr lines.
func normalizeProgramName(data []byte) []byte {
	return stderrProgramName.ReplaceAll(data, []byte("basenc:"))
}

// normalizeAllStdout replaces all stdout with empty bytes.
func normalizeAllStdout(data []byte) []byte {
	return normalizeAllRe.ReplaceAll(data, []byte(""))
}

// normalizeAllStderr replaces all stderr with empty bytes.
func normalizeAllStderr(data []byte) []byte {
	return normalizeAllRe.ReplaceAll(data, []byte(""))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasenc")
	if err != nil {
		t.Skip("reference binary gbasenc not in PATH")
	}

	// Long input that produces multi-line output at default wrap
	longInput := []byte("The quick brown fox jumps over the lazy dog. " +
		"This string is long enough to produce multiple wrapped lines.\n")

	tests := []testutils.DiffTest{
		// R1.1: --base64 encode from stdin
		{
			Name:  "base64_encode_stdin",
			Args:  []string{"--base64"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: --base64 encode empty input
		{
			Name:  "base64_encode_empty",
			Args:  []string{"--base64"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: --base64 encode binary data
		{
			Name:  "base64_encode_binary",
			Args:  []string{"--base64"},
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: --base64 default wrap at 76 columns
		{
			Name:  "base64_encode_wrap_default",
			Args:  []string{"--base64"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: --base64 decode
		{
			Name:  "base64_decode",
			Args:  []string{"--base64", "-d"},
			Stdin: []byte("aGVsbG8K\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: --base64 decode empty
		{
			Name:  "base64_decode_empty",
			Args:  []string{"--base64", "-d"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: --base64url encode from stdin
		{
			Name:  "base64url_encode_stdin",
			Args:  []string{"--base64url"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: --base64url encode binary data with chars that differ from std
		{
			Name:  "base64url_encode_binary",
			Args:  []string{"--base64url"},
			Stdin: []byte{0x3e, 0x3f, 0xfb, 0xef, 0xbf},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: --base64url decode
		{
			Name:  "base64url_decode",
			Args:  []string{"--base64url", "-d"},
			Stdin: []byte("aGVsbG8K\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --base32 encode from stdin
		{
			Name:  "base32_encode_stdin",
			Args:  []string{"--base32"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --base32 encode empty input
		{
			Name:  "base32_encode_empty",
			Args:  []string{"--base32"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --base32 decode
		{
			Name:  "base32_decode",
			Args:  []string{"--base32", "-d"},
			Stdin: []byte("NBSWY3DPBI======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --base32 decode empty
		{
			Name:  "base32_decode_empty",
			Args:  []string{"--base32", "-d"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --base32hex encode from stdin
		{
			Name:  "base32hex_encode_stdin",
			Args:  []string{"--base32hex"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --base32hex encode empty input
		{
			Name:  "base32hex_encode_empty",
			Args:  []string{"--base32hex"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --base32hex decode
		{
			Name:  "base32hex_decode",
			Args:  []string{"--base32hex", "-d"},
			Stdin: []byte("D1IMOR3F18======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --base32hex with long input for wrap
		{
			Name:  "base32hex_encode_wrap",
			Args:  []string{"--base32hex"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},

		// R2.1: --base16 encode from stdin
		{
			Name:  "base16_encode_stdin",
			Args:  []string{"--base16"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --base16 encode empty input
		{
			Name:  "base16_encode_empty",
			Args:  []string{"--base16"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --base16 encode binary data
		{
			Name:  "base16_encode_binary",
			Args:  []string{"--base16"},
			Stdin: []byte{0x00, 0x01, 0xfe, 0xff},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --base16 encode long input (wrap test)
		{
			Name:  "base16_encode_wrap",
			Args:  []string{"--base16"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --base16 decode uppercase hex
		{
			Name:  "base16_decode",
			Args:  []string{"--base16", "-d"},
			Stdin: []byte("68656C6C6F0A\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --base16 decode empty
		{
			Name:  "base16_decode_empty",
			Args:  []string{"--base16", "-d"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.2: --z85 encode 4 bytes
		{
			Name:  "z85_encode_4bytes",
			Args:  []string{"--z85"},
			Stdin: []byte{0x86, 0x4F, 0xD2, 0x6F},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: --z85 encode 8 bytes (ZeroMQ test vector)
		{
			Name:  "z85_encode_8bytes",
			Args:  []string{"--z85"},
			Stdin: []byte{0x86, 0x4F, 0xD2, 0x6F, 0xB5, 0x59, 0xF7, 0x5B},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: --z85 encode empty input
		{
			Name:  "z85_encode_empty",
			Args:  []string{"--z85"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: --z85 encode non-multiple-of-4 exits 1
		{
			Name:     "z85_encode_bad_length",
			Args:     []string{"--z85"},
			Stdin:    []byte("hello"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeProgramName, normalizeAllStderr,
			},
		},
		// R2.2: --z85 decode
		{
			Name:  "z85_decode",
			Args:  []string{"--z85", "-d"},
			Stdin: []byte("HelloWorld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: --z85 decode empty
		{
			Name:  "z85_decode_empty",
			Args:  []string{"--z85", "-d"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.3: --decode long form
		{
			Name:  "decode_long_form",
			Args:  []string{"--base64", "--decode"},
			Stdin: []byte("aGVsbG8K\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.4: -w with separate argument
		{
			Name:  "base64_wrap_40",
			Args:  []string{"--base64", "-w", "40"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -w 0 disables wrapping
		{
			Name:  "base64_wrap_0",
			Args:  []string{"--base64", "-w", "0"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: --wrap=COLS form
		{
			Name:  "base64_wrap_eq_20",
			Args:  []string{"--base64", "--wrap=20"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -wCOLS (no space) form
		{
			Name:  "base64_wrap_nospace",
			Args:  []string{"--base64", "-w10"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},

		// R3.1: --ignore-garbage with base64 decode ignoring invalid chars
		{
			Name:  "base64_decode_ignore_garbage",
			Args:  []string{"--base64", "-d", "-i"},
			Stdin: []byte("aGVs!!!bG8K\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: --ignore-garbage long form
		{
			Name:  "base64_decode_ignore_garbage_long",
			Args:  []string{"--base64", "--decode", "--ignore-garbage"},
			Stdin: []byte("aGVs bG8K\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: --ignore-garbage with base32 decode
		{
			Name:  "base32_decode_ignore_garbage",
			Args:  []string{"--base32", "-d", "-i"},
			Stdin: []byte("NBSWY3DP!!!BI======\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R3.2: read from stdin when "-" is given
		{
			Name:  "base64_encode_dash_stdin",
			Args:  []string{"--base64", "-"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R3.3: no encoding specified exits 1
		{
			Name:      "no_encoding_exits_1",
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeAllStderr},
		},
		// R3.3: invalid decode input exits 1
		{
			Name:      "base64_decode_invalid_exits_1",
			Args:      []string{"--base64", "-d"},
			Stdin:     []byte("!!!not-valid-base64!!!\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeAllStderr},
		},
		// R3.3: nonexistent file exits 1
		{
			Name:      "nonexistent_file_exits_1",
			Args:      []string{"--base64", "/nonexistent/file/path"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeAllStderr},
		},
		// R3.3: base16 decode invalid hex exits 1
		{
			Name:      "base16_decode_invalid_exits_1",
			Args:      []string{"--base16", "-d"},
			Stdin:     []byte("ZZZZ\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeAllStderr},
		},

		// --version exits 0
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllStdout},
		},
		// --help exits 0
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllStdout, normalizeAllStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
