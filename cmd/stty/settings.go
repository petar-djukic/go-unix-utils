// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func applySettings(fd int, t *unix.Termios, settings []string) error {
	for _, s := range settings {
		if err := applySetting(t, s); err != nil {
			return err
		}
	}
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, t)
}

func applySetting(t *unix.Termios, s string) error {
	if strings.Contains(s, ":") {
		return restoreFromSave(t, s)
	}
	negate := strings.HasPrefix(s, "-")
	name := s
	if negate {
		name = s[1:]
	}
	if applyFlag(t, name, negate) {
		return nil
	}
	if !negate && applyCSize(t, name) {
		return nil
	}
	return fmt.Errorf("invalid argument '%s'", s)
}

func applyFlag(t *unix.Termios, name string, negate bool) bool {
	if f, ok := findFlag(name, iflags); ok {
		toggleBit(&t.Iflag, f.mask, !negate)
		return true
	}
	if f, ok := findFlag(name, oflags); ok {
		toggleBit(&t.Oflag, f.mask, !negate)
		return true
	}
	if f, ok := findFlag(name, cflags); ok {
		toggleBit(&t.Cflag, f.mask, !negate)
		return true
	}
	if f, ok := findFlag(name, lflags); ok {
		toggleBit(&t.Lflag, f.mask, !negate)
		return true
	}
	return false
}

func findFlag(name string, flags []flagDef) (flagDef, bool) {
	for _, f := range flags {
		if f.name == name {
			return f, true
		}
	}
	return flagDef{}, false
}

func toggleBit(field *uint64, mask uint64, set bool) {
	if set {
		*field |= mask
	} else {
		*field &^= mask
	}
}

func applyCSize(t *unix.Termios, name string) bool {
	var val uint64
	switch name {
	case "cs5":
		val = unix.CS5
	case "cs6":
		val = unix.CS6
	case "cs7":
		val = unix.CS7
	case "cs8":
		val = unix.CS8
	default:
		return false
	}
	t.Cflag = (t.Cflag &^ unix.CSIZE) | val
	return true
}

func restoreFromSave(t *unix.Termios, s string) error {
	parts := strings.Split(s, ":")
	expected := 4 + len(t.Cc) + 2
	if len(parts) != expected {
		return fmt.Errorf("invalid argument '%s'", s)
	}
	vals, err := parseHexFields(parts)
	if err != nil {
		return fmt.Errorf("invalid argument '%s'", s)
	}
	t.Iflag = vals[0]
	t.Oflag = vals[1]
	t.Cflag = vals[2]
	t.Lflag = vals[3]
	for i := range t.Cc {
		t.Cc[i] = uint8(vals[4+i])
	}
	t.Ispeed = vals[4+len(t.Cc)]
	t.Ospeed = vals[4+len(t.Cc)+1]
	return nil
}

func parseHexFields(parts []string) ([]uint64, error) {
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 64)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	return vals, nil
}
