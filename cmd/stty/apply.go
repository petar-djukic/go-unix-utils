// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd105-stty R2.1–R2.4: apply terminal setting changes.
package main

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// applyAll iterates through setting arguments and applies each one.
func applyAll(t *unix.Termios, args []string) error {
	for i := 0; i < len(args); i++ {
		extra, err := applySetting(t, args[i:])
		if err != nil {
			return err
		}
		i += extra
	}
	return nil
}

// applySetting applies a single setting from the argument slice.
// Returns the number of extra arguments consumed beyond args[0].
func applySetting(t *unix.Termios, args []string) (int, error) {
	arg := args[0]
	if _, ok := charDefs[arg]; ok {
		return applyCharSetting(t, arg, args[1:])
	}
	if arg == "ispeed" || arg == "ospeed" {
		return applySpeedSetting(t, arg, args[1:])
	}
	if applyComboSetting(t, arg) {
		return 0, nil
	}
	if applyFlagSetting(t, arg) {
		return 0, nil
	}
	if applyBareSpeed(t, arg) {
		return 0, nil
	}
	return 0, fmt.Errorf("invalid argument %q", arg)
}

// applyCharSetting sets a special character value.
// R2.2: char name followed by a value argument.
func applyCharSetting(t *unix.Termios, name string, rest []string) (int, error) {
	if len(rest) == 0 {
		return 0, fmt.Errorf("missing argument for %q", name)
	}
	val, err := parseCharValue(rest[0])
	if err != nil {
		return 0, fmt.Errorf("invalid value for %s: %w", name, err)
	}
	t.Cc[charDefs[name]] = val
	return 1, nil
}

// applySpeedSetting handles ispeed/ospeed N arguments.
// R2.4: set input or output speed separately.
func applySpeedSetting(t *unix.Termios, dir string, rest []string) (int, error) {
	if len(rest) == 0 {
		return 0, fmt.Errorf("missing argument for %q", dir)
	}
	speed, err := parseSpeedValue(rest[0])
	if err != nil {
		return 0, err
	}
	if dir == "ispeed" {
		setInputSpeed(t, speed)
	} else {
		setOutputSpeed(t, speed)
	}
	return 1, nil
}

// applyFlagSetting enables or disables a single termios flag.
// R2.1: names prefixed with - disable the flag.
func applyFlagSetting(t *unix.Termios, arg string) bool {
	enable := true
	name := arg
	if strings.HasPrefix(arg, "-") {
		enable = false
		name = arg[1:]
	}
	def, ok := flagDefs[name]
	if !ok {
		return false
	}
	applyFlag(t, def, enable)
	return true
}

// applyFlag sets or clears a flag in the termios struct.
func applyFlag(t *unix.Termios, def flagDef, enable bool) {
	if def.mask != 0 && enable {
		clearFieldBits(t, def.field, def.mask)
		setFieldBits(t, def.field, def.bits)
		return
	}
	if enable {
		setFieldBits(t, def.field, def.bits)
	} else {
		clearFieldBits(t, def.field, def.bits)
	}
}

// setFieldBits sets the given bits in the specified termios field.
func setFieldBits(t *unix.Termios, field flagField, bits uint64) {
	switch field {
	case fieldInput:
		setIflagBits(t, bits)
	case fieldOutput:
		setOflagBits(t, bits)
	case fieldControl:
		setCflagBits(t, bits)
	case fieldLocal:
		setLflagBits(t, bits)
	}
}

// clearFieldBits clears the given bits in the specified termios field.
func clearFieldBits(t *unix.Termios, field flagField, bits uint64) {
	switch field {
	case fieldInput:
		clearIflag(t, bits)
	case fieldOutput:
		clearOflag(t, bits)
	case fieldControl:
		clearCflag(t, bits)
	case fieldLocal:
		clearLflag(t, bits)
	}
}

// applyBareSpeed handles a bare speed number setting both directions.
// R2.4: a bare number N sets both input and output speed.
func applyBareSpeed(t *unix.Termios, arg string) bool {
	speed, err := parseSpeedValue(arg)
	if err != nil {
		return false
	}
	setInputSpeed(t, speed)
	setOutputSpeed(t, speed)
	return true
}

// parseSpeedValue parses and validates a baud rate value.
func parseSpeedValue(s string) (uint64, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid speed %q", s)
	}
	if !isValidSpeed(n) {
		return 0, fmt.Errorf("invalid speed %q", s)
	}
	return n, nil
}

// applyComboSetting handles combination settings.
// R2.3: sane, raw, cooked, evenp/parity, oddp.
func applyComboSetting(t *unix.Termios, name string) bool {
	switch name {
	case "sane":
		applySane(t)
	case "raw":
		applyRaw(t)
	case "cooked", "-raw":
		applyCooked(t)
	case "evenp", "parity":
		applyEvenParity(t)
	case "oddp":
		applyOddParity(t)
	case "-parity", "-evenp", "-oddp":
		applyNoParity(t)
	default:
		return false
	}
	return true
}

// applySane resets terminal to reasonable defaults.
func applySane(t *unix.Termios) {
	applySaneChars(t)
	applySaneFlags(t)
}

// applySaneChars sets control characters to standard defaults.
func applySaneChars(t *unix.Termios) {
	t.Cc[unix.VINTR] = 3      // ^C
	t.Cc[unix.VQUIT] = 28     // ^\
	t.Cc[unix.VERASE] = 127   // ^?
	t.Cc[unix.VKILL] = 21     // ^U
	t.Cc[unix.VEOF] = 4       // ^D
	t.Cc[unix.VEOL] = 0       // <undef>
	t.Cc[unix.VEOL2] = 0      // <undef>
	t.Cc[unix.VSTART] = 17    // ^Q
	t.Cc[unix.VSTOP] = 19     // ^S
	t.Cc[unix.VSUSP] = 26     // ^Z
	t.Cc[unix.VREPRINT] = 18  // ^R
	t.Cc[unix.VWERASE] = 23   // ^W
	t.Cc[unix.VLNEXT] = 22    // ^V
	t.Cc[unix.VDISCARD] = 15  // ^O
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
}

// applySaneFlags sets termios flags to sane defaults.
func applySaneFlags(t *unix.Termios) {
	clearIflag(t, uint64(unix.IGNBRK|unix.INLCR|unix.IGNCR))
	setIflagBits(t, uint64(unix.BRKINT|unix.ICRNL))
	clearOflag(t, uint64(unix.OCRNL|unix.ONOCR|unix.ONLRET))
	setOflagBits(t, uint64(unix.OPOST|unix.ONLCR))
	setCflagBits(t, uint64(unix.CREAD))
	clearLflag(t, uint64(unix.ECHONL|unix.NOFLSH|
		unix.TOSTOP|unix.ECHOPRT))
	setLflagBits(t, uint64(unix.ISIG|unix.ICANON|unix.IEXTEN|
		unix.ECHO|unix.ECHOE|unix.ECHOK|unix.ECHOCTL|unix.ECHOKE))
}

// applyRaw sets raw mode (no input/output processing).
func applyRaw(t *unix.Termios) {
	clearIflag(t, uint64(unix.IGNBRK|unix.BRKINT|unix.IGNPAR|
		unix.PARMRK|unix.INPCK|unix.ISTRIP|unix.INLCR|unix.IGNCR|
		unix.ICRNL|unix.IXON|unix.IXOFF|unix.IXANY|unix.IMAXBEL))
	clearOflag(t, uint64(unix.OPOST))
	clearLflag(t, uint64(unix.ISIG|unix.ICANON))
	t.Cc[unix.VMIN] = 1
	t.Cc[unix.VTIME] = 0
}

// applyCooked restores cooked mode (standard line processing).
func applyCooked(t *unix.Termios) {
	setIflagBits(t, uint64(unix.BRKINT|unix.IGNPAR|
		unix.ISTRIP|unix.ICRNL|unix.IXON))
	setOflagBits(t, uint64(unix.OPOST))
	setLflagBits(t, uint64(unix.ISIG|unix.ICANON))
}

// applyEvenParity enables even parity with 7-bit characters.
func applyEvenParity(t *unix.Termios) {
	setCflagBits(t, uint64(unix.PARENB))
	clearCflag(t, uint64(unix.PARODD))
	clearCflag(t, uint64(unix.CSIZE))
	setCflagBits(t, uint64(unix.CS7))
}

// applyOddParity enables odd parity with 7-bit characters.
func applyOddParity(t *unix.Termios) {
	setCflagBits(t, uint64(unix.PARENB|unix.PARODD))
	clearCflag(t, uint64(unix.CSIZE))
	setCflagBits(t, uint64(unix.CS7))
}

// applyNoParity disables parity and sets 8-bit characters.
func applyNoParity(t *unix.Termios) {
	clearCflag(t, uint64(unix.PARENB))
	clearCflag(t, uint64(unix.CSIZE))
	setCflagBits(t, uint64(unix.CS8))
}
