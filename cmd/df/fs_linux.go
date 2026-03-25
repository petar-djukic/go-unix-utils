// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

// Platform-specific filesystem enumeration for Linux.
// Parses /proc/mounts and uses syscall.Statfs for block counts.

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
)

type mountEntry struct {
	device     string
	mountPoint string
	fsType     string
}

// getAllFilesystems returns info for all currently mounted filesystems.
// R1.1: enumerate all mounted filesystems via /proc/mounts.
func getAllFilesystems() ([]fsInfo, error) {
	mounts, err := parseProcMounts()
	if err != nil {
		return nil, err
	}
	result := make([]fsInfo, 0, len(mounts))
	for _, m := range mounts {
		var st syscall.Statfs_t
		if err := syscall.Statfs(m.mountPoint, &st); err != nil {
			continue
		}
		result = append(result, mountToFsInfo(m, &st))
	}
	return result, nil
}

// getPathFilesystem returns filesystem info for the filesystem containing path.
// R1.4: report the filesystem containing a given FILE argument.
func getPathFilesystem(path string) (*fsInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	mount, err := findMountForPath(path)
	if err != nil {
		return nil, err
	}
	info := mountToFsInfo(*mount, &st)
	return &info, nil
}

// mountToFsInfo converts a mount entry and Statfs_t to a portable fsInfo.
// Uses Frsize (fragment size) as the block unit, falling back to Bsize.
func mountToFsInfo(m mountEntry, st *syscall.Statfs_t) fsInfo {
	blockSize := uint64(st.Frsize)
	if blockSize == 0 {
		blockSize = uint64(st.Bsize)
	}
	return fsInfo{
		Device:      m.device,
		MountPoint:  m.mountPoint,
		FSType:      m.fsType,
		TotalBlocks: st.Blocks,
		BlockSize:   blockSize,
		FreeBlocks:  st.Bfree,
		AvailBlocks: st.Bavail,
		TotalInodes: st.Files,
		FreeInodes:  st.Ffree,
	}
}

// parseProcMounts reads /proc/mounts and returns all mount entries.
func parseProcMounts() ([]mountEntry, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, fmt.Errorf("reading mounts: %w", err)
	}
	defer f.Close() // best-effort close on read-only file
	var mounts []mountEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mounts = append(mounts, mountEntry{
			device:     fields[0],
			mountPoint: fields[1],
			fsType:     fields[2],
		})
	}
	return mounts, scanner.Err()
}

// findMountForPath finds the mount entry whose mount point is the longest
// prefix of path.
func findMountForPath(path string) (*mountEntry, error) {
	mounts, err := parseProcMounts()
	if err != nil {
		return nil, err
	}
	var best *mountEntry
	for i := range mounts {
		mp := mounts[i].mountPoint
		if path == mp || strings.HasPrefix(path, mp+"/") || mp == "/" {
			if best == nil || len(mp) > len(best.mountPoint) {
				best = &mounts[i]
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no mount point found for %s", path)
	}
	return best, nil
}
