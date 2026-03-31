// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const mtabPath = "/etc/mtab"
const procMountsPath = "/proc/mounts"

// enumerateFilesystems returns all mounted filesystems on Linux.
func enumerateFilesystems() ([]fsEntry, error) {
	f, err := openMountTable()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMountEntries(f)
}

// openMountTable opens /etc/mtab or /proc/mounts.
func openMountTable() (*os.File, error) {
	f, err := os.Open(mtabPath)
	if err != nil {
		f, err = os.Open(procMountsPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read mount table: %w", err)
		}
	}
	return f, nil
}

// parseMountEntries reads mount table lines and converts to fsEntry.
func parseMountEntries(f *os.File) ([]fsEntry, error) {
	var entries []fsEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry, ok := parseMountLine(scanner.Text())
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries, scanner.Err()
}

// parseMountLine parses a single mount table line.
func parseMountLine(line string) (fsEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return fsEntry{}, false
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(fields[1], &fs); err != nil {
		return fsEntry{}, false
	}
	return linuxStatfsToEntry(&fs, fields[0], fields[1], fields[2]), true
}

// linuxStatfsToEntry converts Linux Statfs_t and mount info to fsEntry.
// D1: inode counts from Files (total) and Ffree; used = total - free.
func linuxStatfsToEntry(fs *syscall.Statfs_t, device, mount, fstype string) fsEntry {
	bsize := fs.Frsize
	if bsize == 0 {
		bsize = fs.Bsize
	}
	total := int64(fs.Blocks) * bsize / 1024
	free := int64(fs.Bfree) * bsize / 1024
	avail := int64(fs.Bavail) * bsize / 1024
	inodesTotal := int64(fs.Files)
	inodesFree := int64(fs.Ffree)
	return fsEntry{
		source:      device,
		fsType:      fstype,
		blocks1K:    total,
		used:        total - free,
		available:   avail,
		inodesTotal: inodesTotal,
		inodesUsed:  inodesTotal - inodesFree,
		inodesFree:  inodesFree,
		mountedOn:   mount,
	}
}

// statfsForPath returns filesystem info for a specific path on Linux.
func statfsForPath(path string) (*fsEntry, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return nil, err
	}
	device, mountPoint, fstype := findMountForPath(path)
	entry := linuxStatfsToEntry(&fs, device, mountPoint, fstype)
	return &entry, nil
}

// findMountForPath finds the mount entry for a given file path.
func findMountForPath(path string) (device, mountPoint, fsType string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	f, err := os.Open(procMountsPath)
	if err != nil {
		return "-", absPath, "-"
	}
	defer f.Close()
	return matchLongestMount(f, absPath)
}

// matchLongestMount finds the longest matching mount point for a path.
func matchLongestMount(f *os.File, absPath string) (string, string, string) {
	var bestDevice, bestMount, bestType string
	var bestLen int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mp := fields[1]
		if matchesMount(absPath, mp) && len(mp) > bestLen {
			bestLen = len(mp)
			bestDevice, bestMount, bestType = fields[0], mp, fields[2]
		}
	}
	if bestLen == 0 {
		return "-", absPath, "-"
	}
	return bestDevice, bestMount, bestType
}

// matchesMount checks if absPath is under mount point mp.
func matchesMount(absPath, mp string) bool {
	if mp == "/" {
		return true
	}
	return absPath == mp || strings.HasPrefix(absPath, mp+"/")
}
