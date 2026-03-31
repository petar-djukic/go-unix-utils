// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Setting application logic for cmd/stty.
//
// Implements prd105-stty R4.1, R5.1, R6.1, R6.2.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// settingOp modifies termios state.
type settingOp func(t *unix.Termios)

// flagField identifies which termios field a flag belongs to.
type flagField int

const (
	fieldCflag flagField = iota
	fieldIflag
	fieldOflag
	fieldLflag
)

// flagSpec describes a flag's termios field and bitmask.
type flagSpec struct {
	field flagField
	bits  uint64
}

// flagMap maps known flag names to their field and bitmask.
var flagMap = buildFlagMap()

// ccMap maps special character names to their cc entry.
var ccMap = buildCCMap()

// negatableCombos lists combination settings that accept a - prefix.
var negatableCombos = map[string]bool{
	"raw": true, "cooked": true,
	"evenp": true, "parity": true, "oddp": true,
}

// comboNames lists all recognized combination setting names.
var comboNames = map[string]bool{
	"sane": true, "raw": true, "cooked": true,
	"evenp": true, "parity": true, "oddp": true,
}

func buildFlagMap() map[string]flagSpec {
	m := make(map[string]flagSpec)
	for _, f := range controlFlags {
		m[f.name] = flagSpec{fieldCflag, f.bits}
	}
	for _, f := range inputFlags {
		m[f.name] = flagSpec{fieldIflag, f.bits}
	}
	for _, f := range outputFlags {
		m[f.name] = flagSpec{fieldOflag, f.bits}
	}
	for _, f := range localFlags {
		m[f.name] = flagSpec{fieldLflag, f.bits}
	}
	return m
}

func buildCCMap() map[string]*ccEntry {
	m := make(map[string]*ccEntry, len(specialChars))
	for i := range specialChars {
		m[specialChars[i].name] = &specialChars[i]
	}
	return m
}

// parseSetting parses a setting from args and appends ops to cfg.
// Returns the number of args consumed.
func parseSetting(cfg *config, args []string) (int, error) {
	if op, ok := parseSingleArg(args[0]); ok {
		cfg.ops = append(cfg.ops, op)
		return 1, nil
	}
	return parsePairArg(cfg, args)
}

// parseSingleArg tries to parse a single-argument setting.
// R4.1: flag names and -flag names. R6.1: combination names.
// R6.2: numeric speed. Also handles saved -g settings.
func parseSingleArg(arg string) (settingOp, bool) {
	if comboNames[arg] {
		return comboOp(arg), true
	}
	if strings.HasPrefix(arg, "-") && negatableCombos[arg[1:]] {
		return negComboOp(arg[1:]), true
	}
	if op, ok := parseCSize(arg); ok {
		return op, true
	}
	if spec, ok := flagMap[arg]; ok {
		return setFlagOp(spec, true), true
	}
	if strings.HasPrefix(arg, "-") {
		if spec, ok := flagMap[arg[1:]]; ok {
			return setFlagOp(spec, false), true
		}
	}
	if speed, err := strconv.ParseUint(arg, 10, 64); err == nil {
		return bothSpeedOp(speed), true
	}
	if isSavedSettings(arg) {
		if op, err := restoreOp(arg); err == nil {
			return op, true
		}
	}
	return nil, false
}

// parsePairArg tries to parse a two-argument setting.
// R5.1: special character name followed by value.
// R6.2: ispeed/ospeed followed by baud rate.
func parsePairArg(cfg *config, args []string) (int, error) {
	arg := args[0]
	if cc, ok := ccMap[arg]; ok {
		if len(args) < 2 {
			return 0, fmt.Errorf("missing argument to '%s'", arg)
		}
		val, err := parseCharValue(args[1], cc.isNum)
		if err != nil {
			return 0, err
		}
		cfg.ops = append(cfg.ops, setCCOp(cc.index, val))
		return 2, nil
	}
	if arg == "ispeed" || arg == "ospeed" {
		if len(args) < 2 {
			return 0, fmt.Errorf("missing argument to '%s'", arg)
		}
		speed, err := parseSpeed(args[1])
		if err != nil {
			return 0, err
		}
		cfg.ops = append(cfg.ops, oneSpeedOp(arg, speed))
		return 2, nil
	}
	return 0, fmt.Errorf("invalid argument '%s'", arg)
}

// setFlagOp returns an op that sets or clears a termios flag.
// R4.1: enable with name, disable with -name.
func setFlagOp(spec flagSpec, set bool) settingOp {
	return func(t *unix.Termios) {
		field := flagFieldPtr(t, spec.field)
		if set {
			*field |= spec.bits
		} else {
			*field &^= spec.bits
		}
	}
}

// flagFieldPtr returns a pointer to the appropriate termios field.
func flagFieldPtr(t *unix.Termios, f flagField) *uint64 {
	switch f {
	case fieldIflag:
		return &t.Iflag
	case fieldOflag:
		return &t.Oflag
	case fieldLflag:
		return &t.Lflag
	default:
		return &t.Cflag
	}
}

// setCCOp returns an op that sets a special character value.
// R5.1: set special character slot to given value.
func setCCOp(index, val byte) settingOp {
	return func(t *unix.Termios) {
		t.Cc[index] = val
	}
}

// oneSpeedOp returns an op that sets ispeed or ospeed.
// R6.2: ispeed N or ospeed N.
func oneSpeedOp(which string, speed uint64) settingOp {
	return func(t *unix.Termios) {
		if which == "ispeed" {
			t.Ispeed = speed
		} else {
			t.Ospeed = speed
		}
	}
}

// bothSpeedOp returns an op that sets both ispeed and ospeed.
// R6.2: N alone sets both speeds.
func bothSpeedOp(speed uint64) settingOp {
	return func(t *unix.Termios) {
		t.Ispeed = speed
		t.Ospeed = speed
	}
}

// parseCSize parses cs5, cs6, cs7, cs8 character-size settings.
func parseCSize(arg string) (settingOp, bool) {
	var bits uint64
	switch arg {
	case "cs5":
		bits = unix.CS5
	case "cs6":
		bits = unix.CS6
	case "cs7":
		bits = unix.CS7
	case "cs8":
		bits = unix.CS8
	default:
		return nil, false
	}
	return func(t *unix.Termios) {
		t.Cflag &^= unix.CSIZE
		t.Cflag |= bits
	}, true
}

// comboOp returns an op for a named combination setting.
// R6.1: sane, raw, cooked, evenp/parity, oddp.
func comboOp(name string) settingOp {
	switch name {
	case "sane":
		return applySane
	case "raw":
		return applyRaw
	case "cooked":
		return applyCooked
	case "evenp", "parity":
		return applyEvenParity
	case "oddp":
		return applyOddParity
	default:
		return func(*unix.Termios) {}
	}
}

// negComboOp returns an op for a negated combination setting.
func negComboOp(name string) settingOp {
	switch name {
	case "raw":
		return applyCooked
	case "cooked":
		return applyRaw
	case "evenp", "parity", "oddp":
		return applyNoParity
	default:
		return func(*unix.Termios) {}
	}
}

// applySane resets terminal to sane defaults.
// R6.1: sane resets all flags and special characters.
func applySane(t *unix.Termios) {
	for _, cc := range specialChars {
		t.Cc[cc.index] = cc.saneVal
	}
	t.Iflag = unix.BRKINT | unix.ICRNL | unix.IMAXBEL | unix.IUTF8
	t.Oflag = unix.OPOST | unix.ONLCR
	t.Cflag = (t.Cflag &^ (unix.CSIZE | unix.PARODD | unix.PARENB)) | unix.CS8 | unix.CREAD
	t.Lflag = unix.ISIG | unix.ICANON | unix.IEXTEN | unix.ECHO |
		unix.ECHOE | unix.ECHOK | unix.ECHOCTL | unix.ECHOKE
}

// applyRaw sets raw mode: no input processing, byte-at-a-time.
// R6.1: raw clears input flags, disables canonical and signal processing.
func applyRaw(t *unix.Termios) {
	t.Iflag = 0
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ISIG | unix.ICANON
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
}

// applyCooked reverses raw mode: line-buffered, processing enabled.
// R6.1: cooked (same as -raw) restores line discipline.
func applyCooked(t *unix.Termios) {
	t.Iflag |= unix.BRKINT | unix.IGNPAR | unix.ISTRIP | unix.ICRNL | unix.IXON
	t.Oflag |= unix.OPOST
	t.Lflag |= unix.ISIG | unix.ICANON
	t.Cc[unix.VEOF] = 0x04
	t.Cc[unix.VEOL] = posixVDisable
}

// applyEvenParity enables even parity with 7-bit characters.
// R6.1: evenp/parity sets CS7 + PARENB, clears PARODD.
func applyEvenParity(t *unix.Termios) {
	t.Cflag &^= unix.CSIZE | unix.PARODD
	t.Cflag |= unix.CS7 | unix.PARENB
}

// applyOddParity enables odd parity with 7-bit characters.
// R6.1: oddp sets CS7 + PARENB + PARODD.
func applyOddParity(t *unix.Termios) {
	t.Cflag &^= unix.CSIZE
	t.Cflag |= unix.CS7 | unix.PARENB | unix.PARODD
}

// applyNoParity disables parity and sets 8-bit characters.
// R6.1: -evenp/-parity/-oddp clears PARENB, sets CS8.
func applyNoParity(t *unix.Termios) {
	t.Cflag &^= unix.PARENB | unix.CSIZE
	t.Cflag |= unix.CS8
}

// parseCharValue parses a special character value.
// R5.1: ^C for control chars, ^? for DEL, ^-/undef for disabled.
func parseCharValue(s string, isNum bool) (byte, error) {
	if isNum {
		val, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			return 0, fmt.Errorf("invalid integer argument '%s'", s)
		}
		return byte(val), nil
	}
	if s == "^-" || s == "undef" {
		return posixVDisable, nil
	}
	if len(s) == 2 && s[0] == '^' {
		return parseCtrlChar(s[1])
	}
	if len(s) == 1 {
		return s[0], nil
	}
	return 0, fmt.Errorf("invalid character '%s'", s)
}

// parseCtrlChar converts a control character notation to its byte value.
func parseCtrlChar(c byte) (byte, error) {
	if c == '?' {
		return 0x7f, nil
	}
	if c >= '@' && c <= '_' {
		return c - '@', nil
	}
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 1, nil
	}
	return 0, fmt.Errorf("invalid control character '^%c'", c)
}

// parseSpeed parses a baud rate string.
// R6.2: speed must be a non-negative integer.
func parseSpeed(s string) (uint64, error) {
	speed, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid argument '%s'", s)
	}
	return speed, nil
}

// isSavedSettings returns true if s looks like -g output (colon-separated hex).
func isSavedSettings(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) < 6 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.ParseUint(p, 16, 64); err != nil {
			return false
		}
	}
	return true
}

// restoreOp creates a settingOp from saved -g output.
func restoreOp(s string) (settingOp, error) {
	parts := strings.Split(s, ":")
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 16, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid saved setting: %s", s)
		}
		vals[i] = v
	}
	return func(t *unix.Termios) { restoreTermios(t, vals) }, nil
}

// restoreTermios applies saved settings values to a termios struct.
// Format: iflag:oflag:cflag:lflag:cc[0]..cc[N]:ispeed:ospeed.
func restoreTermios(t *unix.Termios, vals []uint64) {
	if len(vals) < 4 {
		return
	}
	t.Iflag = vals[0]
	t.Oflag = vals[1]
	t.Cflag = vals[2]
	t.Lflag = vals[3]
	ccEnd := len(vals) - 2
	for i := 4; i < ccEnd && i-4 < len(t.Cc); i++ {
		t.Cc[i-4] = byte(vals[i])
	}
	if len(vals) >= 6 {
		t.Ispeed = vals[len(vals)-2]
		t.Ospeed = vals[len(vals)-1]
	}
}

// applySettings opens the terminal, applies ops, and sets attributes.
// R4.1, R5.1, R6.1, R6.2: setting application entry point.
func applySettings(cfg *config, stderr *os.File) int {
	fd, cleanup, source, err := openTermFD(cfg.device)
	if err != nil {
		printQuotedErr(stderr, source, err)
		return 1
	}
	defer cleanup()
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		printQuotedErr(stderr, source, err)
		return 1
	}
	for _, op := range cfg.ops {
		op(termios)
	}
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, termios); err != nil {
		printQuotedErr(stderr, source, err)
		return 1
	}
	return 0
}
