// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/echo implements the echo command.
// Implements: prd020-echo R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3: Handle SIGPIPE gracefully per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3, R1.4, R2.3, R2.4: Parse leading flag arguments. GNU echo recognizes
	// -n, -e, -E and combinations like -neE. Once a non-flag argument is
	// encountered, all remaining arguments (including later dash-prefixed ones)
	// are positional. When both -e and -E appear, the last one wins.
	suppressNewline := false
	enableEscapes := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		valid := true
		for _, c := range arg[1:] {
			if c != 'n' && c != 'e' && c != 'E' {
				valid = false
				break
			}
		}
		if !valid {
			break
		}
		// R2.4: Process flags left-to-right; last -e or -E wins.
		for _, c := range arg[1:] {
			switch c {
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

	// R1.1: Write arguments separated by spaces.
	// R1.2: No arguments produces empty output (newline appended below).
	output := strings.Join(args[i:], " ")

	// R2.1: When -e is active, interpret backslash escape sequences.
	if enableEscapes {
		var stop bool
		output, stop = interpretEscapes(output)
		// R2.2: \c suppresses trailing newline and all further output.
		if stop {
			suppressNewline = true
		}
	}

	if !suppressNewline {
		output += "\n"
	}

	// R1.1, R3.1, R3.2: Write to stdout; exit 1 on write failure.
	_, err := fmt.Fprint(os.Stdout, output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "echo: write error: %v\n", err)
		os.Exit(1)
	}
}

// interpretEscapes processes backslash escape sequences in s per prd020 R2.1.
// Returns the processed string and a boolean indicating whether \c was
// encountered (meaning all further output should be suppressed).
func interpretEscapes(s string) (string, bool) {
	var buf []byte
	for j := 0; j < len(s); j++ {
		if s[j] != '\\' {
			buf = append(buf, s[j])
			continue
		}
		// Backslash at end of string: emit literal backslash.
		if j+1 >= len(s) {
			buf = append(buf, '\\')
			continue
		}
		j++
		switch s[j] {
		case '\\':
			buf = append(buf, '\\')
		case 'a':
			buf = append(buf, 0x07)
		case 'b':
			buf = append(buf, 0x08)
		case 'c':
			// R2.2: Suppress all further output including trailing newline.
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
			// R2.1: \0NNN — octal value, 1-3 digits after \0.
			val := byte(0)
			digits := 0
			for digits < 3 && j+1+digits < len(s) && s[j+1+digits] >= '0' && s[j+1+digits] <= '7' {
				val = val*8 + (s[j+1+digits] - '0')
				digits++
			}
			j += digits
			buf = append(buf, val)
		case 'x':
			// R2.1: \xHH — hex value, 1-2 hex digits.
			val := byte(0)
			digits := 0
			for digits < 2 && j+1+digits < len(s) {
				c := s[j+1+digits]
				var nibble byte
				switch {
				case c >= '0' && c <= '9':
					nibble = c - '0'
				case c >= 'a' && c <= 'f':
					nibble = c - 'a' + 10
				case c >= 'A' && c <= 'F':
					nibble = c - 'A' + 10
				default:
					goto hexDone
				}
				val = val*16 + nibble
				digits++
			}
		hexDone:
			if digits > 0 {
				j += digits
				buf = append(buf, val)
			} else {
				// No valid hex digits: emit \x literally.
				buf = append(buf, '\\', 'x')
			}
		default:
			// Unknown escape: emit backslash and the character literally.
			buf = append(buf, '\\', s[j])
		}
	}
	return string(buf), false
}
