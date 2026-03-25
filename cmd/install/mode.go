// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// resolveMode parses the -m flag value or returns the default.
// R1.2: supports both octal (0755) and symbolic (u+rwx) modes.
func resolveMode(cfg config) os.FileMode {
	if cfg.mode == "" {
		return defaultMode
	}
	if len(cfg.mode) > 0 && isOctalDigit(cfg.mode[0]) {
		return parseOctalMode(cfg.mode)
	}
	mode, err := parseSymbolicMode(cfg.mode)
	if err != nil {
		printErr("invalid mode '%s'", cfg.mode)
		return defaultMode
	}
	return mode
}

// isOctalDigit reports whether ch is an octal digit.
func isOctalDigit(ch byte) bool {
	return ch >= '0' && ch <= '7'
}

// parseOctalMode parses an octal mode string into os.FileMode.
func parseOctalMode(s string) os.FileMode {
	mode, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		printErr("invalid mode '%s'", s)
		return defaultMode
	}
	return unixToFileMode(uint32(mode))
}

// unixToFileMode converts Unix-style 12-bit mode to Go os.FileMode.
func unixToFileMode(m uint32) os.FileMode {
	fm := os.FileMode(m & 0o777)
	if m&0o4000 != 0 {
		fm |= os.ModeSetuid
	}
	if m&0o2000 != 0 {
		fm |= os.ModeSetgid
	}
	if m&0o1000 != 0 {
		fm |= os.ModeSticky
	}
	return fm
}

// parseSymbolicMode parses a symbolic mode string (e.g., "u+rwx,go-w").
// The base mode is 0; operations are applied sequentially.
func parseSymbolicMode(spec string) (os.FileMode, error) {
	var mode uint32
	for _, clause := range strings.Split(spec, ",") {
		result, err := applyModeClause(clause, mode)
		if err != nil {
			return 0, err
		}
		mode = result
	}
	return unixToFileMode(mode), nil
}

// applyModeClause applies a single symbolic mode clause to a mode.
func applyModeClause(clause string, mode uint32) (uint32, error) {
	who, pos := parseModeWho(clause)
	if pos >= len(clause) {
		return 0, fmt.Errorf("invalid mode '%s'", clause)
	}
	op := clause[pos]
	if op != '+' && op != '-' && op != '=' {
		return 0, fmt.Errorf("invalid mode '%s'", clause)
	}
	perm := parseModePerms(clause[pos+1:], who)
	return applyModeOp(mode, who, op, perm), nil
}

// parseModeWho extracts the who mask from the clause prefix.
// Returns the who bitmask and the position after the who chars.
func parseModeWho(clause string) (uint32, int) {
	var who uint32
	i := 0
	for i < len(clause) {
		switch clause[i] {
		case 'u':
			who |= 0o4700
		case 'g':
			who |= 0o2070
		case 'o':
			who |= 0o1007
		case 'a':
			who |= 0o7777
		default:
			if who == 0 {
				who = 0o7777
			}
			return who, i
		}
		i++
	}
	if who == 0 {
		who = 0o7777
	}
	return who, i
}

// parseModePerms converts permission chars to mode bits for the given who.
func parseModePerms(s string, who uint32) uint32 {
	var perm uint32
	for i := 0; i < len(s); i++ {
		perm |= permCharToBits(s[i], who)
	}
	return perm
}

// permCharToBits returns mode bits for a single permission character.
func permCharToBits(ch byte, who uint32) uint32 {
	switch ch {
	case 'r':
		return expandPerm(who, 0o4)
	case 'w':
		return expandPerm(who, 0o2)
	case 'x', 'X':
		return expandPerm(who, 0o1)
	case 's':
		return who & 0o6000
	case 't':
		return 0o1000
	}
	return 0
}

// expandPerm distributes a base permission bit across who classes.
func expandPerm(who uint32, baseBit uint32) uint32 {
	var bits uint32
	if who&0o700 != 0 {
		bits |= baseBit << 6
	}
	if who&0o070 != 0 {
		bits |= baseBit << 3
	}
	if who&0o007 != 0 {
		bits |= baseBit
	}
	return bits
}

// applyModeOp applies an operator to the mode with the given perm bits.
func applyModeOp(mode, who uint32, op byte, perm uint32) uint32 {
	switch op {
	case '+':
		return mode | perm
	case '-':
		return mode &^ perm
	case '=':
		return (mode &^ who) | perm
	}
	return mode
}
