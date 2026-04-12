// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

// Linux-specific filesystem stat using syscall.Statfs.
// Implements srd082 R5.1 (filesystem status via statfs(2)).
package main

import (
	"fmt"
	"syscall"
)

// statFilesystem returns filesystem status for the given path.
func statFilesystem(path string) (*fsInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	return &fsInfo{
		typeName: linuxFsTypeName(st.Type),
		typeHex:  uint32(st.Type),
		bsize:    st.Bsize,
		frsize:   st.Frsize,
		blocks:   st.Blocks,
		bfree:    st.Bfree,
		bavail:   st.Bavail,
		files:    st.Files,
		ffree:    st.Ffree,
		fsidVal:  st.Fsid.Val,
		namelen:  st.Namelen,
	}, nil
}

// linuxFsTypeName maps a Linux filesystem type number to a name.
func linuxFsTypeName(t int64) string {
	names := map[int64]string{
		0xEF53:     "ext2/ext3",
		0x9123683E: "btrfs",
		0x58465342: "xfs",
		0x6969:     "nfs",
		0x01021994: "tmpfs",
		0x2FC12FC1: "zfs",
	}
	if n, ok := names[t]; ok {
		return n
	}
	return fmt.Sprintf("UNKNOWN (0x%x)", t)
}
