// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/sys/unix"
)

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

func printAllDisplay(w io.Writer, t *unix.Termios, rows, cols int) error {
	if _, err := fmt.Fprint(w, speedLine(t, rows, cols)); err != nil {
		return err
	}
	maxCol := cols
	if maxCol == 0 {
		maxCol = 80
	}
	ww := &wrapWriter{w: w, maxCol: maxCol}
	if err := writeAllCC(ww, t.Cc[:]); err != nil {
		return err
	}
	if err := writeAllCFlags(ww, t.Cflag); err != nil {
		return err
	}
	if err := writeAllFlags(ww, t.Iflag, iflags); err != nil {
		return err
	}
	if err := writeAllFlags(ww, t.Oflag, oflags); err != nil {
		return err
	}
	return writeAllFlags(ww, t.Lflag, lflags)
}

func printSaveFormat(w io.Writer, t *unix.Termios) error {
	if _, err := fmt.Fprintf(w, "%x:%x:%x:%x", t.Iflag, t.Oflag, t.Cflag, t.Lflag); err != nil {
		return err
	}
	for _, v := range t.Cc {
		if _, err := fmt.Fprintf(w, ":%x", v); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, ":%x:%x\n", t.Ispeed, t.Ospeed)
	return err
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

func csizeAllStr(cflag uint64) string {
	switch cflag & unix.CSIZE {
	case unix.CS5:
		return "cs5"
	case unix.CS6:
		return "cs6"
	case unix.CS7:
		return "cs7"
	default:
		return "cs8"
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

func flagStr(val uint64, f flagDef) string {
	if val&f.mask != 0 {
		return f.name
	}
	return "-" + f.name
}

type wrapWriter struct {
	w      io.Writer
	col    int
	maxCol int
}

func (ww *wrapWriter) add(s string) error {
	if ww.col > 0 {
		if ww.maxCol-ww.col < len(s) {
			if _, err := fmt.Fprint(ww.w, "\n"); err != nil {
				return err
			}
			ww.col = 0
		} else {
			if _, err := fmt.Fprint(ww.w, " "); err != nil {
				return err
			}
			ww.col++
		}
	}
	if _, err := fmt.Fprint(ww.w, s); err != nil {
		return err
	}
	ww.col += len(s)
	return nil
}

func (ww *wrapWriter) newline() error {
	if _, err := fmt.Fprint(ww.w, "\n"); err != nil {
		return err
	}
	ww.col = 0
	return nil
}

func writeAllCC(ww *wrapWriter, cc []uint8) error {
	for _, c := range controlChars {
		s := c.name + " = " + formatCC(cc[c.index], c.isNum) + ";"
		if err := ww.add(s); err != nil {
			return err
		}
	}
	return ww.newline()
}

func writeAllCFlags(ww *wrapWriter, cflag uint64) error {
	for _, f := range cflags[:2] {
		if err := ww.add(flagStr(cflag, f)); err != nil {
			return err
		}
	}
	if err := ww.add(csizeAllStr(cflag)); err != nil {
		return err
	}
	for _, f := range cflags[2:] {
		if err := ww.add(flagStr(cflag, f)); err != nil {
			return err
		}
	}
	return ww.newline()
}

func writeAllFlags(ww *wrapWriter, val uint64, flags []flagDef) error {
	for _, f := range flags {
		if err := ww.add(flagStr(val, f)); err != nil {
			return err
		}
	}
	return ww.newline()
}
