// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd020-echo R1.1-R1.4, R2.1-R2.4:
// cmd/echo prints its arguments to stdout separated by spaces, followed by
// a newline. The -n flag suppresses the trailing newline. The -e flag enables
// backslash escape sequence interpretation. The -E flag disables it (default).
// Unrecognized flags are passed through as literal text. Installs SIGPIPE
// handler for clean exit on broken pipe.
package main

import (
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3 (prd): install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.4: GNU echo recognizes only -n, -e, -E and combinations thereof
	// (e.g., -nE, -neE) as flags. Any argument starting with '-' that
	// contains characters outside {n, e, E} is treated as a literal operand.
	// Flag parsing stops at the first non-flag argument.
	suppressNewline := false
	escapeEnabled := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		// Check every character after '-' is one of n, e, E.
		validFlag := true
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'n', 'e', 'E':
				// valid flag character
			default:
				validFlag = false
			}
		}
		if !validFlag {
			break
		}
		// R1.3: apply -n if present in this flag group.
		if strings.ContainsRune(arg, 'n') {
			suppressNewline = true
		}
		// R2.4: when both -e and -E appear, the last one wins.
		// Process characters left-to-right within each flag group.
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'e':
				escapeEnabled = true
			case 'E':
				escapeEnabled = false
			}
		}
		i++
	}

	// R1.1: print remaining arguments separated by single spaces.
	// R1.2: with no arguments, output is empty (newline handled below).
	output := strings.Join(args[i:], " ")

	// R2.1: interpret escape sequences when -e is active.
	if escapeEnabled {
		var stopped bool
		output, stopped = interpretEscapes(output)
		// R2.2: \c suppresses trailing newline and all further output.
		if stopped {
			suppressNewline = true
		}
	}

	// R1.1, R1.3: append newline unless -n was given.
	if !suppressNewline {
		output += "\n"
	}

	// R3.1, R3.2: exit 0 on success, >0 on write error.
	_, err := os.Stdout.WriteString(output)
	if err != nil {
		os.Exit(1)
	}
}

// interpretEscapes processes backslash escape sequences in s per R2.1-R2.3.
// Returns the interpreted string and whether \c was encountered (R2.2).
func interpretEscapes(s string) (string, bool) {
	var buf []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			buf = append(buf, s[i])
			continue
		}
		// Need at least one character after the backslash.
		if i+1 >= len(s) {
			buf = append(buf, '\\')
			continue
		}
		i++
		switch s[i] {
		case '\\':
			buf = append(buf, '\\')
		case 'a':
			buf = append(buf, 0x07)
		case 'b':
			buf = append(buf, 0x08)
		case 'c':
			// R2.2: suppress all further output including trailing newline.
			return string(buf), true
		case 'e':
			buf = append(buf, 0x1B)
		case 'f':
			buf = append(buf, 0x0C)
		case 'n':
			buf = append(buf, 0x0A)
		case 'r':
			buf = append(buf, 0x0D)
		case 't':
			buf = append(buf, 0x09)
		case 'v':
			buf = append(buf, 0x0B)
		case '0':
			// R2.3: \0NNN — up to three octal digits.
			val := byte(0)
			digits := 0
			for digits < 3 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
				val = val*8 + (s[i+1] - '0')
				i++
				digits++
			}
			buf = append(buf, val)
		case 'x':
			// R2.3: \xHH — up to two hex digits.
			if i+1 < len(s) && isHexDigit(s[i+1]) {
				val := hexVal(s[i+1])
				i++
				if i+1 < len(s) && isHexDigit(s[i+1]) {
					val = val*16 + hexVal(s[i+1])
					i++
				}
				buf = append(buf, val)
			} else {
				// No valid hex digit follows: emit literal \x.
				buf = append(buf, '\\', 'x')
			}
		default:
			// Unknown escape: emit backslash and the character literally.
			buf = append(buf, '\\', s[i])
		}
	}
	return string(buf), false
}

// isHexDigit reports whether c is a valid hexadecimal digit.
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// hexVal returns the numeric value of a hexadecimal digit.
func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}
