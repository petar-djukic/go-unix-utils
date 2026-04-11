// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// Darwin-specific filesystem querying using getfsstat(2) and statfs(2).
// Implements srd106-df R1.1 (mounted filesystem enumeration) and
// R1.4 (filesystem lookup for FILE arguments).
package main

import "syscall"

// mntNowait is the MNT_NOWAIT flag for getfsstat(2) on Darwin.
const mntNowait = 2

// getMounts returns all mounted filesystems via getfsstat(2).
// R1.1: provides source, target, type, and block counts for each mount.
func getMounts() ([]mountInfo, error) {
	n, err := syscall.Getfsstat(nil, mntNowait)
	if err != nil {
		return nil, err
	}
	buf := make([]syscall.Statfs_t, n)
	n, err = syscall.Getfsstat(buf, mntNowait)
	if err != nil {
		return nil, err
	}
	entries := make([]mountInfo, n)
	for i := 0; i < n; i++ {
		entries[i] = statfsToMount(&buf[i])
	}
	return entries, nil
}

// getFilesystemInfo returns filesystem data for the path's mount point.
// R1.4: uses statfs(2) to identify the containing filesystem.
func getFilesystemInfo(path string) (*mountInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	m := statfsToMount(&st)
	return &m, nil
}

// statfsToMount converts a Darwin Statfs_t to a mountInfo.
func statfsToMount(st *syscall.Statfs_t) mountInfo {
	return mountInfo{
		source:      int8ToString(st.Mntfromname[:]),
		target:      int8ToString(st.Mntonname[:]),
		fsType:      int8ToString(st.Fstypename[:]),
		totalBlocks: st.Blocks,
		freeBlocks:  st.Bfree,
		availBlocks: st.Bavail,
		blockSize:   int64(st.Bsize),
	}
}

// int8ToString converts a null-terminated int8 array to a Go string.
func int8ToString(arr []int8) string {
	buf := make([]byte, 0, len(arr))
	for _, b := range arr {
		if b == 0 {
			break
		}
		buf = append(buf, byte(b))
	}
	return string(buf)
}
