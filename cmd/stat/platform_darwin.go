// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// Platform-specific helpers for stat on Darwin.
// Implements prd082-stat birth time extraction, device major/minor, and statfs.

package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// statfsInfo holds filesystem status information from statfs(2).
type statfsInfo struct {
	fsIDHex       string // filesystem ID formatted as hex
	maxName       string // max filename length or "?" if unknown
	typeName      string
	typeNum       uint64
	blockSize     int64
	fundBlockSize int64
	blocks        uint64
	blocksFree    uint64
	blocksAvail   uint64
	files         uint64
	filesFree     uint64
}

// getBirthTime extracts the birth time from a Darwin Stat_t.
func getBirthTime(info os.FileInfo) (time.Time, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	bt := time.Unix(st.Birthtimespec.Sec, st.Birthtimespec.Nsec)
	return bt, true
}

// rawMode returns the raw Unix st_mode value from a Darwin Stat_t.
func rawMode(info os.FileInfo) uint32 {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return uint32(st.Mode)
}

// deviceMajor extracts the major device number on Darwin.
func deviceMajor(dev uint64) uint64 {
	return (dev >> 24) & 0xff
}

// deviceMinor extracts the minor device number on Darwin.
func deviceMinor(dev uint64) uint64 {
	return dev & 0xffffff
}

// getStatfs returns filesystem status for the given path.
func getStatfs(path string) (*statfsInfo, error) {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(path, &buf); err != nil {
		return nil, err
	}
	fsid := (uint64(uint32(buf.Fsid.Val[0])) << 32) | uint64(uint32(buf.Fsid.Val[1]))
	return &statfsInfo{
		fsIDHex:       fmt.Sprintf("%x", fsid),
		maxName:       "?", // Darwin statfs does not expose f_namemax
		typeName:      int8ToString(buf.Fstypename[:]),
		typeNum:       uint64(buf.Type),
		blockSize:     int64(buf.Bsize),
		fundBlockSize: int64(buf.Bsize),
		blocks:        buf.Blocks,
		blocksFree:    buf.Bfree,
		blocksAvail:   buf.Bavail,
		files:         buf.Files,
		filesFree:     buf.Ffree,
	}, nil
}

// int8ToString converts a null-terminated int8 slice to a Go string.
func int8ToString(s []int8) string {
	var b []byte
	for _, v := range s {
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}
