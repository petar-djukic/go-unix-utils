// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/echo: display a line of text.
// Implements srd020-echo R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3: install SIGPIPE handler so echo exits 0 when pipe closes.
	sys.InstallSIGPIPEHandler()

	noNewline, escapes, args := parseFlags(os.Args[1:])

	output := buildOutput(args, noNewline, escapes)

	if _, err := os.Stdout.WriteString(output); err != nil {
		os.Exit(1)
	}
}

// parseFlags performs GNU echo-style flag parsing.
// R1.3: -n suppresses trailing newline.
// R2.3: -E disables escapes (default). R2.4: last of -e/-E wins.
// R1.4: only -n, -e, -E (and combinations) are recognized as flags.
// Unrecognized flags are treated as positional arguments.
// Flag parsing stops at the first non-flag argument.
func parseFlags(args []string) (noNewline, escapes bool, rest []string) {
	i := 0
	for i < len(args) {
		if !isValidFlagArg(args[i]) {
			break
		}
		for _, c := range args[i][1:] {
			switch c {
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

// isValidFlagArg checks whether an argument is a valid GNU echo flag group.
// A valid flag argument starts with '-' followed by one or more of [neE].
func isValidFlagArg(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	for _, c := range arg[1:] {
		if c != 'n' && c != 'e' && c != 'E' {
			return false
		}
	}
	return true
}

// buildOutput constructs the output string from positional arguments.
// R1.1: arguments joined by spaces, followed by newline.
// R1.2: no arguments produces only a newline.
// R1.3: noNewline suppresses the trailing newline.
// R2.1: escapes enables backslash interpretation.
// R2.2: \c terminates output immediately.
func buildOutput(args []string, noNewline, escapes bool) string {
	if !escapes {
		result := strings.Join(args, " ")
		if !noNewline {
			result += "\n"
		}
		return result
	}
	return buildEscapedOutput(args, noNewline)
}

// buildEscapedOutput handles output when -e is active.
// R2.1: interprets backslash escapes. R2.2: \c stops all output.
func buildEscapedOutput(args []string, noNewline bool) string {
	var buf strings.Builder
	for i, arg := range args {
		if i > 0 {
			buf.WriteByte(' ')
		}
		if interpretEscapes(&buf, arg) {
			return buf.String()
		}
	}
	if !noNewline {
		buf.WriteByte('\n')
	}
	return buf.String()
}

// interpretEscapes writes arg to buf with escape interpretation.
// Returns true if \c was encountered (R2.2: stop all further output).
func interpretEscapes(buf *strings.Builder, s string) bool {
	i := 0
	for i < len(s) {
		if s[i] != '\\' || i+1 >= len(s) {
			buf.WriteByte(s[i])
			i++
			continue
		}
		stop := handleEscape(buf, s, i)
		if stop < 0 {
			return true
		}
		i = stop
	}
	return false
}

// handleEscape processes escape at s[pos] (backslash).
// Returns the new index past the escape, or -1 for \c (stop output).
func handleEscape(buf *strings.Builder, s string, pos int) int {
	next := s[pos+1]
	switch next {
	case '\\':
		buf.WriteByte('\\')
	case 'a':
		buf.WriteByte(0x07)
	case 'b':
		buf.WriteByte(0x08)
	case 'c':
		return -1
	case 'e':
		buf.WriteByte(0x1B)
	case 'f':
		buf.WriteByte(0x0C)
	case 'n':
		buf.WriteByte(0x0A)
	case 'r':
		buf.WriteByte(0x0D)
	case 't':
		buf.WriteByte(0x09)
	case 'v':
		buf.WriteByte(0x0B)
	case '0':
		return parseOctal(buf, s, pos+2)
	case 'x':
		return parseHex(buf, s, pos+2)
	default:
		buf.WriteByte('\\')
		buf.WriteByte(next)
	}
	return pos + 2
}

// parseOctal reads up to 3 octal digits starting at pos, writes the byte.
// \0NNN format: the '0' is already consumed by handleEscape.
func parseOctal(buf *strings.Builder, s string, pos int) int {
	val := 0
	count := 0
	for i := pos; i < len(s) && count < 3; i++ {
		d := s[i]
		if d < '0' || d > '7' {
			break
		}
		val = val*8 + int(d-'0')
		count++
	}
	buf.WriteByte(byte(val & 0xFF))
	return pos + count
}

// parseHex reads up to 2 hex digits starting at pos, writes the byte.
// \xHH format: the 'x' is already consumed by handleEscape.
func parseHex(buf *strings.Builder, s string, pos int) int {
	val := 0
	count := 0
	for i := pos; i < len(s) && count < 2; i++ {
		d := hexVal(s[i])
		if d < 0 {
			break
		}
		val = val*16 + d
		count++
	}
	if count == 0 {
		buf.WriteString("\\x")
		return pos
	}
	buf.WriteByte(byte(val))
	return pos + count
}

// hexVal returns the numeric value of a hex digit, or -1.
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
