// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package main

import (
	"fmt"
	"syscall"
)

// mntNowait is the MNT_NOWAIT flag for Getfsstat (not exported by syscall).
const mntNowait = 2

// enumerateFilesystems returns all mounted filesystems on Darwin.
func enumerateFilesystems() ([]fsEntry, error) {
	n, err := syscall.Getfsstat(nil, mntNowait)
	if err != nil {
		return nil, fmt.Errorf("getfsstat: %w", err)
	}
	buf := make([]syscall.Statfs_t, n)
	n, err = syscall.Getfsstat(buf, mntNowait)
	if err != nil {
		return nil, fmt.Errorf("getfsstat: %w", err)
	}
	entries := make([]fsEntry, 0, n)
	for i := range buf[:n] {
		entries = append(entries, darwinStatfsToEntry(&buf[i]))
	}
	return entries, nil
}

// statfsForPath returns filesystem info for a specific path on Darwin.
func statfsForPath(path string) (*fsEntry, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return nil, err
	}
	entry := darwinStatfsToEntry(&fs)
	return &entry, nil
}

// darwinStatfsToEntry converts a Darwin Statfs_t to an fsEntry.
// D1: inode counts from Files (total) and Ffree; used = total - free.
func darwinStatfsToEntry(fs *syscall.Statfs_t) fsEntry {
	bsize := int64(fs.Bsize)
	total := int64(fs.Blocks) * bsize / 1024
	free := int64(fs.Bfree) * bsize / 1024
	avail := int64(fs.Bavail) * bsize / 1024
	inodesTotal := int64(fs.Files)
	inodesFree := int64(fs.Ffree)
	return fsEntry{
		source:      int8SliceToString(fs.Mntfromname[:]),
		fsType:      int8SliceToString(fs.Fstypename[:]),
		blocks1K:    total,
		used:        total - free,
		available:   avail,
		inodesTotal: inodesTotal,
		inodesUsed:  inodesTotal - inodesFree,
		inodesFree:  inodesFree,
		mountedOn:   int8SliceToString(fs.Mntonname[:]),
	}
}

// int8SliceToString converts a null-terminated int8 slice to a Go string.
func int8SliceToString(arr []int8) string {
	buf := make([]byte, 0, len(arr))
	for _, b := range arr {
		if b == 0 {
			break
		}
		buf = append(buf, byte(b))
	}
	return string(buf)
}
