// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Format string expansion for stat.
// Implements prd082-stat R3.1 (file format directives), R4.1/R4.2 (printf escapes),
// R5.1/R6.1 (filesystem format directives).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// expandFormat expands %X directives using directiveFn. When doEscapes is true,
// backslash sequences are also expanded (--printf mode).
func expandFormat(fmtStr string, directiveFn func(byte) string, doEscapes bool) string {
	var buf strings.Builder
	for i := 0; i < len(fmtStr); i++ {
		switch {
		case fmtStr[i] == '%' && i+1 < len(fmtStr):
			i++
			buf.WriteString(directiveFn(fmtStr[i]))
		case doEscapes && fmtStr[i] == '\\' && i+1 < len(fmtStr):
			n := writeEscape(&buf, fmtStr[i+1:])
			i += n
		default:
			buf.WriteByte(fmtStr[i])
		}
	}
	return buf.String()
}

// expandFileFormat expands format directives for file mode.
func expandFileFormat(fmtStr, path string, fi *sys.FileInfo, doEscapes bool) string {
	return expandFormat(fmtStr, func(c byte) string {
		return fileDirective(c, path, fi)
	}, doEscapes)
}

// expandFSFormat expands format directives for filesystem mode.
func expandFSFormat(fmtStr, path string, fsi *statfsInfo, doEscapes bool) string {
	return expandFormat(fmtStr, func(c byte) string {
		return fsDirective(c, path, fsi)
	}, doEscapes)
}

// R3.1: file format directives a through n.
func fileDirective(c byte, path string, fi *sys.FileInfo) string {
	switch c {
	case 'a':
		return fmt.Sprintf("%o", unixPerms(fi.Mode))
	case 'A':
		return modeString(fi.Mode)
	case 'b':
		return fmt.Sprintf("%d", fi.Blocks)
	case 'B':
		return "512"
	case 'd':
		return fmt.Sprintf("%d", fi.Dev)
	case 'D':
		return fmt.Sprintf("%x", fi.Dev)
	case 'f':
		return fmt.Sprintf("%x", rawMode(fi.Info))
	case 'F':
		return fileTypeStr(fi)
	case 'g':
		return fmt.Sprintf("%d", fi.Gid)
	case 'G':
		return lookupGroup(fi.Gid)
	case 'h':
		return fmt.Sprintf("%d", fi.Nlink)
	case 'i':
		return fmt.Sprintf("%d", fi.Ino)
	case 'm':
		return getMountPoint(path)
	case 'n':
		return path
	default:
		return fileDirectiveExt(c, path, fi)
	}
}

// R3.1: file format directives N through Z and %.
func fileDirectiveExt(c byte, path string, fi *sys.FileInfo) string {
	switch c {
	case 'N':
		return quotedFileName(path, fi)
	case 'o':
		return fmt.Sprintf("%d", fi.Blksize)
	case 's':
		return fmt.Sprintf("%d", fi.Size)
	case 't':
		return fmt.Sprintf("%x", deviceMajor(fi.Rdev))
	case 'T':
		return fmt.Sprintf("%x", deviceMinor(fi.Rdev))
	case 'u':
		return fmt.Sprintf("%d", fi.Uid)
	case 'U':
		return lookupUser(fi.Uid)
	case 'w':
		return formatBirthTime(fi)
	case 'W':
		return birthTimeEpoch(fi)
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
	case '%':
		return "%"
	default:
		return "%" + string(c)
	}
}

// R5.1/R6.1: filesystem format directives.
func fsDirective(c byte, path string, fsi *statfsInfo) string {
	switch c {
	case 'a':
		return fmt.Sprintf("%d", fsi.blocksAvail)
	case 'b':
		return fmt.Sprintf("%d", fsi.blocks)
	case 'c':
		return fmt.Sprintf("%d", fsi.files)
	case 'd':
		return fmt.Sprintf("%d", fsi.filesFree)
	case 'f':
		return fmt.Sprintf("%d", fsi.blocksFree)
	case 'i':
		return fsi.fsIDHex
	case 'l':
		return fsi.maxName
	case 'n':
		return path
	case 's':
		return fmt.Sprintf("%d", fsi.blockSize)
	case 'S':
		return fmt.Sprintf("%d", fsi.fundBlockSize)
	case 't':
		return fmt.Sprintf("%x", fsi.typeNum)
	case 'T':
		return fsi.typeName
	case '%':
		return "%"
	default:
		return "%" + string(c)
	}
}

// R4.1/R4.2: process a single backslash escape sequence.
// Returns the number of input bytes consumed (after the backslash).
func writeEscape(buf *strings.Builder, s string) int {
	if len(s) == 0 {
		buf.WriteByte('\\')
		return 0
	}
	switch s[0] {
	case 'a':
		buf.WriteByte('\a')
	case 'b':
		buf.WriteByte('\b')
	case 'e':
		buf.WriteByte(0x1b)
	case 'f':
		buf.WriteByte('\f')
	case 'n':
		buf.WriteByte('\n')
	case 'r':
		buf.WriteByte('\r')
	case 't':
		buf.WriteByte('\t')
	case 'v':
		buf.WriteByte('\v')
	case '\\':
		buf.WriteByte('\\')
	case '"':
		buf.WriteByte('"')
	case '0', '1', '2', '3', '4', '5', '6', '7':
		val, n := parseOctal(s)
		buf.WriteByte(byte(val))
		return n
	case 'x':
		val, n := parseHex(s[1:])
		if n == 0 {
			buf.WriteString("\\x")
			return 1
		}
		buf.WriteByte(byte(val))
		return 1 + n
	default:
		buf.WriteByte('\\')
		buf.WriteByte(s[0])
	}
	return 1
}

// parseOctal reads up to 3 octal digits and returns value and byte count.
func parseOctal(s string) (int, int) {
	val, n := 0, 0
	for n < 3 && n < len(s) && s[n] >= '0' && s[n] <= '7' {
		val = val*8 + int(s[n]-'0')
		n++
	}
	return val, n
}

// parseHex reads up to 2 hex digits and returns value and byte count.
func parseHex(s string) (int, int) {
	val, n := 0, 0
	for n < 2 && n < len(s) {
		c := s[n]
		switch {
		case c >= '0' && c <= '9':
			val = val*16 + int(c-'0')
		case c >= 'a' && c <= 'f':
			val = val*16 + int(c-'a'+10)
		case c >= 'A' && c <= 'F':
			val = val*16 + int(c-'A'+10)
		default:
			return val, n
		}
		n++
	}
	return val, n
}

// quotedFileName returns %N format: 'name' or 'name' -> 'target' for symlinks.
func quotedFileName(path string, fi *sys.FileInfo) string {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "'" + path + "'"
		}
		return fmt.Sprintf("'%s' -> '%s'", path, target)
	}
	return "'" + path + "'"
}

// birthTimeEpoch returns birth time as epoch seconds, or "0" if unavailable.
func birthTimeEpoch(fi *sys.FileInfo) string {
	bt, ok := getBirthTime(fi.Info)
	if !ok {
		return "0"
	}
	return fmt.Sprintf("%d", bt.Unix())
}
