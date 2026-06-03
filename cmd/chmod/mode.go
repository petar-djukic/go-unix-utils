// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"
)

type permBits struct {
	read   bool
	write  bool
	exec   bool
	execX  bool
	setuid bool
	setgid bool
	sticky bool
}

func evalSymbolic(spec string, current os.FileMode, umask os.FileMode) (os.FileMode, error) {
	perm := current.Perm() | (current & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky))
	for clause := range strings.SplitSeq(spec, ",") {
		var err error
		perm, err = evalClause(clause, perm, umask)
		if err != nil {
			return 0, err
		}
	}
	return perm, nil
}

func evalClause(clause string, perm os.FileMode, umask os.FileMode) (os.FileMode, error) {
	i := 0
	who := parseWho(clause, &i)
	if i >= len(clause) {
		return 0, fmt.Errorf("invalid mode: %q", clause)
	}
	for i < len(clause) {
		op := clause[i]
		if op != '+' && op != '-' && op != '=' {
			return 0, fmt.Errorf("invalid mode: %q", clause)
		}
		i++
		bits := parsePerms(clause, &i)
		perm = applyOp(perm, who, op, bits, umask)
	}
	return perm, nil
}

func parseWho(clause string, pos *int) string {
	start := *pos
	for *pos < len(clause) {
		c := clause[*pos]
		if c == 'u' || c == 'g' || c == 'o' || c == 'a' {
			*pos++
		} else {
			break
		}
	}
	return clause[start:*pos]
}

func parsePerms(clause string, pos *int) permBits {
	var bits permBits
	for *pos < len(clause) {
		c := clause[*pos]
		switch c {
		case 'r':
			bits.read = true
		case 'w':
			bits.write = true
		case 'x':
			bits.exec = true
		case 'X':
			bits.execX = true
		case 's':
			bits.setuid = true
			bits.setgid = true
		case 't':
			bits.sticky = true
		default:
			return bits
		}
		*pos++
	}
	return bits
}

func applyOp(perm os.FileMode, who string, op byte, bits permBits, umask os.FileMode) os.FileMode {
	implicit := who == ""
	if implicit || strings.ContainsRune(who, 'a') {
		who = "ugo"
	}

	var mask os.FileMode
	for _, w := range who {
		mask |= buildMask(w, bits, perm)
	}

	if implicit {
		mask &^= umask
	}

	switch op {
	case '+':
		perm |= mask
	case '-':
		perm &^= mask
	case '=':
		perm = applyEquals(perm, who, mask)
	}
	return perm
}

func buildMask(who rune, bits permBits, current os.FileMode) os.FileMode {
	var rBit, wBit, xBit os.FileMode
	var specialBit os.FileMode
	useSpecial := false

	switch who {
	case 'u':
		rBit, wBit, xBit = 0400, 0200, 0100
		specialBit = os.ModeSetuid
		useSpecial = bits.setuid || bits.setgid
	case 'g':
		rBit, wBit, xBit = 0040, 0020, 0010
		specialBit = os.ModeSetgid
		useSpecial = bits.setuid || bits.setgid
	case 'o':
		rBit, wBit, xBit = 0004, 0002, 0001
		specialBit = os.ModeSticky
		useSpecial = bits.sticky
	}

	var mask os.FileMode
	if bits.read {
		mask |= rBit
	}
	if bits.write {
		mask |= wBit
	}
	if bits.exec {
		mask |= xBit
	}
	if bits.execX && current&0111 != 0 {
		mask |= xBit
	}
	if useSpecial {
		mask |= specialBit
	}
	return mask
}

func applyEquals(perm os.FileMode, who string, mask os.FileMode) os.FileMode {
	var clear os.FileMode
	for _, w := range who {
		switch w {
		case 'u':
			clear |= 0700 | os.ModeSetuid
		case 'g':
			clear |= 0070 | os.ModeSetgid
		case 'o':
			clear |= 0007 | os.ModeSticky
		}
	}
	perm &^= clear
	perm |= mask
	return perm
}
