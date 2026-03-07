// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU echo: display a line of text.
// Implements prd020-echo R1-R3.
package main

import (
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2: Parse flags manually. GNU echo only recognizes flags before the first
	// non-flag argument. Flags are any combination of -n, -e, -E in a single
	// dash-prefixed token (e.g., -neE, -nE). An argument that starts with '-'
	// but contains any character other than n, e, E is not a flag.
	suppressNewline := false
	escapes := false
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
				escapes = true
			case 'E':
				escapes = false
			}
		}
		i++
	}

	// Build output.
	var buf []byte
	for j := i; j < len(args); j++ {
		if j > i {
			buf = append(buf, ' ')
		}
		if escapes {
			buf = appendEscaped(buf, args[j])
			if len(buf) > 0 && buf[len(buf)-1] == 0xFF {
				// R2.2: \c sentinel — truncate and stop.
				buf = buf[:len(buf)-1]
				write(buf)
				return
			}
		} else {
			buf = append(buf, args[j]...)
		}
	}

	if !suppressNewline {
		buf = append(buf, '\n')
	}

	write(buf)
}

// write writes buf to stdout and exits 1 on error (R3.2).
func write(buf []byte) {
	_, err := os.Stdout.Write(buf)
	if err != nil {
		os.Exit(1)
	}
}

// appendEscaped appends s to buf with backslash escape interpretation (R2.1).
// If \c is encountered, a sentinel byte 0xFF is appended and the function returns.
func appendEscaped(buf []byte, s string) []byte {
	for k := 0; k < len(s); k++ {
		if s[k] != '\\' {
			buf = append(buf, s[k])
			continue
		}
		if k+1 >= len(s) {
			buf = append(buf, '\\')
			continue
		}
		k++
		switch s[k] {
		case '\\':
			buf = append(buf, '\\')
		case 'a':
			buf = append(buf, 0x07)
		case 'b':
			buf = append(buf, 0x08)
		case 'c':
			// R2.2: Sentinel for immediate termination.
			buf = append(buf, 0xFF)
			return buf
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
			// \0NNN — octal, 1-3 digits after the '0'.
			val := byte(0)
			digits := 0
			for digits < 3 && k+1 < len(s) && s[k+1] >= '0' && s[k+1] <= '7' {
				k++
				val = val*8 + (s[k] - '0')
				digits++
			}
			buf = append(buf, val)
		case 'x':
			// \xHH — hex, 1-2 digits.
			if k+1 >= len(s) || !isHexDigit(s[k+1]) {
				// No valid hex digit follows; output literal \x.
				buf = append(buf, '\\', 'x')
				continue
			}
			val := byte(0)
			digits := 0
			for digits < 2 && k+1 < len(s) && isHexDigit(s[k+1]) {
				k++
				val = val*16 + hexVal(s[k])
				digits++
			}
			buf = append(buf, val)
		default:
			// Unknown escape — output backslash and the character literally.
			buf = append(buf, '\\', s[k])
		}
	}
	return buf
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
