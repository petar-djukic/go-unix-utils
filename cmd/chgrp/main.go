// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/chgrp: change group ownership.
// Implements srd090 R1.1-R1.4 (group ownership change),
// R2.1-R2.3 (recursive and symlink handling),
// R3.1-R3.3 (output control and exit codes).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "chgrp"

// symlinkPolicy controls how symlinks are handled during recursive traversal.
type symlinkPolicy int

const (
	symlinkNone    symlinkPolicy = iota // -P: don't follow symlinks (default with -R)
	symlinkCmdLine                      // -H: follow command-line symlinks only
	symlinkAll                          // -L: follow all symlinks
)

// options holds parsed command-line flags for chgrp.
type options struct {
	recursive   bool          // R2.1: -R/--recursive
	verbose     bool          // R3.1: -v/--verbose
	changes     bool          // R3.1: -c/--changes
	silent      bool          // R3.1: -f/--silent/--quiet
	noDerefer   bool          // R2.2: -h/--no-dereference
	reference   string        // R1.2: --reference=RFILE
	symlinks    symlinkPolicy // R2.3: -H/-L/-P symlink traversal
	dereference bool          // R2.2: --dereference (default behavior)
}

// R3.3, R1.1: main entry with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()

	opts, group, files := parseArgs(os.Args[1:])
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		os.Exit(1)
	}

	exitCode := run(opts, group, files)
	os.Exit(exitCode)
}

// parseArgs separates flags, group argument, and file operands.
// Supports short flags (-R, -v, -c, -f, -h, -H, -L, -P), combined short flags,
// and long forms (--recursive, --verbose, --changes, --silent,
// --quiet, --no-dereference, --dereference, --reference=RFILE).
func parseArgs(rawArgs []string) (options, string, []string) {
	var opts options
	var positional []string
	endOfFlags := false

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if endOfFlags {
			positional = append(positional, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			i = parseLongFlag(&opts, rawArgs, i)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if isShortFlags(arg) {
				parseShortFlags(&opts, arg[1:])
				continue
			}
		}
		positional = append(positional, arg)
	}

	// R1.2: when --reference is used, all positional args are files.
	// R1.1: otherwise, first positional is GROUP, rest are files.
	if opts.reference != "" {
		return opts, "", positional
	}
	if len(positional) == 0 {
		return opts, "", nil
	}
	return opts, positional[0], positional[1:]
}

// isShortFlags checks if arg (without leading -) contains only
// valid short flag characters for chgrp.
func isShortFlags(arg string) bool {
	for _, c := range arg[1:] {
		switch c {
		case 'R', 'v', 'c', 'f', 'h', 'H', 'L', 'P':
			// valid short flag
		default:
			return false
		}
	}
	return true
}

// parseLongFlag handles long-form flags for chgrp.
func parseLongFlag(opts *options, rawArgs []string, idx int) int {
	flag := rawArgs[idx]
	switch {
	case flag == "--recursive":
		opts.recursive = true
	case flag == "--verbose":
		opts.verbose = true
	case flag == "--changes":
		opts.changes = true
	case flag == "--silent", flag == "--quiet":
		opts.silent = true
	case flag == "--no-dereference":
		opts.noDerefer = true
	case flag == "--dereference":
		opts.dereference = true
	case strings.HasPrefix(flag, "--reference="):
		// R1.2: --reference=RFILE
		opts.reference = strings.TrimPrefix(flag, "--reference=")
	}
	return idx
}

// parseShortFlags handles combined short flags like -Rvc.
func parseShortFlags(opts *options, chars string) {
	for _, c := range chars {
		switch c {
		case 'R':
			opts.recursive = true
		case 'v':
			opts.verbose = true
		case 'c':
			opts.changes = true
		case 'f':
			opts.silent = true
		case 'h':
			opts.noDerefer = true
		case 'H':
			opts.symlinks = symlinkCmdLine
		case 'L':
			opts.symlinks = symlinkAll
		case 'P':
			opts.symlinks = symlinkNone
		}
	}
}

// run applies the group change to all files and returns the exit code.
// R1.3: processes multiple FILE arguments.
// R1.4: continues processing remaining files on error, exits 1.
// R3.2: exits 0 when all files processed successfully, 1 on any error.
func run(opts options, group string, files []string) int {
	targetGID, err := resolveGroup(opts, group)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}

	exitCode := 0
	for _, file := range files {
		if err := applyGroup(opts, targetGID, file); err != nil {
			if !opts.silent {
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			}
			exitCode = 1
		}
	}
	return exitCode
}

// resolveGroup determines the target GID from --reference or group argument.
// R1.1: accepts GROUP as name or numeric GID.
// R1.2: --reference=RFILE sets each FILE's group to match RFILE's group.
func resolveGroup(opts options, group string) (int, error) {
	if opts.reference != "" {
		return groupFromReference(opts.reference)
	}
	return parseGroup(group)
}

// groupFromReference reads the group from a reference file.
// R1.2: --reference=RFILE sets each FILE's group to match RFILE's group.
// TODO: stub — returns 0 pending full implementation.
func groupFromReference(rfile string) (int, error) {
	fmt.Fprintf(os.Stderr, "TODO: groupFromReference(%q)\n", rfile)
	return 0, nil
}

// parseGroup parses a group name or numeric GID string.
// R1.1: accepts GROUP as name or numeric GID.
// TODO: stub — returns 0 pending full implementation.
func parseGroup(group string) (int, error) {
	fmt.Fprintf(os.Stderr, "TODO: parseGroup(%q)\n", group)
	return 0, nil
}

// applyGroup applies the group change to a single file or recursively.
// R2.1: when recursive, traverses directories.
func applyGroup(opts options, gid int, path string) error {
	if opts.recursive {
		return applyGroupRecursive(opts, gid, path)
	}
	return changeGroup(opts, gid, path)
}

// applyGroupRecursive recursively applies group changes to a directory tree.
// R2.1: -R/--recursive changes group for directories and their contents.
// R2.3: respects -H/-L/-P symlink traversal policy.
// TODO: stub — returns nil pending full implementation.
func applyGroupRecursive(opts options, gid int, root string) error {
	fmt.Fprintf(os.Stderr, "TODO: applyGroupRecursive(%q, gid=%d)\n", root, gid)
	return nil
}

// changeGroup changes the group ownership of a single file.
// R1.1: changes file's group to GID.
// R2.2: respects --no-dereference / --dereference for symlinks.
// TODO: stub — returns nil pending full implementation.
func changeGroup(opts options, gid int, path string) error {
	fmt.Fprintf(os.Stderr, "TODO: changeGroup(%q, gid=%d)\n", path, gid)
	return nil
}

// printDiagnostic prints a verbose or changes-only diagnostic message.
// R3.1: -v prints a diagnostic for every file processed.
// R3.1: -c prints a diagnostic only when changes are made.
// TODO: stub — prints TODO pending full implementation.
func printDiagnostic(opts options, path string, oldGID, newGID int) {
	fmt.Fprintf(os.Stderr, "TODO: printDiagnostic(%q, old=%d, new=%d)\n",
		path, oldGID, newGID)
}

// reportError prints an error message to stderr unless silent mode is active.
// R1.4: prints error to stderr and continues processing remaining files.
// R3.1: -f/--silent suppresses most errors.
func reportError(opts options, path string, err error) {
	if !opts.silent {
		fmt.Fprintf(os.Stderr, "%s: changing group of '%s': %s\n",
			programName, path, err)
	}
}
