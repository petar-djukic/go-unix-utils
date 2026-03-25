// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Defines terminal setting names, flag mappings, and character mappings
// for prd105-stty.
package main

import (
	"fmt"
	"strconv"

	"golang.org/x/sys/unix"
)

// flagField identifies which termios field a flag belongs to.
type flagField int

const (
	fieldInput   flagField = iota
	fieldOutput
	fieldControl
	fieldLocal
)

// flagDef defines a single termios flag setting.
type flagDef struct {
	field flagField
	bits  uint64
	mask  uint64 // non-zero for multi-bit fields (e.g., CSIZE)
}

// flagDefs maps setting names to their termios flag definitions.
// R2.1: each name can be enabled or prefixed with - to disable.
var flagDefs = map[string]flagDef{
	// Input mode flags
	"ignbrk":  {fieldInput, uint64(unix.IGNBRK), 0},
	"brkint":  {fieldInput, uint64(unix.BRKINT), 0},
	"ignpar":  {fieldInput, uint64(unix.IGNPAR), 0},
	"parmrk":  {fieldInput, uint64(unix.PARMRK), 0},
	"inpck":   {fieldInput, uint64(unix.INPCK), 0},
	"istrip":  {fieldInput, uint64(unix.ISTRIP), 0},
	"inlcr":   {fieldInput, uint64(unix.INLCR), 0},
	"igncr":   {fieldInput, uint64(unix.IGNCR), 0},
	"icrnl":   {fieldInput, uint64(unix.ICRNL), 0},
	"ixon":    {fieldInput, uint64(unix.IXON), 0},
	"ixoff":   {fieldInput, uint64(unix.IXOFF), 0},
	"ixany":   {fieldInput, uint64(unix.IXANY), 0},
	"imaxbel": {fieldInput, uint64(unix.IMAXBEL), 0},
	// Output mode flags
	"opost":  {fieldOutput, uint64(unix.OPOST), 0},
	"onlcr":  {fieldOutput, uint64(unix.ONLCR), 0},
	"ocrnl":  {fieldOutput, uint64(unix.OCRNL), 0},
	"onocr":  {fieldOutput, uint64(unix.ONOCR), 0},
	"onlret": {fieldOutput, uint64(unix.ONLRET), 0},
	// Control mode flags
	"cs5":    {fieldControl, uint64(unix.CS5), uint64(unix.CSIZE)},
	"cs6":    {fieldControl, uint64(unix.CS6), uint64(unix.CSIZE)},
	"cs7":    {fieldControl, uint64(unix.CS7), uint64(unix.CSIZE)},
	"cs8":    {fieldControl, uint64(unix.CS8), uint64(unix.CSIZE)},
	"cstopb": {fieldControl, uint64(unix.CSTOPB), 0},
	"cread":  {fieldControl, uint64(unix.CREAD), 0},
	"parenb": {fieldControl, uint64(unix.PARENB), 0},
	"parodd": {fieldControl, uint64(unix.PARODD), 0},
	"hupcl":  {fieldControl, uint64(unix.HUPCL), 0},
	"clocal": {fieldControl, uint64(unix.CLOCAL), 0},
	// Local mode flags
	"isig":    {fieldLocal, uint64(unix.ISIG), 0},
	"icanon":  {fieldLocal, uint64(unix.ICANON), 0},
	"iexten":  {fieldLocal, uint64(unix.IEXTEN), 0},
	"echo":    {fieldLocal, uint64(unix.ECHO), 0},
	"echoe":   {fieldLocal, uint64(unix.ECHOE), 0},
	"echok":   {fieldLocal, uint64(unix.ECHOK), 0},
	"echonl":  {fieldLocal, uint64(unix.ECHONL), 0},
	"noflsh":  {fieldLocal, uint64(unix.NOFLSH), 0},
	"tostop":  {fieldLocal, uint64(unix.TOSTOP), 0},
	"echoctl": {fieldLocal, uint64(unix.ECHOCTL), 0},
	"echoprt": {fieldLocal, uint64(unix.ECHOPRT), 0},
	"echoke":  {fieldLocal, uint64(unix.ECHOKE), 0},
	"flusho":  {fieldLocal, uint64(unix.FLUSHO), 0},
	"pendin":  {fieldLocal, uint64(unix.PENDIN), 0},
}

// flagOrder defines the display order for printAllFlags.
var flagOrder = []string{
	"ignbrk", "brkint", "ignpar", "parmrk", "inpck", "istrip",
	"inlcr", "igncr", "icrnl", "ixon", "ixoff", "ixany", "imaxbel",
	"opost", "onlcr", "ocrnl", "onocr", "onlret",
	"cs5", "cs6", "cs7", "cs8", "cstopb", "cread", "parenb",
	"parodd", "hupcl", "clocal",
	"isig", "icanon", "iexten", "echo", "echoe", "echok",
	"echonl", "noflsh", "tostop", "echoctl", "echoprt", "echoke",
	"flusho", "pendin",
}

// charDefs maps special character names to Cc array indices.
// R2.2: supports intr, quit, erase, kill, eof, eol, eol2, start,
// stop, susp, rprnt, werase, lnext, discard, min, time.
// swtch is Linux-only (VSWTC) and omitted on Darwin.
var charDefs = map[string]uint8{
	"intr":    unix.VINTR,
	"quit":    unix.VQUIT,
	"erase":   unix.VERASE,
	"kill":    unix.VKILL,
	"eof":     unix.VEOF,
	"eol":     unix.VEOL,
	"eol2":    unix.VEOL2,
	"start":   unix.VSTART,
	"stop":    unix.VSTOP,
	"susp":    unix.VSUSP,
	"rprnt":   unix.VREPRINT,
	"werase":  unix.VWERASE,
	"lnext":   unix.VLNEXT,
	"discard": unix.VDISCARD,
	"min":     unix.VMIN,
	"time":    unix.VTIME,
}

// charOrder defines the display order for printAllChars.
var charOrder = []string{
	"intr", "quit", "erase", "kill", "eof", "eol", "eol2",
	"start", "stop", "susp", "rprnt", "werase", "lnext", "discard",
	"min", "time",
}

// isFlagSet checks whether a flag is currently enabled in termios.
func isFlagSet(t *unix.Termios, def flagDef) bool {
	val := getFieldBits(t, def.field)
	if def.mask != 0 {
		return (val & def.mask) == def.bits
	}
	return (val & def.bits) != 0
}

// parseCharValue parses a special character value string.
// R2.2: supports ^X notation, ^? (DEL), undef, and numeric values.
func parseCharValue(s string) (uint8, error) {
	if s == "undef" || s == "^-" {
		return 0, nil
	}
	if len(s) == 2 && s[0] == '^' {
		return parseControlChar(s[1])
	}
	n, err := strconv.ParseUint(s, 0, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid character value %q", s)
	}
	return uint8(n), nil
}

// parseControlChar converts a ^X control character notation to a byte.
func parseControlChar(c byte) (uint8, error) {
	switch {
	case c == '?':
		return 127, nil
	case c >= 'a' && c <= 'z':
		return c - 'a' + 1, nil
	case c >= '@' && c <= '_':
		return c - '@', nil
	default:
		return 0, fmt.Errorf("invalid control character ^%c", c)
	}
}
