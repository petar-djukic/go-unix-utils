// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
	"time"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func fmtError(path string, err error) error {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	if len(msg) > 0 && unicode.IsLower(rune(msg[0])) {
		msg = string(unicode.ToUpper(rune(msg[0]))) + msg[1:]
	}
	return fmt.Errorf("stat: cannot stat '%s': %s", path, msg)
}

func fileTypeStrSize(mode os.FileMode, size int64) string {
	if mode.IsRegular() && size == 0 {
		return "regular empty file"
	}
	return fileTypeStr(mode)
}

func fileTypeStr(mode os.FileMode) string {
	switch {
	case mode.IsRegular():
		return "regular file"
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return "character special file"
	case mode&os.ModeDevice != 0:
		return "block special file"
	case mode&os.ModeNamedPipe != 0:
		return "fifo"
	case mode&os.ModeSocket != 0:
		return "socket"
	default:
		return "weird file"
	}
}

func permString(mode os.FileMode) string {
	var buf [10]byte
	buf[0] = fileTypeChar(mode)
	const rwx = "rwx"
	perm := mode.Perm()
	for i := range 9 {
		if perm&(1<<uint(8-i)) != 0 {
			buf[1+i] = rwx[i%3]
		} else {
			buf[1+i] = '-'
		}
	}
	applySpecialBits(mode, &buf)
	return string(buf[:])
}

func fileTypeChar(mode os.FileMode) byte {
	switch {
	case mode.IsRegular():
		return '-'
	case mode.IsDir():
		return 'd'
	case mode&os.ModeSymlink != 0:
		return 'l'
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return 'c'
	case mode&os.ModeDevice != 0:
		return 'b'
	case mode&os.ModeNamedPipe != 0:
		return 'p'
	case mode&os.ModeSocket != 0:
		return 's'
	default:
		return '?'
	}
}

func applySpecialBits(mode os.FileMode, buf *[10]byte) {
	if mode&os.ModeSetuid != 0 {
		if buf[3] == 'x' {
			buf[3] = 's'
		} else {
			buf[3] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if buf[6] == 'x' {
			buf[6] = 's'
		} else {
			buf[6] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if buf[9] == 'x' {
			buf[9] = 't'
		} else {
			buf[9] = 'T'
		}
	}
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000000000 -0700")
}

func formatTimeBirth(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return formatTime(t)
}

func epochBirth(t time.Time) string {
	if t.IsZero() {
		return "0"
	}
	return fmt.Sprintf("%d", t.Unix())
}

func birthTime(fi *sys.FileInfo) time.Time {
	if fi.Info == nil {
		return time.Time{}
	}
	stat, ok := fi.Info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}
	}
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}

func rawModeFromInfo(fi *sys.FileInfo) uint16 {
	if fi.Info == nil {
		return 0
	}
	stat, ok := fi.Info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return stat.Mode
}

func lookupUser(uid uint32) string {
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return strconv.Itoa(int(uid))
	}
	return u.Username
}

func lookupGroup(gid uint32) string {
	g, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.Itoa(int(gid))
	}
	return g.Name
}

func major(dev uint64) uint64 {
	return (dev >> 24) & 0xff
}

func minor(dev uint64) uint64 {
	return dev & 0xffffff
}

func quoteFile(name string) string {
	return "'" + name + "'"
}

func quotedName(fi *sys.FileInfo, path string) string {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return quoteFile(path)
		}
		return quoteFile(path) + " -> " + quoteFile(target)
	}
	return quoteFile(path)
}

func mountPoint(path string) string {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return "?"
	}
	mp := int8ToStr(fs.Mntonname[:])
	if mp == "/System/Volumes/Data" {
		return "/"
	}
	return mp
}

func int8ToStr(arr []int8) string {
	buf := make([]byte, 0, len(arr))
	for _, v := range arr {
		if v == 0 {
			break
		}
		buf = append(buf, byte(v))
	}
	return string(buf)
}

func fsIDHex(fsid syscall.Fsid) string {
	return fmt.Sprintf("%x", uint64(uint32(fsid.Val[0]))<<32|uint64(uint32(fsid.Val[1])))
}
