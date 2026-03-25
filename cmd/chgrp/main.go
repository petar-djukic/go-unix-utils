// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd090-chgrp R1.1, R1.2, R1.3, R1.4.
// chgrp changes the group ownership of files.

package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R3.3: SIGPIPE handling per shared_protocols.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// chgrpFlags holds parsed command-line options.
type chgrpFlags struct {
	reference string // --reference=RFILE (R1.2)
}

// run parses arguments and applies group changes. Returns exit code.
func run(args []string) int {
	flags, remaining := parseFlags(args)
	gid, files, err := resolveTarget(flags, remaining)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "chgrp: missing operand")
		return 1
	}
	return applyToFiles(gid, files)
}

// --- Flag parsing (R1.1) ---

// parseFlags extracts option flags from args, returning flags and remaining args.
func parseFlags(args []string) (chgrpFlags, []string) {
	var f chgrpFlags
	var rest []string
	endFlags := false
	for _, a := range args {
		if endFlags {
			rest = append(rest, a)
			continue
		}
		if a == "--" {
			endFlags = true
			continue
		}
		if v, ok := strings.CutPrefix(a, "--reference="); ok {
			f.reference = v
			continue
		}
		rest = append(rest, a)
	}
	return f, rest
}

// --- Group resolution (R1.2) ---

// resolveTarget determines the target GID and file list from flags and args.
func resolveTarget(f chgrpFlags, args []string) (int, []string, error) {
	if f.reference != "" {
		gid, err := gidFromReference(f.reference)
		if err != nil {
			return 0, nil, err
		}
		return gid, args, nil
	}
	if len(args) < 1 {
		return 0, nil, fmt.Errorf("chgrp: missing operand")
	}
	if len(args) < 2 {
		return 0, nil, fmt.Errorf(
			"chgrp: missing operand after '%s'", args[0])
	}
	gid, err := resolveGroup(args[0])
	if err != nil {
		return 0, nil, err
	}
	return gid, args[1:], nil
}

// resolveGroup converts a group name or numeric GID string to a numeric GID.
func resolveGroup(group string) (int, error) {
	// Try numeric GID first.
	if gid, err := strconv.Atoi(group); err == nil && gid >= 0 {
		return gid, nil
	}
	// Look up by name.
	g, err := user.LookupGroupId(group)
	if err == nil {
		return strconv.Atoi(g.Gid)
	}
	g, err = user.LookupGroup(group)
	if err != nil {
		return 0, fmt.Errorf(
			"chgrp: invalid group: '%s'", group)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf(
			"chgrp: invalid group: '%s'", group)
	}
	return gid, nil
}

// gidFromReference reads the group ID from a reference file (R1.4).
func gidFromReference(path string) (int, error) {
	info, err := sys.Stat(path)
	if err != nil {
		return 0, fmt.Errorf(
			"chgrp: failed to get attributes of '%s': %w", path, err)
	}
	return int(info.Gid), nil
}

// --- Core application logic (R1.3, R1.4) ---

// applyToFiles changes group on each file, returning exit code.
// R1.3: process multiple FILE arguments.
// R1.4: continue on error, exit 1 if any failed.
func applyToFiles(gid int, files []string) int {
	exitCode := 0
	for _, path := range files {
		if err := changeGroup(path, gid); err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitCode = 1
		}
	}
	return exitCode
}

// changeGroup changes the group of a single file, preserving the owner UID.
// R1.3: uses os.Lchown to handle symlinks; preserves existing owner.
func changeGroup(path string, gid int) error {
	info, err := sys.Lstat(path)
	if err != nil {
		return fmt.Errorf(
			"chgrp: cannot access '%s': %w", path, err)
	}
	uid := int(info.Uid)
	if err := os.Lchown(path, uid, gid); err != nil {
		return fmt.Errorf(
			"chgrp: changing group of '%s': %w", path, err)
	}
	return nil
}
