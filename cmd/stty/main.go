// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd105-stty R1.1.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: stty [-F DEVICE | --file=DEVICE] [SETTING]...
  or:  stty [-F DEVICE | --file=DEVICE] [-a|--all]
  or:  stty [-F DEVICE | --file=DEVICE] [-g|--save]
Print or change terminal line settings.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `stty (go-unix-utils) dev
`

type flagDef struct {
	name string
	mask uint64
	sane int8 // 1=should be set in sane mode, -1=should be unset, 0=no check
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
	{"cstopb", unix.CSTOPB, 0},
	{"cread", unix.CREAD, 1},
	{"parenb", unix.PARENB, 0},
	{"parodd", unix.PARODD, 0},
	{"hupcl", unix.HUPCL, 0},
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
	{"reprint", unix.VREPRINT, 0x12, false},
	{"werase", unix.VWERASE, 0x17, false},
	{"lnext", unix.VLNEXT, 0x16, false},
	{"discard", unix.VDISCARD, 0x0f, false},
	{"status", unix.VSTATUS, 0x14, false},
	{"min", unix.VMIN, 1, true},
	{"time", unix.VTIME, 0, true},
}

func main() {
	sys.InstallSIGPIPEHandler()
	parseArgs(os.Args[1:])

	fd := int(os.Stdin.Fd())
	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stty: 'standard input': %v\n", err)
		os.Exit(1)
	}

	rows, cols := getWinSize(fd)
	if err := printDefaultDisplay(os.Stdout, t, rows, cols); err != nil {
		os.Exit(1)
	}
}

func parseArgs(args []string) {
	for _, arg := range args {
		switch arg {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "stty: invalid argument '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'stty --help' for more information.")
			os.Exit(1)
		}
	}
}

func getWinSize(fd int) (int, int) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}
	return int(ws.Row), int(ws.Col)
}

func printDefaultDisplay(w io.Writer, t *unix.Termios, rows, cols int) error {
	if _, err := fmt.Fprint(w, speedLine(t, rows, cols)); err != nil {
		return err
	}

	var changed []string
	if cs := csizeStr(t.Cflag); cs != "" {
		changed = append(changed, cs)
	}
	changed = append(changed, diffFlags(t.Cflag, cflags)...)
	changed = append(changed, diffFlags(t.Iflag, iflags)...)
	changed = append(changed, diffFlags(t.Oflag, oflags)...)
	changed = append(changed, diffFlags(t.Lflag, lflags)...)
	changed = append(changed, diffCC(t.Cc[:])...)

	if len(changed) > 0 {
		if _, err := fmt.Fprintln(w, strings.Join(changed, " ")); err != nil {
			return err
		}
	}
	return nil
}

func speedLine(t *unix.Termios, rows, cols int) string {
	if t.Ispeed != t.Ospeed && t.Ispeed != 0 {
		return fmt.Sprintf("ispeed %d baud; ospeed %d baud; rows %d; columns %d;\n",
			t.Ispeed, t.Ospeed, rows, cols)
	}
	return fmt.Sprintf("speed %d baud; rows %d; columns %d;\n", t.Ospeed, rows, cols)
}

func csizeStr(cflag uint64) string {
	switch cflag & unix.CSIZE {
	case unix.CS5:
		return "cs5"
	case unix.CS6:
		return "cs6"
	case unix.CS7:
		return "cs7"
	default:
		return ""
	}
}

func diffFlags(current uint64, flags []flagDef) []string {
	var result []string
	for _, f := range flags {
		if f.sane == 0 {
			continue
		}
		set := current&f.mask != 0
		if f.sane == 1 && !set {
			result = append(result, "-"+f.name)
		} else if f.sane == -1 && set {
			result = append(result, f.name)
		}
	}
	return result
}

func diffCC(cc []uint8) []string {
	var result []string
	for _, c := range controlChars {
		if cc[c.index] != c.saneVal {
			result = append(result, c.name+" = "+formatCC(cc[c.index], c.isNum)+";")
		}
	}
	return result
}

func formatCC(val uint8, isNum bool) string {
	if isNum {
		return fmt.Sprintf("%d", val)
	}
	if val == vdisable {
		return "<undef>"
	}
	if val == 0x7f {
		return "^?"
	}
	if val < 0x20 {
		return "^" + string(rune(val+'@'))
	}
	return string(rune(val))
}
