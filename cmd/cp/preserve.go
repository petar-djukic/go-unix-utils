// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Attribute preservation for cmd/cp.
// Implements srd056-cp R3.1 (preserve mode/ownership/timestamps),
// R3.3 (--preserve=ATTR_LIST with comma-separated attributes).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// preserveSet holds which file attributes to preserve during copy.
// R3.3: parsed from comma-separated --preserve=ATTR_LIST.
type preserveSet struct {
	mode       bool
	ownership  bool
	timestamps bool
	links      bool
}

// devIno identifies a file by device and inode for hard link tracking.
// R3.3: used with --preserve=links to recreate hard link structure.
type devIno struct {
	dev uint64
	ino uint64
}

// parsePreserve parses a comma-separated attribute list.
// R3.3: supported: mode, ownership, timestamps, links, all.
func parsePreserve(s string) preserveSet {
	if s == "" {
		return preserveSet{}
	}
	if s == "all" {
		return preserveSet{true, true, true, true}
	}
	return parseAttrs(s)
}

// parseAttrs maps individual attribute names to preserve flags.
func parseAttrs(s string) preserveSet {
	var ps preserveSet
	for _, a := range strings.Split(s, ",") {
		switch strings.TrimSpace(a) {
		case "mode":
			ps.mode = true
		case "ownership":
			ps.ownership = true
		case "timestamps":
			ps.timestamps = true
		case "links":
			ps.links = true
		}
	}
	return ps
}

// active reports whether any attribute preservation is requested.
func (ps preserveSet) active() bool {
	return ps.mode || ps.ownership || ps.timestamps || ps.links
}

// applyFilePreserve applies requested attributes from srcInfo to dest.
// R3.1: mode, ownership, timestamps applied in that order.
func applyFilePreserve(ps preserveSet, dest string, info *sys.FileInfo) error {
	if !ps.active() {
		return nil
	}
	if ps.mode {
		if err := preserveMode(dest, info); err != nil {
			return err
		}
	}
	if ps.timestamps {
		if err := preserveTimestamps(dest, info); err != nil {
			return err
		}
	}
	if ps.ownership {
		preserveOwnership(dest, info)
	}
	return nil
}

// preserveMode sets the file permission bits on dest.
func preserveMode(dest string, info *sys.FileInfo) error {
	perm := info.Mode.Perm()
	special := info.Mode & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	if err := os.Chmod(dest, perm|special); err != nil {
		return fmt.Errorf("preserving permissions for '%s': %s",
			dest, sysErrMsg(err))
	}
	return nil
}

// preserveTimestamps sets access and modification times on dest.
func preserveTimestamps(dest string, info *sys.FileInfo) error {
	if err := os.Chtimes(dest, info.AccessTime, info.ModTime); err != nil {
		return fmt.Errorf("preserving times for '%s': %s",
			dest, sysErrMsg(err))
	}
	return nil
}

// preserveOwnership sets uid/gid on dest. Warnings are printed on
// failure because ownership changes typically require root privileges.
func preserveOwnership(dest string, info *sys.FileInfo) {
	if err := os.Lchown(dest, int(info.Uid), int(info.Gid)); err != nil {
		fmt.Fprintf(os.Stderr, "%s: preserving ownership for '%s': %s\n",
			programName, dest, sysErrMsg(err))
	}
}

// checkHardLink checks if srcInfo represents a hard-linked file that
// was already copied. Returns the previous destination and true if so.
// R3.3: used with --preserve=links.
func checkHardLink(cs *copyState, info *sys.FileInfo, dest string) (string, bool) {
	if !cs.prs.links || info.Nlink <= 1 {
		return "", false
	}
	di := devIno{dev: info.Dev, ino: info.Ino}
	if prev, ok := cs.hardLinks[di]; ok {
		return prev, true
	}
	cs.hardLinks[di] = dest
	return "", false
}
