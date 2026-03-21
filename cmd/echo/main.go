// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd020-echo R1.1–R1.4, R2.1–R2.4, R3.1–R3.3, R4.1–R4.3.
// R1.1: arguments joined by spaces, followed by newline.
// R1.2: no arguments outputs only a newline.
// R1.3: -n suppresses trailing newline.
// R1.4: unrecognized flags are passed through as literal text.
// R2.1: -e enables backslash escape interpretation.
// R2.2: \c with -e terminates output immediately.
// R2.3: -E disables escape interpretation (default).
// R2.4: last of -e/-E on command line wins.
package main

import (
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

// run parses flags and writes output, returning the exit code.
func run(args []string, w *os.File) int {
	noNewline, escapes, remaining := parseFlags(args)
	output, truncated := buildOutput(remaining, escapes)
	if truncated {
		noNewline = true // R2.2: \c suppresses trailing newline
	}
	if err := writeOutput(w, output, noNewline); err != nil {
		return 1
	}
	return 0
}

// writeOutput writes the output string, optionally appending a newline.
func writeOutput(w *os.File, output string, noNewline bool) error {
	if noNewline {
		_, err := w.WriteString(output)
		return err
	}
	_, err := w.WriteString(output + "\n")
	return err
}

// buildOutput joins arguments with spaces and applies escape processing.
// Returns the output string and whether \c truncated it (R2.2).
func buildOutput(args []string, escapes bool) (string, bool) {
	joined := strings.Join(args, " ")
	if !escapes {
		return joined, false
	}
	return processEscapes(joined)
}

// processEscapes interprets backslash escape sequences in s per R2.1.
// Returns the processed string and whether \c was encountered (R2.2).
func processEscapes(s string) (string, bool) {
	var buf strings.Builder
	buf.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '\\' || i+1 >= len(s) {
			buf.WriteByte(s[i])
			i++
			continue
		}
		advance := handleEscape(s, i, &buf)
		if advance < 0 {
			return buf.String(), true // \c: truncate
		}
		i += advance
	}
	return buf.String(), false
}

// handleEscape processes a single escape sequence starting at s[pos].
// Returns the number of bytes consumed, or -1 for \c (truncate).
func handleEscape(s string, pos int, buf *strings.Builder) int {
	ch := s[pos+1]
	switch ch {
	case '\\':
		buf.WriteByte('\\')
	case 'a':
		buf.WriteByte(0x07)
	case 'b':
		buf.WriteByte(0x08)
	case 'c':
		return -1 // R2.2: suppress all further output
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
		return handleOctal(s, pos, buf)
	case 'x':
		return handleHex(s, pos, buf)
	default:
		buf.WriteByte('\\')
		buf.WriteByte(ch)
	}
	return 2
}

// handleOctal processes \0NNN octal escapes (1-3 octal digits after \0).
func handleOctal(s string, pos int, buf *strings.Builder) int {
	start := pos + 2
	val := byte(0)
	digits := 0
	for i := start; i < len(s) && i < start+3; i++ {
		if s[i] < '0' || s[i] > '7' {
			break
		}
		val = val*8 + (s[i] - '0')
		digits++
	}
	buf.WriteByte(val)
	return 2 + digits
}

// handleHex processes \xHH hex escapes (1-2 hex digits after \x).
func handleHex(s string, pos int, buf *strings.Builder) int {
	start := pos + 2
	val := byte(0)
	digits := 0
	for i := start; i < len(s) && i < start+2; i++ {
		d, ok := hexDigit(s[i])
		if !ok {
			break
		}
		val = val*16 + d
		digits++
	}
	if digits == 0 {
		buf.WriteByte('\\')
		buf.WriteByte('x')
		return 2
	}
	buf.WriteByte(val)
	return 2 + digits
}

// hexDigit returns the numeric value of a hex digit and true, or 0 and false.
func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// parseFlags extracts leading GNU echo flags from the argument list.
// GNU echo recognizes flags that consist entirely of the characters n, e, E.
// R1.3: -n suppresses the trailing newline.
// R2.4: last of -e/-E on command line wins.
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

// isEchoFlag returns true if arg is a valid GNU echo flag string.
// Valid flags are "-" followed by one or more characters from {n, e, E}.
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
