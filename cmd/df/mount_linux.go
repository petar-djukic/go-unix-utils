// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

// Linux-specific filesystem querying using /proc/mounts and statfs(2).
// Implements srd106-df R1.1 (mounted filesystem enumeration) and
// R1.4 (filesystem lookup for FILE arguments).
package main

import (
	"bufio"
	"os"
	"strings"
	"syscall"
)

// getMounts returns all mounted filesystems by parsing /proc/mounts.
// R1.1: provides source, target, type, and block counts for each mount.
func getMounts() ([]mountInfo, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMounts(f)
}

// parseMounts reads mount entries and queries block counts for each.
func parseMounts(f *os.File) ([]mountInfo, error) {
	var entries []mountInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m, ok := parseMountLine(scanner.Text())
		if ok {
			entries = append(entries, m)
		}
	}
	return entries, scanner.Err()
}

// parseMountLine parses one /proc/mounts line and stats the filesystem.
func parseMountLine(line string) (mountInfo, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return mountInfo{}, false
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(fields[1], &st); err != nil {
		return mountInfo{}, false
	}
	bs := st.Frsize
	if bs == 0 {
		bs = st.Bsize
	}
	return mountInfo{
		source:      fields[0],
		target:      fields[1],
		fsType:      fields[2],
		totalBlocks: st.Blocks,
		freeBlocks:  st.Bfree,
		availBlocks: st.Bavail,
		blockSize:   bs,
		totalInodes: st.Files,
		freeInodes:  st.Ffree,
	}, true
}

// getFilesystemInfo returns filesystem data for the path's mount point.
// R1.4: uses statfs(2) for block counts and /proc/mounts for source info.
func getFilesystemInfo(path string) (*mountInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	source, target, fsType := findMountEntry(path)
	bs := st.Frsize
	if bs == 0 {
		bs = st.Bsize
	}
	return &mountInfo{
		source:      source,
		target:      target,
		fsType:      fsType,
		totalBlocks: st.Blocks,
		freeBlocks:  st.Bfree,
		availBlocks: st.Bavail,
		blockSize:   bs,
		totalInodes: st.Files,
		freeInodes:  st.Ffree,
	}, nil
}

// findMountEntry finds the mount point for a path by longest-prefix match.
func findMountEntry(path string) (string, string, string) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return "-", "/", "-"
	}
	defer f.Close()
	return longestPrefixMount(f, path)
}

// longestPrefixMount scans mount entries for the longest prefix match.
func longestPrefixMount(f *os.File, path string) (string, string, string) {
	var bestSrc, bestTgt, bestType string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		tgt := fields[1]
		if strings.HasPrefix(path, tgt) && len(tgt) > len(bestTgt) {
			bestSrc, bestTgt, bestType = fields[0], tgt, fields[2]
		}
	}
	if bestTgt == "" {
		return "-", "/", "-"
	}
	return bestSrc, bestTgt, bestType
}
