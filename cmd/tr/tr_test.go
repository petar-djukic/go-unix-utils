// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd054-tr R1.1-R1.4, R2.1-R2.4, R3.1-R3.3, R4.1, R4.2, R4.3 (differential tests)
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// trErrRe matches tr/gtr error prefix and normalizes the program name.
var trErrRe = regexp.MustCompile(`(?m)^g?tr: `)

// trTryRe matches the "Try '...(g)tr --help'" line and normalizes the
// program name and path.
var trTryRe = regexp.MustCompile(`(?m)Try '[^']*g?tr --help'`)

// normalizeTrErrors replaces the program name prefix in error messages so
// that "gtr: ..." and "tr: ..." compare equal.
func normalizeTrErrors(b []byte) []byte {
	b = trErrRe.ReplaceAll(b, []byte("tr: "))
	b = trTryRe.ReplaceAll(b, []byte("Try 'tr --help'"))
	return b
}

// TestDiff runs differential tests comparing the Go tr binary against the
// GNU reference binary (gtr) for R1.1-R1.4 scenarios.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtr")
	if err != nil {
		t.Skipf("reference binary gtr not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Basic character translation.
		{
			Name:  "translate single char",
			Args:  []string{"e", "a"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "translate multiple chars",
			Args:  []string{"helo", "HELO"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "translate identity",
			Args:  []string{"abc", "abc"},
			Stdin: []byte("abc\n"),
		},

		// R1.1, R1.3: Character range translation.
		{
			Name:  "lowercase to uppercase range",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "uppercase to lowercase range",
			Args:  []string{"A-Z", "a-z"},
			Stdin: []byte("HELLO WORLD\n"),
		},
		{
			Name:  "digit range translation",
			Args:  []string{"0-9", "a-j"},
			Stdin: []byte("0123456789\n"),
		},
		{
			Name:  "partial range",
			Args:  []string{"a-f", "A-F"},
			Stdin: []byte("abcdefgh\n"),
		},

		// R1.1: SET2 shorter than SET1 -- extend with last char.
		{
			Name:  "set2 shorter extends with last char",
			Args:  []string{"abcdef", "xy"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:  "set2 single char maps all",
			Args:  []string{"abc", "x"},
			Stdin: []byte("aabbcc\n"),
		},
		{
			Name:  "set2 shorter with ranges",
			Args:  []string{"a-z", "X"},
			Stdin: []byte("hello\n"),
		},

		// R1.3: Escape sequences.
		{
			Name:  "escape newline in set1",
			Args:  []string{`\n`, "X"},
			Stdin: []byte("line1\nline2\n"),
		},
		{
			Name:  "escape tab in set1",
			Args:  []string{`\t`, " "},
			Stdin: []byte("col1\tcol2\n"),
		},
		{
			Name:  "escape backslash",
			Args:  []string{`\\`, "X"},
			Stdin: []byte("a\\b\\c\n"),
		},
		{
			Name:  "octal escape",
			Args:  []string{`\141`, "X"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "octal escape 012 is newline",
			Args:  []string{`\012`, "X"},
			Stdin: []byte("a\nb\n"),
		},

		// R1.4: POSIX character classes.
		{
			Name:  "class lower to upper",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "class upper to lower",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("HELLO WORLD\n"),
		},
		{
			Name:  "class digit translation",
			Args:  []string{"[:digit:]", "XXXXXXXXXX"},
			Stdin: []byte("abc123def\n"),
		},

		// R1.2: Empty input.
		{
			Name:  "empty input",
			Args:  []string{"a", "b"},
			Stdin: []byte(""),
		},

		// R1.3: Mixed individual chars and ranges.
		{
			Name:  "mixed chars and range",
			Args:  []string{"aeiou", "AEIOU"},
			Stdin: []byte("hello world\n"),
		},

		// Binary data passthrough.
		{
			Name:  "non-matching chars pass through",
			Args:  []string{"x", "y"},
			Stdin: []byte("hello\n"),
		},

		// Multiple lines.
		{
			Name:  "multiple lines",
			Args:  []string{"a-z", "A-Z"},
			Stdin: []byte("line one\nline two\nline three\n"),
		},

		// R2.1: POSIX character classes -- all classes under LC_ALL=C.
		{
			Name:  "r2.1 class alpha to digits",
			Args:  []string{"[:alpha:]", "[0*]"},
			Stdin: []byte("abc123XYZ\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class digit delete via translate",
			Args:  []string{"[:digit:]", "XXXXXXXXXX"},
			Stdin: []byte("abc123def456\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class alnum",
			Args:  []string{"[:alnum:]", "[X*]"},
			Stdin: []byte("abc 123 !@#\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class blank",
			Args:  []string{"[:blank:]", "X"},
			Stdin: []byte("hello\tworld here\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class space",
			Args:  []string{"[:space:]", "[X*]"},
			Stdin: []byte("a b\tc\nd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class punct",
			Args:  []string{"[:punct:]", "[X*]"},
			Stdin: []byte("hello, world! foo@bar.\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class graph",
			Args:  []string{"[:graph:]", "[X*]"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class print",
			Args:  []string{"[:print:]", "[X*]"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class xdigit",
			Args:  []string{"[:xdigit:]", "[X*]"},
			Stdin: []byte("0123456789abcdefABCDEFGHIJ\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class cntrl replace with X",
			Args:  []string{"[:cntrl:]", "[X*]"},
			Stdin: []byte("hello\x01\x02\x03world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class lower to upper",
			Args:  []string{"[:lower:]", "[:upper:]"},
			Stdin: []byte("Hello World 123\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1 class upper to lower",
			Args:  []string{"[:upper:]", "[:lower:]"},
			Stdin: []byte("Hello World 123\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.2: Backslash escape sequences.
		{
			Name:  "r2.2 escape alert",
			Args:  []string{`\a`, "X"},
			Stdin: []byte("a\ab\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape backspace",
			Args:  []string{`\b`, "X"},
			Stdin: []byte("a\bb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape form feed",
			Args:  []string{`\f`, "X"},
			Stdin: []byte("a\fb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape newline",
			Args:  []string{`\n`, "X"},
			Stdin: []byte("line1\nline2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape carriage return",
			Args:  []string{`\r`, "X"},
			Stdin: []byte("line1\rline2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape tab",
			Args:  []string{`\t`, "X"},
			Stdin: []byte("col1\tcol2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape vertical tab",
			Args:  []string{`\v`, "X"},
			Stdin: []byte("a\vb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape backslash",
			Args:  []string{`\\`, "X"},
			Stdin: []byte("a\\b\\c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 octal escape 141 is a",
			Args:  []string{`\141`, "X"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 octal escape 012 is newline",
			Args:  []string{`\012`, "X"},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 octal escape 011 is tab",
			Args:  []string{`\011`, " "},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 octal escape 000 is null",
			Args:  []string{`\000`, "X"},
			Stdin: []byte("a\x00b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2 escape in set2",
			Args:  []string{"X", `\n`},
			Stdin: []byte("aXbXc\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.3: Character ranges.
		{
			Name:  "r2.3 range a-f",
			Args:  []string{"a-f", "A-F"},
			Stdin: []byte("abcdefghij\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.3 range 0-9",
			Args:  []string{"0-9", "a-j"},
			Stdin: []byte("0123456789abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.3 range A-Z",
			Args:  []string{"A-Z", "a-z"},
			Stdin: []byte("HELLO WORLD\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.3 range single char a-a",
			Args:  []string{"a-a", "X"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:      "r2.3 invalid range z-a",
			Args:      []string{"z-a", "A-Z"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTrErrors},
		},
		{
			Name:      "r2.3 invalid range 9-0",
			Args:      []string{"9-0", "a-j"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTrErrors},
		},
		{
			Name:  "r2.3 range with escape end",
			Args:  []string{`a-\172`, "A-Z"},
			Stdin: []byte("abcxyz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.3 multiple ranges",
			Args:  []string{"a-zA-Z", "A-Za-z"},
			Stdin: []byte("Hello World\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.4: Repeat notation [c*n].
		{
			Name:  "r2.4 repeat explicit count",
			Args:  []string{"abcde", "[x*5]"},
			Stdin: []byte("abcde\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.4 repeat fill star",
			Args:  []string{"a-z", "[X*]"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.4 repeat fill star zero",
			Args:  []string{"a-z", "[X*0]"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.4 repeat with prefix",
			Args:  []string{"abcdef", "xy[z*]"},
			Stdin: []byte("abcdef\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.4 repeat octal count",
			Args:  []string{"abcdefgh", "[X*010]"},
			Stdin: []byte("abcdefgh\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.4 repeat count 1",
			Args:  []string{"abc", "[X*1]YZ"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R3.1: Delete mode (-d).
		{
			Name:  "r3.1 delete single char",
			Args:  []string{"-d", "l"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete multiple chars",
			Args:  []string{"-d", "aeiou"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete with range",
			Args:  []string{"-d", "a-z"},
			Stdin: []byte("Hello World 123\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete digits",
			Args:  []string{"-d", "[:digit:]"},
			Stdin: []byte("hello 123 world 456\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete newline",
			Args:  []string{"-d", `\n`},
			Stdin: []byte("line1\nline2\nline3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete no match",
			Args:  []string{"-d", "xyz"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete all chars",
			Args:  []string{"-d", "[:print:][:cntrl:]"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete --delete long flag",
			Args:  []string{"--delete", "l"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.1 delete empty input",
			Args:  []string{"-d", "abc"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},

		// R3.2: Squeeze mode (-s).
		{
			Name:  "r3.2 squeeze single set",
			Args:  []string{"-s", "a-z"},
			Stdin: []byte("aabbcc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze spaces",
			Args:  []string{"-s", " "},
			Stdin: []byte("hello   world   foo\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze newlines",
			Args:  []string{"-s", `\n`},
			Stdin: []byte("a\n\n\nb\n\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze no repeats",
			Args:  []string{"-s", "abc"},
			Stdin: []byte("abcabc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze with translate",
			Args:  []string{"-s", "a-z", "A-Z"},
			Stdin: []byte("aabbcc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze translate lower to upper",
			Args:  []string{"-s", "[:lower:]", "[:upper:]"},
			Stdin: []byte("aabbbccdd\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze --squeeze-repeats long flag",
			Args:  []string{"--squeeze-repeats", "a-c"},
			Stdin: []byte("aabbcc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze empty input",
			Args:  []string{"-s", "abc"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze only matching chars",
			Args:  []string{"-s", "a"},
			Stdin: []byte("baaab\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.2 squeeze translate set2 shorter",
			Args:  []string{"-s", "abc", "x"},
			Stdin: []byte("aabbcc\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R3.3: Combined delete-squeeze (-ds).
		{
			Name:  "r3.3 delete squeeze basic",
			Args:  []string{"-ds", "aeiou", "a-z"},
			Stdin: []byte("aabbbcddee hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.3 delete squeeze digits and spaces",
			Args:  []string{"-ds", "[:digit:]", " "},
			Stdin: []byte("abc 123 def  456  ghi\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.3 delete squeeze separate flags",
			Args:  []string{"-d", "-s", "aeiou", "a-z"},
			Stdin: []byte("aabbbcddee hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.3 delete squeeze reverse flag order",
			Args:  []string{"-sd", "aeiou", "a-z"},
			Stdin: []byte("aabbbcddee hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.3 delete squeeze empty after delete",
			Args:  []string{"-ds", "a-z", " "},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r3.3 delete squeeze no squeeze needed",
			Args:  []string{"-ds", "x", "abc"},
			Stdin: []byte("abcxabc\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R4.1: Successful exit code (0) -- covered implicitly by all tests above
		// with ExitCode defaulting to 0. These tests explicitly document R4.1.
		{
			Name:  "r4.1 translate exits 0",
			Args:  []string{"a", "b"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.1 delete exits 0",
			Args:  []string{"-d", "a"},
			Stdin: []byte("abc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.1 squeeze exits 0",
			Args:  []string{"-s", "a"},
			Stdin: []byte("aaa\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R4.2: Usage error exit code (1).
		{
			Name:      "r4.2 missing operand",
			Args:      []string{},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTrErrors},
		},
		{
			Name:      "r4.2 translate missing set2",
			Args:      []string{"abc"},
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTrErrors},
		},
		{
			Name:      "r4.2 invalid character class",
			Args:      []string{"[:bogus:]", "x"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTrErrors},
		},
		{
			Name:      "r4.2 delete with extra operand",
			Args:      []string{"-d", "a", "b"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTrErrors},
		},
		{
			Name:      "r4.2 extra operand in translate",
			Args:      []string{"a", "b", "c"},
			Stdin:     []byte("hello\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTrErrors},
		},

		// R4.3, R2.4: Complement mode (-c/-C/--complement).
		{
			Name:  "r4.3 complement translate replaces non-alpha",
			Args:  []string{"-c", "a-z\\n", "*"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement translate uppercase flag",
			Args:  []string{"-C", "a-z\\n", "*"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement long flag",
			Args:  []string{"--complement", "a-z\\n", "*"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement delete non-alpha",
			Args:  []string{"-cd", "a-z"},
			Stdin: []byte("hello 123 world!\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement delete keep digits",
			Args:  []string{"-cd", "[:digit:]\\n"},
			Stdin: []byte("abc 123 def 456\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement squeeze non-alpha",
			Args:  []string{"-cs", "a-z", "\\n"},
			Stdin: []byte("hello 123 world 456 foo\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement delete squeeze",
			Args:  []string{"-cds", "[:alpha:]", "\\n"},
			Stdin: []byte("hello   world\n123\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement translate non-alpha to star",
			Args:  []string{"-c", "[:alpha:]\\n", "*"},
			Stdin: []byte("abc 123 def\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement with class lower",
			Args:  []string{"-cd", "[:lower:]"},
			Stdin: []byte("Hello World 123\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement empty input",
			Args:  []string{"-c", "abc", "X"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r4.3 complement all chars in set",
			Args:  []string{"-c", "a-z", "X"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
