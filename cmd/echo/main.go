// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the echo utility for displaying a line of text.
//
// Implements prd020-echo: core output behavior (R1), escape sequence
// interpretation (R2), exit codes and SIGPIPE (R3).
package main

import (
	"bufio"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// Parse flags. GNU echo only recognizes -n, -e, -E as flags when they
	// appear as complete arguments consisting solely of these flag characters.
	// It does not use -- to end flags. Once a non-flag argument is seen, all
	// remaining arguments (including those starting with -) are positional.
	suppressNewline := false
	escapeEnabled := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		valid := true
		n, e, bigE := false, false, false
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				n = true
			case 'e':
				e = true
			case 'E':
				bigE = true
			default:
				valid = false
			}
		}
		if !valid {
			break
		}
		if n {
			suppressNewline = true
		}
		// R2.4: last of -e/-E wins, processed left to right within the arg.
		if e || bigE {
			for _, ch := range arg[1:] {
				switch ch {
				case 'e':
					escapeEnabled = true
				case 'E':
					escapeEnabled = false
				}
			}
		}
		i++
	}

	positional := args[i:]

	w := bufio.NewWriter(os.Stdout)

	for idx, arg := range positional {
		if idx > 0 {
			w.WriteByte(' ')
		}
		if escapeEnabled {
			if writeEscaped(w, arg) {
				// \c encountered: stop all output.
				w.Flush()
				os.Exit(0)
			}
		} else {
			w.WriteString(arg)
		}
	}

	if !suppressNewline {
		w.WriteByte('\n')
	}

	w.Flush()
}

// writeEscaped writes arg to w with backslash escape interpretation.
// Returns true if \c was encountered (caller must stop all output).
func writeEscaped(w *bufio.Writer, arg string) bool {
	for i := 0; i < len(arg); i++ {
		if arg[i] != '\\' || i+1 >= len(arg) {
			w.WriteByte(arg[i])
			continue
		}
		i++
		switch arg[i] {
		case '\\':
			w.WriteByte('\\')
		case 'a':
			w.WriteByte(0x07)
		case 'b':
			w.WriteByte(0x08)
		case 'c':
			return true
		case 'e':
			w.WriteByte(0x1B)
		case 'f':
			w.WriteByte(0x0C)
		case 'n':
			w.WriteByte(0x0A)
		case 'r':
			w.WriteByte(0x0D)
		case 't':
			w.WriteByte(0x09)
		case 'v':
			w.WriteByte(0x0B)
		case '0':
			// R2.1: \0NNN octal, 1-3 digits after the 0.
			val := byte(0)
			digits := 0
			for digits < 3 && i+1 < len(arg) && arg[i+1] >= '0' && arg[i+1] <= '7' {
				i++
				val = val*8 + (arg[i] - '0')
				digits++
			}
			w.WriteByte(val)
		case 'x':
			// R2.1: \xHH hex, 1-2 digits.
			if i+1 < len(arg) && isHexDigit(arg[i+1]) {
				i++
				val := hexVal(arg[i])
				if i+1 < len(arg) && isHexDigit(arg[i+1]) {
					i++
					val = val*16 + hexVal(arg[i])
				}
				w.WriteByte(val)
			} else {
				// No hex digits follow: write \x literally.
				w.WriteByte('\\')
				w.WriteByte('x')
			}
		default:
			// Unrecognized escape: write backslash and the character literally.
			w.WriteByte('\\')
			w.WriteByte(arg[i])
		}
	}
	return false
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func hexVal(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	default:
		return b - 'A' + 10
	}
}
