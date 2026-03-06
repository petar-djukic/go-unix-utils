// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the echo utility (prd020-echo R1-R4).
// Echo writes its arguments to stdout separated by spaces, with optional
// newline suppression (-n) and backslash escape interpretation (-e/-E).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3: Handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// D2/D3: Manual flag parsing. Only recognize -n, -e, -E (and clusters)
	// as leading arguments before the first non-flag argument.
	suppressNewline := false
	enableEscapes := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		valid := true
		for _, ch := range arg[1:] {
			if ch != 'n' && ch != 'e' && ch != 'E' {
				valid = false
				break
			}
		}
		if !valid {
			break
		}
		// R2.4: Last -e or -E wins.
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				suppressNewline = true
			case 'e':
				enableEscapes = true
			case 'E':
				enableEscapes = false
			}
		}
		i++
	}

	remaining := args[i:]
	output := strings.Join(remaining, " ")

	if enableEscapes {
		output = processEscapes(output)
	}

	// R3.1/R3.2: Exit 0 on success, 1 on write error.
	if !suppressNewline {
		output += "\n"
	}
	_, err := fmt.Fprint(os.Stdout, output)
	if err != nil {
		os.Exit(1)
	}
}

// processEscapes interprets backslash escape sequences per R2.1/R2.2.
// Returns the processed string. If \c is encountered, it returns everything
// up to that point and signals suppression of all further output by setting
// the trailing newline flag via the returned string (caller must not append
// newline after \c).
func processEscapes(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '\\' {
			buf.WriteByte(s[i])
			i++
			continue
		}
		// Backslash found.
		if i+1 >= len(s) {
			buf.WriteByte('\\')
			i++
			continue
		}
		next := s[i+1]
		switch next {
		case '\\':
			buf.WriteByte('\\')
			i += 2
		case 'a':
			buf.WriteByte(0x07)
			i += 2
		case 'b':
			buf.WriteByte(0x08)
			i += 2
		case 'c':
			// R2.2: Terminate output immediately.
			// Write what we have and exit now.
			fmt.Fprint(os.Stdout, buf.String())
			os.Exit(0)
		case 'e':
			buf.WriteByte(0x1B)
			i += 2
		case 'f':
			buf.WriteByte(0x0C)
			i += 2
		case 'n':
			buf.WriteByte(0x0A)
			i += 2
		case 'r':
			buf.WriteByte(0x0D)
			i += 2
		case 't':
			buf.WriteByte(0x09)
			i += 2
		case 'v':
			buf.WriteByte(0x0B)
			i += 2
		case '0':
			// R2.1: \0NNN — octal, 1-3 digits after the 0.
			i += 2
			val := byte(0)
			digits := 0
			for digits < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7' {
				val = val*8 + (s[i] - '0')
				i++
				digits++
			}
			buf.WriteByte(val)
		case 'x':
			// R2.1: \xHH — hex, 1-2 digits.
			i += 2
			val := byte(0)
			digits := 0
			for digits < 2 && i < len(s) {
				h := hexVal(s[i])
				if h < 0 {
					break
				}
				val = val*16 + byte(h)
				i++
				digits++
			}
			if digits == 0 {
				// No valid hex digits; output literal \x.
				buf.WriteByte('\\')
				buf.WriteByte('x')
			} else {
				buf.WriteByte(val)
			}
		default:
			// Unknown escape: output backslash and the character literally.
			buf.WriteByte('\\')
			buf.WriteByte(next)
			i += 2
		}
	}
	return buf.String()
}

// hexVal returns the numeric value of a hex digit, or -1 if not a hex digit.
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
