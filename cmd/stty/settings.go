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
	for i := 0; i < len(settings); i++ {
		n, err := applySetting(t, settings[i:])
		if err != nil {
			return err
		}
		i += n - 1
	}
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, t)
}

func applySetting(t *unix.Termios, args []string) (int, error) {
	s := args[0]
	if strings.Contains(s, ":") {
		return 1, restoreFromSave(t, s)
	}
	if s == "ispeed" || s == "ospeed" {
		return applySpeedKeyword(t, s, args[1:])
	}
	negate := strings.HasPrefix(s, "-")
	name := s
	if negate {
		name = s[1:]
	}
	if applyFlag(t, name, negate) {
		return 1, nil
	}
	if !negate && applyCSize(t, name) {
		return 1, nil
	}
	if cc, ok := findCC(name); ok && !negate {
		return applyCharSetting(t, cc, args[1:])
	}
	if applyCombo(t, s) {
		return 1, nil
	}
	if n, err := strconv.ParseUint(s, 10, 64); err == nil {
		if !validBaudRates[n] {
			return 0, fmt.Errorf("invalid argument '%s'", s)
		}
		t.Ispeed = n
		t.Ospeed = n
		return 1, nil
	}
	return 0, fmt.Errorf("invalid argument '%s'", s)
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

func findCC(name string) (ccDef, bool) {
	for _, c := range controlChars {
		if c.name == name {
			return c, true
		}
	}
	return ccDef{}, false
}

func applyCharSetting(t *unix.Termios, cc ccDef, rest []string) (int, error) {
	if len(rest) < 1 {
		return 0, fmt.Errorf("missing argument to '%s'", cc.name)
	}
	val, err := parseCharValue(rest[0])
	if err != nil {
		return 0, err
	}
	t.Cc[cc.index] = val
	return 2, nil
}

func parseCharValue(s string) (uint8, error) {
	if s == "^-" || s == "undef" {
		return vdisable, nil
	}
	if len(s) == 2 && s[0] == '^' {
		if s[1] == '?' {
			return 0x7f, nil
		}
		return s[1] & 0x1f, nil
	}
	if len(s) == 1 {
		return s[0], nil
	}
	n, err := strconv.ParseUint(s, 0, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid integer argument '%s'", s)
	}
	return uint8(n), nil
}

func applySpeedKeyword(t *unix.Termios, keyword string, rest []string) (int, error) {
	if len(rest) < 1 {
		return 0, fmt.Errorf("missing argument to '%s'", keyword)
	}
	n, err := strconv.ParseUint(rest[0], 10, 64)
	if err != nil || !validBaudRates[n] {
		return 0, fmt.Errorf("invalid argument '%s'", rest[0])
	}
	if keyword == "ispeed" {
		t.Ispeed = n
	} else {
		t.Ospeed = n
	}
	return 2, nil
}

func applyCombo(t *unix.Termios, s string) bool {
	switch s {
	case "sane":
		applySane(t)
	case "raw":
		applyRaw(t)
	case "cooked", "-raw":
		applyCooked(t)
	case "evenp", "parity":
		applyEvenParity(t)
	case "-evenp", "-parity":
		clearParity(t)
	case "oddp":
		applyOddParity(t)
	case "-oddp":
		clearOddParity(t)
	default:
		return false
	}
	return true
}

func applySane(t *unix.Termios) {
	setFlagsSane(&t.Cflag, cflags)
	setFlagsSane(&t.Iflag, iflags)
	setFlagsSane(&t.Oflag, oflags)
	setFlagsSane(&t.Lflag, lflags)
	t.Cflag = (t.Cflag &^ unix.CSIZE) | unix.CS8
	for _, c := range controlChars {
		t.Cc[c.index] = c.saneVal
	}
}

func setFlagsSane(field *uint64, flags []flagDef) {
	for _, f := range flags {
		if f.sane == 1 {
			*field |= f.mask
		} else if f.sane == -1 {
			*field &^= f.mask
		}
	}
}

func applyRaw(t *unix.Termios) {
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK |
		unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON |
		unix.ISIG | unix.IEXTEN
	t.Cflag = (t.Cflag &^ (unix.CSIZE | unix.PARENB)) | unix.CS8
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
}

func applyCooked(t *unix.Termios) {
	t.Iflag |= unix.BRKINT | unix.IGNPAR | unix.ISTRIP |
		unix.ICRNL | unix.IXON
	t.Oflag |= unix.OPOST
	t.Lflag |= unix.ISIG | unix.ICANON
	t.Cc[unix.VEOF] = 0x04
}

func applyEvenParity(t *unix.Termios) {
	t.Cflag = (t.Cflag &^ (unix.CSIZE | unix.PARODD)) | unix.CS7 | unix.PARENB
}

func clearParity(t *unix.Termios) {
	t.Cflag = (t.Cflag &^ (unix.CSIZE | unix.PARENB)) | unix.CS8
}

func applyOddParity(t *unix.Termios) {
	t.Cflag = (t.Cflag &^ unix.CSIZE) | unix.CS7 | unix.PARENB | unix.PARODD
}

func clearOddParity(t *unix.Termios) {
	t.Cflag = (t.Cflag &^ (unix.CSIZE | unix.PARENB | unix.PARODD)) | unix.CS8
}
