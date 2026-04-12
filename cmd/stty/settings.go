// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Settings modification for cmd/stty.
// Implements srd105-stty R4.1, R5.1, R6.1, R6.2.
package main

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// flagCategory identifies which termios field a flag belongs to.
type flagCategory int

const (
	catControl flagCategory = iota
	catInput
	catOutput
	catLocal
)

// flagTableEntry pairs a flag table with its category.
type flagTableEntry struct {
	entries []displayEntry
	cat     flagCategory
}

// allFlagTables lists all flag tables for lookup.
var allFlagTables = []flagTableEntry{
	{controlFlags, catControl},
	{inputFlags, catInput},
	{outputFlags, catOutput},
	{localFlags, catLocal},
}

// flagFieldPtr returns a pointer to the termios field for a category.
func flagFieldPtr(t *unix.Termios, cat flagCategory) *uint64 {
	switch cat {
	case catControl:
		return &t.Cflag
	case catInput:
		return &t.Iflag
	case catOutput:
		return &t.Oflag
	case catLocal:
		return &t.Lflag
	}
	return nil
}

// lookupFlag finds a named flag across all flag tables.
func lookupFlag(name string) (*displayEntry, flagCategory, bool) {
	for _, ft := range allFlagTables {
		for i, e := range ft.entries {
			if e.Name == name {
				return &ft.entries[i], ft.cat, true
			}
		}
	}
	return nil, 0, false
}

// lookupMultiValue finds a multi-value option (e.g., cs8, nl0).
func lookupMultiValue(name string) (*displayEntry, uint64, flagCategory, bool) {
	for _, ft := range allFlagTables {
		for i, e := range ft.entries {
			for _, v := range e.Values {
				if v.Name == name {
					return &ft.entries[i], v.Value, ft.cat, true
				}
			}
		}
	}
	return nil, 0, 0, false
}

// lookupSpecialChar finds a special character by name.
func lookupSpecialChar(name string) (*specialChar, bool) {
	for i, sc := range specialCharDisplay {
		if sc.Name == name {
			return &specialCharDisplay[i], true
		}
	}
	return nil, false
}

// parseCharValue converts a character value string to a byte.
// Accepts: ^C (control char), ^? (DEL), ^- or undef (disable),
// a literal character, or a numeric value.
func parseCharValue(s string) (uint8, error) {
	if s == "undef" || s == "^-" {
		return 0xFF, nil
	}
	if len(s) == 2 && s[0] == '^' {
		return parseControlChar(s[1])
	}
	if len(s) == 1 {
		return s[0], nil
	}
	val, err := strconv.ParseUint(s, 0, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid integer argument '%s'", s)
	}
	return uint8(val), nil
}

// parseControlChar converts a control character suffix to a byte.
func parseControlChar(c byte) (uint8, error) {
	if c == '?' {
		return 0x7F, nil
	}
	if c >= 'a' && c <= 'z' {
		c -= 0x20
	}
	if c >= '@' && c <= '_' {
		return c - '@', nil
	}
	return 0, fmt.Errorf("invalid control character: ^%c", c)
}

// saneCharDefaults maps special character indices to sane default values.
var saneCharDefaults = map[int]uint8{
	unix.VINTR:    0x03, // ^C
	unix.VQUIT:    0x1C, // ^\
	unix.VERASE:   0x7F, // ^?
	unix.VKILL:    0x15, // ^U
	unix.VEOF:     0x04, // ^D
	unix.VEOL:     0xFF, // <undef>
	unix.VEOL2:    0xFF, // <undef>
	unix.VSTART:   0x11, // ^Q
	unix.VSTOP:    0x13, // ^S
	unix.VSUSP:    0x1A, // ^Z
	unix.VDSUSP:   0x19, // ^Y
	unix.VREPRINT: 0x12, // ^R
	unix.VWERASE:  0x17, // ^W
	unix.VLNEXT:   0x16, // ^V
	unix.VDISCARD: 0x0F, // ^O
	unix.VSTATUS:  0x14, // ^T
	unix.VMIN:     1,
	unix.VTIME:    0,
}

// resetFlagsToDefault sets all flags in a table to their sane defaults.
func resetFlagsToDefault(entries []displayEntry, field *uint64) {
	for _, e := range entries {
		if len(e.Values) > 0 {
			*field = (*field &^ e.Mask) | e.DefVal
		} else if e.DefOn {
			*field |= e.Mask
		} else {
			*field &^= e.Mask
		}
	}
}

// R6.1: applySane resets terminal settings to reasonable defaults.
func applySane(t *unix.Termios) {
	for idx, val := range saneCharDefaults {
		t.Cc[idx] = val
	}
	resetFlagsToDefault(controlFlags, &t.Cflag)
	resetFlagsToDefault(inputFlags, &t.Iflag)
	resetFlagsToDefault(outputFlags, &t.Oflag)
	resetFlagsToDefault(localFlags, &t.Lflag)
}

// R6.1: applyRaw disables all input/output processing for raw mode.
func applyRaw(t *unix.Termios) {
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.IGNPAR | unix.PARMRK |
		unix.INPCK | unix.ISTRIP | unix.INLCR | unix.IGNCR |
		unix.ICRNL | unix.IXON | unix.IXOFF | unix.IXANY |
		unix.IMAXBEL | darwinIUTF8
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ISIG | unix.ICANON | unix.IEXTEN
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
}

// R6.1: applyCooked restores cooked mode (opposite of raw).
func applyCooked(t *unix.Termios) {
	t.Iflag |= unix.BRKINT | unix.IGNPAR | unix.ISTRIP | unix.ICRNL | unix.IXON
	t.Oflag |= unix.OPOST
	t.Lflag |= unix.ISIG | unix.ICANON
	t.Cc[unix.VEOF] = 0x04
	t.Cc[unix.VEOL] = 0xFF
}

// R6.1: applyEvenParity sets even parity mode.
func applyEvenParity(t *unix.Termios) {
	t.Cflag |= unix.PARENB
	t.Cflag &^= unix.PARODD
	t.Cflag = (t.Cflag &^ unix.CSIZE) | uint64(unix.CS7)
}

// R6.1: applyOddParity sets odd parity mode.
func applyOddParity(t *unix.Termios) {
	t.Cflag |= unix.PARENB | unix.PARODD
	t.Cflag = (t.Cflag &^ unix.CSIZE) | uint64(unix.CS7)
}

// clearParity removes parity and sets 8-bit character size.
func clearParity(t *unix.Termios) {
	t.Cflag &^= unix.PARENB
	t.Cflag = (t.Cflag &^ unix.CSIZE) | uint64(unix.CS8)
}

// combinationNames lists recognized combination setting names.
var combinationNames = map[string]bool{
	"sane": true, "cooked": true, "raw": true,
	"evenp": true, "parity": true, "oddp": true,
}

// applyCombinationSetting applies a named combination setting.
func applyCombinationSetting(t *unix.Termios, name string, enable bool) {
	switch name {
	case "sane":
		applySane(t)
	case "raw":
		if enable {
			applyRaw(t)
		} else {
			applyCooked(t)
		}
	case "cooked":
		if enable {
			applyCooked(t)
		} else {
			applyRaw(t)
		}
	case "evenp", "parity":
		if enable {
			applyEvenParity(t)
		} else {
			clearParity(t)
		}
	case "oddp":
		if enable {
			applyOddParity(t)
		} else {
			clearParity(t)
		}
	}
}

// parseSpeed validates and returns a speed value.
func parseSpeed(s string) (uint64, error) {
	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer argument '%s'", s)
	}
	return val, nil
}

// isSavedFormat checks if a string looks like saved stty settings.
func isSavedFormat(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) < 5 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.ParseUint(p, 16, 64); err != nil {
			return false
		}
	}
	return true
}

// restoreSaved restores terminal settings from a saved (-g) format string.
func restoreSaved(t *unix.Termios, s string) error {
	parts := strings.Split(s, ":")
	expected := 4 + len(t.Cc)
	if len(parts) != expected {
		return fmt.Errorf("invalid argument '%s'", s)
	}
	fields := []*uint64{&t.Iflag, &t.Oflag, &t.Cflag, &t.Lflag}
	for i, f := range fields {
		val, err := strconv.ParseUint(parts[i], 16, 64)
		if err != nil {
			return fmt.Errorf("invalid argument '%s'", s)
		}
		*f = val
	}
	for i := range t.Cc {
		val, err := strconv.ParseUint(parts[4+i], 16, 8)
		if err != nil {
			return fmt.Errorf("invalid argument '%s'", s)
		}
		t.Cc[i] = uint8(val)
	}
	return nil
}

// tryApplySettingArg processes one or two arguments as a setting change.
// Returns the number of arguments consumed and any error.
func tryApplySettingArg(t *unix.Termios, args []string) (int, error) {
	arg := args[0]
	if isSavedFormat(arg) {
		return 1, restoreSaved(t, arg)
	}
	if n, err := trySpeedKeyword(t, args); n > 0 || err != nil {
		return n, err
	}
	enable := true
	name := arg
	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		enable = false
		name = arg[1:]
	}
	return tryNamedSetting(t, args, name, enable)
}

// trySpeedKeyword handles ispeed/ospeed keywords.
func trySpeedKeyword(t *unix.Termios, args []string) (int, error) {
	arg := args[0]
	if arg != "ispeed" && arg != "ospeed" {
		return 0, nil
	}
	if len(args) < 2 {
		return 0, fmt.Errorf("missing argument to '%s'", arg)
	}
	speed, err := parseSpeed(args[1])
	if err != nil {
		return 0, err
	}
	if arg == "ispeed" {
		t.Ispeed = speed
	} else {
		t.Ospeed = speed
	}
	return 2, nil
}

// tryNamedSetting attempts to apply a named setting (flag, special char,
// combination, multi-value, or bare speed).
func tryNamedSetting(t *unix.Termios, args []string, name string, enable bool) (int, error) {
	if combinationNames[name] {
		applyCombinationSetting(t, name, enable)
		return 1, nil
	}
	if sc, ok := lookupSpecialChar(name); ok {
		return applySpecialCharArg(t, sc, args, name, enable)
	}
	if entry, cat, ok := lookupFlag(name); ok {
		applyFlagBit(t, entry, cat, enable)
		return 1, nil
	}
	return tryMultiValueOrSpeed(t, args, name, enable)
}

// applyFlagBit enables or disables a single flag bit in the termios struct.
func applyFlagBit(t *unix.Termios, entry *displayEntry, cat flagCategory, enable bool) {
	field := flagFieldPtr(t, cat)
	if enable {
		*field |= entry.Mask
	} else {
		*field &^= entry.Mask
	}
}

// applySpecialCharArg sets a special character from the argument list.
func applySpecialCharArg(t *unix.Termios, sc *specialChar, args []string, name string, enable bool) (int, error) {
	if !enable {
		return 0, fmt.Errorf("invalid argument '-%s'", name)
	}
	if len(args) < 2 {
		return 0, fmt.Errorf("missing argument to '%s'", name)
	}
	val, err := parseCharValue(args[1])
	if err != nil {
		return 0, err
	}
	t.Cc[sc.Index] = val
	return 2, nil
}

// tryMultiValueOrSpeed tries multi-value flags, then bare speed, then errors.
func tryMultiValueOrSpeed(t *unix.Termios, args []string, name string, enable bool) (int, error) {
	if entry, val, cat, ok := lookupMultiValue(name); ok {
		if !enable {
			return 0, fmt.Errorf("invalid argument '-%s'", name)
		}
		field := flagFieldPtr(t, cat)
		*field = (*field &^ entry.Mask) | val
		return 1, nil
	}
	if speed, err := parseSpeed(args[0]); err == nil {
		t.Ispeed = speed
		t.Ospeed = speed
		return 1, nil
	}
	return 0, fmt.Errorf("invalid argument '%s'", args[0])
}

// processSettings reads terminal settings, applies changes, and writes back.
func processSettings(fd int, settings []string) error {
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	i := 0
	for i < len(settings) {
		consumed, applyErr := tryApplySettingArg(t, settings[i:])
		if applyErr != nil {
			return applyErr
		}
		i += consumed
	}
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, t)
}
