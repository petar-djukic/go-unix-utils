// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd020-echo: Display a Line of Text.
// Covers R1.1-R1.4 (core output behavior), R2.1-R2.2 (escape sequences).
package main

import (
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	noNewline, escapes, args := parseFlags(args)

	output, stopped := buildOutput(args, escapes)
	if !noNewline && !stopped {
		output += "\n"
	}

	_, err := os.Stdout.WriteString(output)
	if err != nil {
		os.Exit(1)
	}
}

// parseFlags consumes leading GNU echo flags (-n, -e, -E) from args.
// GNU echo recognizes flags only when every character in the argument
// after the leading '-' is one of n, e, or E. The first argument that
// does not match this pattern ends flag parsing. D2: manual parsing,
// not the flag package. D3: last -e/-E wins.
func parseFlags(args []string) (bool, bool, []string) {
	noNewline := false
	escapes := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if !isValidFlagArg(arg) {
			break
		}
		for _, ch := range arg[1:] {
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

// isValidFlagArg reports whether s is a valid GNU echo flag argument.
// It must start with '-' and contain only the characters n, e, E after
// the dash. R1.4: unrecognized flags are treated as positional arguments.
func isValidFlagArg(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	for _, ch := range s[1:] {
		if ch != 'n' && ch != 'e' && ch != 'E' {
			return false
		}
	}
	return true
}

// buildOutput joins args with spaces, optionally interpreting escape
// sequences. R1.1: arguments joined by spaces. R1.2: no args yields
// empty string (caller appends newline). R2.1-R2.2: escape handling.
func buildOutput(args []string, escapes bool) (string, bool) {
	if !escapes {
		return strings.Join(args, " "), false
	}
	var b strings.Builder
	for i, arg := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		if stopped := expandEscapes(&b, arg); stopped {
			return b.String(), true
		}
	}
	return b.String(), false
}

// expandEscapes writes arg to b with backslash escape interpretation.
// Returns true if \c was encountered (R2.2: stop all further output).
func expandEscapes(b *strings.Builder, arg string) bool {
	i := 0
	for i < len(arg) {
		if arg[i] != '\\' || i+1 >= len(arg) {
			b.WriteByte(arg[i])
			i++
			continue
		}
		ch := arg[i+1]
		advance, stop := handleEscape(b, ch, arg[i+2:])
		if stop {
			return true
		}
		i += 2 + advance
	}
	return false
}

// handleEscape processes a single backslash escape character and any
// trailing digits. It writes the interpreted byte to b. Returns the
// number of extra bytes consumed beyond the escape character, and
// whether output should stop (\c). R2.1: all standard escapes.
func handleEscape(b *strings.Builder, ch byte, rest string) (int, bool) {
	switch ch {
	case '\\':
		b.WriteByte('\\')
	case 'a':
		b.WriteByte(0x07)
	case 'b':
		b.WriteByte(0x08)
	case 'c':
		return 0, true
	case 'e':
		b.WriteByte(0x1B)
	case 'f':
		b.WriteByte(0x0C)
	case 'n':
		b.WriteByte(0x0A)
	case 'r':
		b.WriteByte(0x0D)
	case 't':
		b.WriteByte(0x09)
	case 'v':
		b.WriteByte(0x0B)
	case '0':
		return parseOctal(b, rest), false
	case 'x':
		return parseHex(b, rest), false
	default:
		b.WriteByte('\\')
		b.WriteByte(ch)
	}
	return 0, false
}

// parseOctal reads up to 3 octal digits from rest after \0, writes
// the resulting byte to b, and returns how many extra bytes were consumed.
func parseOctal(b *strings.Builder, rest string) int {
	val := 0
	count := 0
	for count < 3 && count < len(rest) && rest[count] >= '0' && rest[count] <= '7' {
		val = val*8 + int(rest[count]-'0')
		count++
	}
	b.WriteByte(byte(val & 0xFF))
	return count
}

// parseHex reads up to 2 hex digits from rest after \x, writes the
// resulting byte to b, and returns how many extra bytes were consumed.
func parseHex(b *strings.Builder, rest string) int {
	val := 0
	count := 0
	for count < 2 && count < len(rest) {
		d := hexVal(rest[count])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	if count == 0 {
		b.WriteString("\\x")
		return 0
	}
	b.WriteByte(byte(val))
	return count
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
