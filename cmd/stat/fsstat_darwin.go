// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// Darwin-specific filesystem stat using syscall.Statfs.
// Implements srd082 R5.1 (filesystem status via statfs(2)).
package main

import "syscall"

// statFilesystem returns filesystem status for the given path.
// D1: uses syscall.Statfs on Darwin with Darwin-specific field mapping.
func statFilesystem(path string) (*fsInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil, err
	}
	return &fsInfo{
		typeName: int8ToString(st.Fstypename[:]),
		typeHex:  st.Type,
		bsize:    int64(st.Bsize),
		frsize:   int64(st.Bsize),
		blocks:   st.Blocks,
		bfree:    st.Bfree,
		bavail:   st.Bavail,
		files:    st.Files,
		ffree:    st.Ffree,
		fsidVal:  st.Fsid.Val,
		namelen:  0, // macOS statfs lacks f_namemax; GNU stat shows "?"
	}, nil
}
