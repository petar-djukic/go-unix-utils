// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// Platform-specific filesystem enumeration for Darwin.
// Uses unix.Getfsstat to list mounted filesystems and
// unix.Statfs to query individual paths.

package main

import "golang.org/x/sys/unix"

// getAllFilesystems returns info for all currently mounted filesystems.
// R1.1: enumerate all mounted filesystems via Getfsstat.
func getAllFilesystems() ([]fsInfo, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	buf := make([]unix.Statfs_t, n)
	n, err = unix.Getfsstat(buf, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}
	result := make([]fsInfo, 0, n)
	for i := 0; i < n; i++ {
		result = append(result, convertStatfs(&buf[i]))
	}
	return result, nil
}

// getPathFilesystem returns filesystem info for the filesystem containing path.
// R1.4: report the filesystem containing a given FILE argument.
func getPathFilesystem(path string) (*fsInfo, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return nil, err
	}
	info := convertStatfs(&st)
	return &info, nil
}

// convertStatfs maps a Darwin Statfs_t to our portable fsInfo.
func convertStatfs(s *unix.Statfs_t) fsInfo {
	return fsInfo{
		Device:      nullTermString(s.Mntfromname[:]),
		MountPoint:  nullTermString(s.Mntonname[:]),
		FSType:      nullTermString(s.Fstypename[:]),
		TotalBlocks: s.Blocks,
		BlockSize:   uint64(s.Bsize),
		FreeBlocks:  s.Bfree,
		AvailBlocks: s.Bavail,
		TotalInodes: s.Files,
		FreeInodes:  s.Ffree,
	}
}

// nullTermString converts a null-terminated byte slice to a Go string.
func nullTermString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
