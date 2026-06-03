// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd020-echo R1.1-R1.4, R2.1-R2.4, R3.1-R3.3.
package main

import (
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	noNewline := false
	escapeMode := false

	i := 0
	for i < len(args) {
		arg := args[i]
		if !isFlag(arg) {
			break
		}
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				noNewline = true
			case 'e':
				escapeMode = true
			case 'E':
				escapeMode = false
			}
		}
		i++
	}

	operands := args[i:]
	output := strings.Join(operands, " ")

	truncated := false
	if escapeMode {
		output, truncated = interpretEscapes(output)
	}

	if !noNewline && !truncated {
		output += "\n"
	}

	_, err := os.Stdout.WriteString(output)
	if err != nil {
		os.Exit(1)
	}
}

// isFlag returns true if the argument is a recognized echo flag cluster:
// it must start with '-' followed by one or more characters from {n, e, E}.
func isFlag(arg string) bool {
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

func interpretEscapes(s string) (string, bool) {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		i++
		switch s[i] {
		case '\\':
			b.WriteByte('\\')
		case 'a':
			b.WriteByte(0x07)
		case 'b':
			b.WriteByte(0x08)
		case 'c':
			return b.String(), true
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
			val := byte(0)
			j := i + 1
			for k := 0; k < 3 && j < len(s) && s[j] >= '0' && s[j] <= '7'; k++ {
				val = val*8 + (s[j] - '0')
				j++
			}
			b.WriteByte(val)
			i = j
			continue
		case 'x':
			j := i + 1
			val := byte(0)
			digits := 0
			for digits < 2 && j < len(s) {
				ch := s[j]
				var nib byte
				switch {
				case ch >= '0' && ch <= '9':
					nib = ch - '0'
				case ch >= 'a' && ch <= 'f':
					nib = ch - 'a' + 10
				case ch >= 'A' && ch <= 'F':
					nib = ch - 'A' + 10
				default:
					goto hexDone
				}
				val = val*16 + nib
				digits++
				j++
			}
		hexDone:
			if digits == 0 {
				b.WriteByte('\\')
				b.WriteByte('x')
			} else {
				b.WriteByte(val)
			}
			i = j
			continue
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
		i++
	}
	return b.String(), false
}
