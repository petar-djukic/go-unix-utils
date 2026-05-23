// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func normStderr(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gprintf:"), []byte("printf:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		// R1.1: format string interpretation
		{Name: "literal-text", Args: []string{"hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "literal-with-newline", Args: []string{`hello\n`}, Env: []string{"LC_ALL=C"}},
		{Name: "empty-format", Args: []string{""}, Env: []string{"LC_ALL=C"}},
		{Name: "newline-only", Args: []string{`\n`}, Env: []string{"LC_ALL=C"}},
		{Name: "mixed-literal-directive", Args: []string{`%d apples\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "no-trailing-newline", Args: []string{`%d`, "7"}, Env: []string{"LC_ALL=C"}},
		{Name: "multiple-directives", Args: []string{`%d %s %f\n`, "42", "hello", "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "tab-escape", Args: []string{`a\tb`}, Env: []string{"LC_ALL=C"}},

		// R1.2: integer conversion specifiers
		{Name: "d-positive", Args: []string{`%d\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "d-negative", Args: []string{`%d\n`, "-42"}, Env: []string{"LC_ALL=C"}},
		{Name: "d-zero", Args: []string{`%d\n`, "0"}, Env: []string{"LC_ALL=C"}},
		{Name: "i-specifier", Args: []string{`%i\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "i-negative", Args: []string{`%i\n`, "-7"}, Env: []string{"LC_ALL=C"}},
		{Name: "o-specifier", Args: []string{`%o\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "o-zero", Args: []string{`%o\n`, "0"}, Env: []string{"LC_ALL=C"}},
		{Name: "u-specifier", Args: []string{`%u\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "u-zero", Args: []string{`%u\n`, "0"}, Env: []string{"LC_ALL=C"}},
		{Name: "x-specifier", Args: []string{`%x\n`, "255"}, Env: []string{"LC_ALL=C"}},
		{Name: "x-zero", Args: []string{`%x\n`, "0"}, Env: []string{"LC_ALL=C"}},
		{Name: "X-specifier", Args: []string{`%X\n`, "255"}, Env: []string{"LC_ALL=C"}},
		{Name: "d-octal-input", Args: []string{`%d\n`, "042"}, Env: []string{"LC_ALL=C"}},
		{Name: "d-hex-input", Args: []string{`%d\n`, "0x2A"}, Env: []string{"LC_ALL=C"}},
		{Name: "d-large", Args: []string{`%d\n`, "1000000"}, Env: []string{"LC_ALL=C"}},
		{Name: "x-large", Args: []string{`%x\n`, "65535"}, Env: []string{"LC_ALL=C"}},

		// R1.3: floating-point conversion specifiers
		{Name: "f-basic", Args: []string{`%f\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "f-zero", Args: []string{`%f\n`, "0"}, Env: []string{"LC_ALL=C"}},
		{Name: "f-negative", Args: []string{`%f\n`, "-3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "f-integer-input", Args: []string{`%f\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "e-basic", Args: []string{`%e\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "e-large", Args: []string{`%e\n`, "123456789"}, Env: []string{"LC_ALL=C"}},
		{Name: "g-basic", Args: []string{`%g\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "g-integer", Args: []string{`%g\n`, "100"}, Env: []string{"LC_ALL=C"}},
		{Name: "F-basic", Args: []string{`%F\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "E-basic", Args: []string{`%E\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "G-basic", Args: []string{`%G\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "g-small", Args: []string{`%g\n`, "0.00001"}, Env: []string{"LC_ALL=C"}},

		// R1.4: string, character, backslash-interpreted string
		{Name: "s-basic", Args: []string{`%s\n`, "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "s-empty", Args: []string{`%s\n`, ""}, Env: []string{"LC_ALL=C"}},
		{Name: "s-spaces", Args: []string{`%s\n`, "hello world"}, Env: []string{"LC_ALL=C"}},
		{Name: "c-basic", Args: []string{`%c\n`, "A"}, Env: []string{"LC_ALL=C"}},
		{Name: "c-from-longer", Args: []string{`%c\n`, "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "c-digit", Args: []string{`%c\n`, "9"}, Env: []string{"LC_ALL=C"}},
		{Name: "b-plain", Args: []string{`%b\n`, "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "b-tab", Args: []string{`%b\n`, `hello\tworld`}, Env: []string{"LC_ALL=C"}},
		{Name: "b-newline", Args: []string{`%b`, `hello\nworld`}, Env: []string{"LC_ALL=C"}},
		{Name: "b-backslash", Args: []string{`%b\n`, `a\\b`}, Env: []string{"LC_ALL=C"}},
		{Name: "b-octal", Args: []string{`%b\n`, `\0101`}, Env: []string{"LC_ALL=C"}},
		{Name: "b-suppress", Args: []string{`%b`, `before\cafter`}, Env: []string{"LC_ALL=C"}},
		{Name: "b-hex", Args: []string{`%b\n`, `\x41`}, Env: []string{"LC_ALL=C"}},

		// R1.2+R1.4: multiple specifiers combined
		{Name: "int-and-string", Args: []string{`%d %s\n`, "42", "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "all-int-types", Args: []string{`%d %i %o %u %x %X\n`, "42", "42", "42", "42", "42", "42"}, Env: []string{"LC_ALL=C"}},

		// R2.1: field width and precision
		{Name: "width-int", Args: []string{`%10d\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "width-string", Args: []string{`%10s\n`, "hi"}, Env: []string{"LC_ALL=C"}},
		{Name: "precision-float", Args: []string{`%.5f\n`, "3.14159"}, Env: []string{"LC_ALL=C"}},
		{Name: "precision-float-2", Args: []string{`%.2f\n`, "3.14159"}, Env: []string{"LC_ALL=C"}},
		{Name: "width-precision-float", Args: []string{`%10.3f\n`, "3.14159"}, Env: []string{"LC_ALL=C"}},
		{Name: "precision-string", Args: []string{`%.3s\n`, "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "width-precision-string", Args: []string{`%10.3s\n`, "hello"}, Env: []string{"LC_ALL=C"}},
		{Name: "precision-zero-float", Args: []string{`%.0f\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "width-int-small", Args: []string{`%3d\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "width-int-exact", Args: []string{`%2d\n`, "42"}, Env: []string{"LC_ALL=C"}},

		// R2.2: flag characters
		{Name: "flag-minus-int", Args: []string{`%-10d|\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-minus-string", Args: []string{`%-10s|\n`, "hi"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-plus-positive", Args: []string{`%+d\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-plus-negative", Args: []string{`%+d\n`, "-42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-plus-float", Args: []string{`%+f\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-space-positive", Args: []string{`% d\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-space-negative", Args: []string{`% d\n`, "-42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-zero-pad", Args: []string{`%05d\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-zero-pad-float", Args: []string{`%010.2f\n`, "3.14"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-hash-octal", Args: []string{`%#o\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-hash-hex", Args: []string{`%#x\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-hash-hex-upper", Args: []string{`%#X\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-hash-hex-zero", Args: []string{`%#x\n`, "0"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-combined-plus-zero", Args: []string{`%+08d\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "flag-combined-minus-zero", Args: []string{`%-08d\n`, "42"}, Env: []string{"LC_ALL=C"}},

		// R2.3: '*' for width and precision
		{Name: "star-width", Args: []string{`%*d\n`, "10", "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "star-precision", Args: []string{`%.*f\n`, "3", "3.14159"}, Env: []string{"LC_ALL=C"}},
		{Name: "star-width-precision", Args: []string{`%*.*f\n`, "10", "3", "3.14159"}, Env: []string{"LC_ALL=C"}},
		{Name: "star-width-string", Args: []string{`%*s\n`, "10", "hi"}, Env: []string{"LC_ALL=C"}},
		{Name: "star-negative-width", Args: []string{`%*d|\n`, "-10", "42"}, Env: []string{"LC_ALL=C"}},

		// R2.4: '%%' literal percent
		{Name: "percent-literal", Args: []string{`100%%\n`}, Env: []string{"LC_ALL=C"}},
		{Name: "percent-with-directive", Args: []string{`%d%%\n`, "42"}, Env: []string{"LC_ALL=C"}},
		{Name: "percent-multiple", Args: []string{`%%-%%-%%\n`}, Env: []string{"LC_ALL=C"}},

		// R1.1: error cases with normalizer
		{Name: "d-non-numeric", Args: []string{`%d\n`, "abc"}, Env: []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normStderr}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
