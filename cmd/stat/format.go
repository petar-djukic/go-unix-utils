// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func expandFormat(fi *sys.FileInfo, path, format string, _ bool) string {
	var b strings.Builder
	i := 0
	for i < len(format) {
		if format[i] == '\\' {
			ch, adv := parseEscape(format[i:])
			b.WriteString(ch)
			i += adv
		} else if format[i] == '%' {
			val, adv := parseFileDirective(fi, path, format[i:])
			b.WriteString(val)
			i += adv
		} else {
			b.WriteByte(format[i])
			i++
		}
	}
	return b.String()
}

func expandFsFormat(fs *syscall.Statfs_t, path, format string, _ bool) string {
	var b strings.Builder
	i := 0
	for i < len(format) {
		if format[i] == '\\' {
			ch, adv := parseEscape(format[i:])
			b.WriteString(ch)
			i += adv
		} else if format[i] == '%' {
			val, adv := parseFsDirective(fs, path, format[i:])
			b.WriteString(val)
			i += adv
		} else {
			b.WriteByte(format[i])
			i++
		}
	}
	return b.String()
}

func parseFileDirective(fi *sys.FileInfo, path, s string) (string, int) {
	if len(s) < 2 {
		return "%", 1
	}
	if s[1] == '%' {
		return "%", 2
	}
	flags, width, prec, dir, adv := parseModifiers(s[1:])
	val := fileDirectiveValue(fi, path, dir)
	formatted := applyModifiers(val, flags, width, prec)
	return formatted, 1 + adv
}

func parseFsDirective(fs *syscall.Statfs_t, path, s string) (string, int) {
	if len(s) < 2 {
		return "%", 1
	}
	if s[1] == '%' {
		return "%", 2
	}
	flags, width, prec, dir, adv := parseModifiers(s[1:])
	val := fsDirectiveValue(fs, path, dir)
	formatted := applyModifiers(val, flags, width, prec)
	return formatted, 1 + adv
}

func parseModifiers(s string) (string, int, int, byte, int) {
	i := 0
	var flags string
	for i < len(s) && (s[i] == '-' || s[i] == '+' || s[i] == ' ' || s[i] == '0' || s[i] == '#') {
		flags += string(s[i])
		i++
	}
	width := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		width = width*10 + int(s[i]-'0')
		i++
	}
	prec := -1
	if i < len(s) && s[i] == '.' {
		i++
		prec = 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			prec = prec*10 + int(s[i]-'0')
			i++
		}
	}
	if i >= len(s) {
		return flags, width, prec, 0, i
	}
	return flags, width, prec, s[i], i + 1
}

func applyModifiers(val string, flags string, width int, prec int) string {
	if width == 0 && prec < 0 && flags == "" {
		return val
	}
	fmtStr := "%" + flags
	if width > 0 {
		fmtStr += strconv.Itoa(width)
	}
	if prec >= 0 {
		fmtStr += "." + strconv.Itoa(prec)
	}
	fmtStr += "s"
	return fmt.Sprintf(fmtStr, val)
}

func fileDirectiveValue(fi *sys.FileInfo, path string, dir byte) string {
	rawMode := rawModeFromInfo(fi)
	switch dir {
	case 'a':
		return fmt.Sprintf("%o", rawMode&07777)
	case 'A':
		return permString(fi.Mode)
	case 'b':
		return fmt.Sprintf("%d", fi.Blocks)
	case 'B':
		return "512"
	case 'd':
		return fmt.Sprintf("%d", fi.Dev)
	case 'D':
		return fmt.Sprintf("%x", fi.Dev)
	case 'f':
		return fmt.Sprintf("%x", rawMode)
	case 'F':
		return fileTypeStrSize(fi.Mode, fi.Size)
	case 'g':
		return fmt.Sprintf("%d", fi.Gid)
	case 'G':
		return lookupGroup(fi.Gid)
	case 'h':
		return fmt.Sprintf("%d", fi.Nlink)
	case 'i':
		return fmt.Sprintf("%d", fi.Ino)
	default:
		return fileDirectiveExt(fi, path, dir)
	}
}

func fileDirectiveExt(fi *sys.FileInfo, path string, dir byte) string {
	switch dir {
	case 'm':
		return mountPoint(path)
	case 'n':
		return path
	case 'N':
		return quotedName(fi, path)
	case 'o':
		return fmt.Sprintf("%d", fi.Blksize)
	case 's':
		return fmt.Sprintf("%d", fi.Size)
	case 't':
		return fmt.Sprintf("%x", major(fi.Rdev))
	case 'T':
		return fmt.Sprintf("%x", minor(fi.Rdev))
	case 'u':
		return fmt.Sprintf("%d", fi.Uid)
	case 'U':
		return lookupUser(fi.Uid)
	default:
		return fileTimeDirective(fi, dir)
	}
}

func fileTimeDirective(fi *sys.FileInfo, dir byte) string {
	birth := birthTime(fi)
	switch dir {
	case 'w':
		return formatTimeBirth(birth)
	case 'W':
		return epochBirth(birth)
	case 'x':
		return formatTime(fi.AccessTime)
	case 'X':
		return fmt.Sprintf("%d", fi.AccessTime.Unix())
	case 'y':
		return formatTime(fi.ModTime)
	case 'Y':
		return fmt.Sprintf("%d", fi.ModTime.Unix())
	case 'z':
		return formatTime(fi.ChangeTime)
	case 'Z':
		return fmt.Sprintf("%d", fi.ChangeTime.Unix())
	default:
		return "%" + string(dir)
	}
}

func fsDirectiveValue(fs *syscall.Statfs_t, path string, dir byte) string {
	switch dir {
	case 'a':
		return fmt.Sprintf("%d", fs.Bavail)
	case 'b':
		return fmt.Sprintf("%d", fs.Blocks)
	case 'c':
		return fmt.Sprintf("%d", fs.Files)
	case 'd':
		return fmt.Sprintf("%d", fs.Ffree)
	case 'f':
		return fmt.Sprintf("%d", fs.Bfree)
	case 'i':
		return fsIDHex(fs.Fsid)
	case 'l':
		return "?"
	case 'n':
		return path
	case 's':
		return fmt.Sprintf("%d", fs.Bsize)
	case 'S':
		return fmt.Sprintf("%d", fs.Bsize)
	case 't':
		return fmt.Sprintf("%x", fs.Type)
	case 'T':
		return int8ToStr(fs.Fstypename[:])
	default:
		return "%" + string(dir)
	}
}

func parseEscape(s string) (string, int) {
	if len(s) < 2 {
		return "\\", 1
	}
	switch s[1] {
	case 'n':
		return "\n", 2
	case 't':
		return "\t", 2
	case 'r':
		return "\r", 2
	case '\\':
		return "\\", 2
	case 'a':
		return "\a", 2
	case 'b':
		return "\b", 2
	case 'f':
		return "\f", 2
	case 'v':
		return "\v", 2
	case '0':
		return parseOctalEscape(s)
	case 'x':
		return parseHexEscape(s)
	default:
		return string(s[:2]), 2
	}
}

func parseOctalEscape(s string) (string, int) {
	n := 2
	val := 0
	for n < len(s) && n < 5 && s[n] >= '0' && s[n] <= '7' {
		val = val*8 + int(s[n]-'0')
		n++
	}
	if n == 2 {
		return "\x00", 2
	}
	return string(rune(val)), n
}

func parseHexEscape(s string) (string, int) {
	n := 2
	val := 0
	for n < len(s) && n < 4 {
		c := s[n]
		if c >= '0' && c <= '9' {
			val = val*16 + int(c-'0')
		} else if c >= 'a' && c <= 'f' {
			val = val*16 + int(c-'a') + 10
		} else if c >= 'A' && c <= 'F' {
			val = val*16 + int(c-'A') + 10
		} else {
			break
		}
		n++
	}
	if n == 2 {
		return "\\x", 2
	}
	return string(rune(val)), n
}
