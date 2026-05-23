// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func defaultFile(fi *sys.FileInfo, path string) string {
	var b strings.Builder
	rawMode := rawModeFromInfo(fi)
	writeFileLine(&b, fi, path)
	writeSizeLine(&b, fi)
	writeDeviceLine(&b, fi)
	writeAccessLine(&b, fi, rawMode)
	writeTimestamps(&b, fi)
	return b.String()
}

func writeFileLine(b *strings.Builder, fi *sys.FileInfo, path string) {
	if fi.Mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			fmt.Fprintf(b, "  File: %s\n", path)
		} else {
			fmt.Fprintf(b, "  File: %s -> %s\n", path, target)
		}
	} else {
		fmt.Fprintf(b, "  File: %s\n", path)
	}
}

func writeSizeLine(b *strings.Builder, fi *sys.FileInfo) {
	fmt.Fprintf(b, "  Size: %-10d\tBlocks: %-10d IO Block: %-6d %s\n",
		fi.Size, fi.Blocks, fi.Blksize, fileTypeStrSize(fi.Mode, fi.Size))
}

func writeDeviceLine(b *strings.Builder, fi *sys.FileInfo) {
	maj := major(fi.Dev)
	min := minor(fi.Dev)
	if fi.Mode&(os.ModeDevice|os.ModeCharDevice) != 0 {
		rMaj := major(fi.Rdev)
		rMin := minor(fi.Rdev)
		fmt.Fprintf(b, "Device: %d,%d\tInode: %-10d  Links: %-5d Device type: %d,%d\n",
			maj, min, fi.Ino, fi.Nlink, rMaj, rMin)
	} else {
		fmt.Fprintf(b, "Device: %d,%d\tInode: %-10d  Links: %d\n",
			maj, min, fi.Ino, fi.Nlink)
	}
}

func writeAccessLine(b *strings.Builder, fi *sys.FileInfo, rawMode uint16) {
	octal := rawMode & 07777
	perm := permString(fi.Mode)
	uname := lookupUser(fi.Uid)
	gname := lookupGroup(fi.Gid)
	fmt.Fprintf(b, "Access: (%04o/%s)  Uid: (%5d/%8s)   Gid: (%5d/%8s)\n",
		octal, perm, fi.Uid, uname, fi.Gid, gname)
}

func writeTimestamps(b *strings.Builder, fi *sys.FileInfo) {
	birth := birthTime(fi)
	fmt.Fprintf(b, "Access: %s\n", formatTime(fi.AccessTime))
	fmt.Fprintf(b, "Modify: %s\n", formatTime(fi.ModTime))
	fmt.Fprintf(b, "Change: %s\n", formatTime(fi.ChangeTime))
	fmt.Fprintf(b, " Birth: %s\n", formatTime(birth))
}

func terseFile(fi *sys.FileInfo, path string) string {
	rawMode := rawModeFromInfo(fi)
	birth := birthTime(fi)
	return fmt.Sprintf("%s %d %d %x %d %d %x %d %d %x %x %d %d %d %d %d",
		path, fi.Size, fi.Blocks, rawMode,
		fi.Uid, fi.Gid, fi.Dev, fi.Ino, fi.Nlink,
		major(fi.Rdev), minor(fi.Rdev),
		fi.AccessTime.Unix(), fi.ModTime.Unix(),
		fi.ChangeTime.Unix(), birth.Unix(),
		fi.Blksize)
}

func defaultFS(fs *syscall.Statfs_t, path string) string {
	var b strings.Builder
	typeName := int8ToStr(fs.Fstypename[:])
	fsid := fsIDHex(fs.Fsid)
	fmt.Fprintf(&b, "  File: \"%s\"\n", path)
	fmt.Fprintf(&b, "    ID: %s Namelen: %-7s Type: %s\n",
		fsid, "?", typeName)
	fmt.Fprintf(&b, "Block size: %-10d Fundamental block size: %d\n",
		fs.Bsize, fs.Bsize)
	fmt.Fprintf(&b, "Blocks: Total: %-10d Free: %-10d Available: %d\n",
		fs.Blocks, fs.Bfree, fs.Bavail)
	fmt.Fprintf(&b, "Inodes: Total: %-10d Free: %d\n",
		fs.Files, fs.Ffree)
	return b.String()
}

func terseFS(fs *syscall.Statfs_t, path string) string {
	fsid := fsIDHex(fs.Fsid)
	return fmt.Sprintf("%s %s ? %x %d %d %d %d %d %d %d",
		path, fsid, fs.Type,
		fs.Bsize, fs.Bsize,
		fs.Blocks, fs.Bfree, fs.Bavail,
		fs.Files, fs.Ffree)
}
