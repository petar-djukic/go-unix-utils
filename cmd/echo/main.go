// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/echo implements GNU echo: display a line of text.
// Implements prd020-echo R1.1-R1.4, R2.1-R2.4, R3.1-R3.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	noNewline, escapes, args := parseFlags(args)

	output, suppressNewline := buildOutput(args, escapes)

	if _, err := fmt.Fprint(os.Stdout, output); err != nil {
		os.Exit(1)
	}
	if !noNewline && !suppressNewline {
		if _, err := fmt.Fprintln(os.Stdout); err != nil {
			os.Exit(1)
		}
	}
}

// buildOutput joins args with spaces and optionally interprets escapes.
// R2.1: -e interprets backslash escape sequences.
// R2.2: \c terminates output; returns output up to \c and suppressNewline=true.
// R2.3: -E disables escape interpretation (default).
func buildOutput(args []string, escapes bool) (output string, suppressNewline bool) {
	joined := strings.Join(args, " ")
	if !escapes {
		return joined, false
	}
	return interpretEscapes(joined)
}

// parseFlags extracts recognized GNU echo flags from leading arguments.
// R1.3: -n suppresses trailing newline.
// R2.4: last of -e / -E wins.
// R1.4: Only -n, -e, -E (and combinations) are recognized flags.
func parseFlags(args []string) (noNewline, escapes bool, remaining []string) {
	i := 0
	for i < len(args) {
		if !isEchoFlag(args[i]) {
			break
		}
		for _, ch := range args[i][1:] {
			switch ch {
			case 'n':
				noNewline = true
			case 'e':
				escapes = true
			case 'E':
				escapes = false
			}
		}
		i++
	}
	return noNewline, escapes, args[i:]
}

// isEchoFlag returns true if arg is a recognized GNU echo flag string.
// A valid flag starts with '-' followed by one or more of 'n', 'e', 'E'.
func isEchoFlag(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	for _, ch := range arg[1:] {
		if ch != 'n' && ch != 'e' && ch != 'E' {
			return false
		}
	}
	return true
}

// interpretEscapes processes backslash escape sequences in s.
// R2.1: supports \\, \a, \b, \c, \e, \f, \n, \r, \t, \v, \0NNN, \xHH.
// Returns the processed string and whether \c was encountered.
func interpretEscapes(s string) (result string, terminated bool) {
	var buf strings.Builder
	buf.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '\\' || i+1 >= len(s) {
			buf.WriteByte(s[i])
			i++
			continue
		}
		ch, advance, stop := decodeEscape(s, i+1)
		if stop {
			result = buf.String()
			return result, true
		}
		buf.WriteByte(ch)
		i += 1 + advance
	}
	result = buf.String()
	return result, false
}

// decodeEscape decodes one escape sequence starting at s[pos].
// Returns the decoded byte, number of chars consumed after the backslash,
// and whether \c was encountered (terminate output).
func decodeEscape(s string, pos int) (byte, int, bool) {
	switch s[pos] {
	case '\\':
		return '\\', 1, false
	case 'a':
		return 0x07, 1, false
	case 'b':
		return 0x08, 1, false
	case 'c':
		return 0, 0, true
	case 'e':
		return 0x1B, 1, false
	case 'f':
		return 0x0C, 1, false
	case 'n':
		return 0x0A, 1, false
	case 'r':
		return 0x0D, 1, false
	case 't':
		return 0x09, 1, false
	case 'v':
		return 0x0B, 1, false
	case '0':
		return decodeOctal(s, pos)
	case 'x':
		return decodeHex(s, pos)
	default:
		return '\\', 0, false
	}
}

// decodeOctal parses \0NNN (1-3 octal digits after the '0').
func decodeOctal(s string, pos int) (byte, int, bool) {
	val := 0
	count := 0
	for j := pos + 1; j < len(s) && count < 3; j++ {
		d := s[j]
		if d < '0' || d > '7' {
			break
		}
		val = val*8 + int(d-'0')
		count++
	}
	return byte(val), 1 + count, false
}

// decodeHex parses \xHH (1-2 hex digits after 'x').
func decodeHex(s string, pos int) (byte, int, bool) {
	val := 0
	count := 0
	for j := pos + 1; j < len(s) && count < 2; j++ {
		d := hexVal(s[j])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	if count == 0 {
		return '\\', 0, false
	}
	return byte(val), 1 + count, false
}

// hexVal returns the numeric value of a hex digit, or -1 if not hex.
func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}
