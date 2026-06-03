// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import "golang.org/x/sys/unix"

type flagDef struct {
	name string
	mask uint64
	sane int8
}

type ccDef struct {
	name    string
	index   int
	saneVal uint8
	isNum   bool
}

const vdisable = 0xff

var iflags = []flagDef{
	{"ignbrk", unix.IGNBRK, -1},
	{"brkint", unix.BRKINT, 1},
	{"ignpar", unix.IGNPAR, -1},
	{"parmrk", unix.PARMRK, -1},
	{"inpck", unix.INPCK, -1},
	{"istrip", unix.ISTRIP, -1},
	{"inlcr", unix.INLCR, -1},
	{"igncr", unix.IGNCR, -1},
	{"icrnl", unix.ICRNL, 1},
	{"ixon", unix.IXON, 1},
	{"ixoff", unix.IXOFF, -1},
	{"ixany", unix.IXANY, -1},
	{"imaxbel", unix.IMAXBEL, 1},
}

var oflags = []flagDef{
	{"opost", unix.OPOST, 1},
	{"onlcr", unix.ONLCR, 1},
	{"ocrnl", unix.OCRNL, -1},
	{"onocr", unix.ONOCR, -1},
	{"onlret", unix.ONLRET, -1},
}

var cflags = []flagDef{
	{"parenb", unix.PARENB, 0},
	{"parodd", unix.PARODD, 0},
	{"cstopb", unix.CSTOPB, 0},
	{"hupcl", unix.HUPCL, 0},
	{"cread", unix.CREAD, 1},
	{"clocal", unix.CLOCAL, 0},
}

var lflags = []flagDef{
	{"isig", unix.ISIG, 1},
	{"icanon", unix.ICANON, 1},
	{"echo", unix.ECHO, 1},
	{"echoe", unix.ECHOE, 1},
	{"echok", unix.ECHOK, 1},
	{"echonl", unix.ECHONL, -1},
	{"noflsh", unix.NOFLSH, -1},
	{"tostop", unix.TOSTOP, -1},
	{"echoctl", unix.ECHOCTL, 1},
	{"echoprt", unix.ECHOPRT, -1},
	{"echoke", unix.ECHOKE, 1},
	{"flusho", unix.FLUSHO, -1},
	{"pendin", unix.PENDIN, -1},
	{"iexten", unix.IEXTEN, 1},
}

var controlChars = []ccDef{
	{"intr", unix.VINTR, 0x03, false},
	{"quit", unix.VQUIT, 0x1c, false},
	{"erase", unix.VERASE, 0x7f, false},
	{"kill", unix.VKILL, 0x15, false},
	{"eof", unix.VEOF, 0x04, false},
	{"eol", unix.VEOL, vdisable, false},
	{"eol2", unix.VEOL2, vdisable, false},
	{"start", unix.VSTART, 0x11, false},
	{"stop", unix.VSTOP, 0x13, false},
	{"susp", unix.VSUSP, 0x1a, false},
	{"dsusp", unix.VDSUSP, 0x19, false},
	{"rprnt", unix.VREPRINT, 0x12, false},
	{"werase", unix.VWERASE, 0x17, false},
	{"lnext", unix.VLNEXT, 0x16, false},
	{"discard", unix.VDISCARD, 0x0f, false},
	{"min", unix.VMIN, 1, true},
	{"time", unix.VTIME, 0, true},
	{"status", unix.VSTATUS, 0x14, false},
}

var validBaudRates = map[uint64]bool{
	0: true, 50: true, 75: true, 110: true, 134: true, 150: true,
	200: true, 300: true, 600: true, 1200: true, 1800: true,
	2400: true, 4800: true, 9600: true, 19200: true, 38400: true,
	57600: true, 115200: true, 230400: true,
}
