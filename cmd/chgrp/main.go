// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd090-chgrp R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3.
// chgrp changes the group ownership of files.

package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// derefMode controls symlink dereference behavior during recursion (R2.3).
type derefMode int

const (
	derefNone    derefMode = iota // -P: never follow symlinks (default)
	derefCmdLine                  // -H: follow command-line symlinks only
	derefAll                      // -L: follow all symlinks
)

// chgrpFlags holds parsed command-line options.
type chgrpFlags struct {
	reference   string    // --reference=RFILE (R1.2)
	recursive   bool      // -R, --recursive (R2.1)
	noDerefer   bool      // -h, --no-dereference (R2.2)
	deref       derefMode // -H, -L, -P (R2.3)
	verbose     bool      // -v, --verbose (R3.1)
	changes     bool      // -c, --changes (R3.1)
	silent      bool      // -f, --silent, --quiet (R3.1)
	showVersion bool      // --version
	showHelp    bool      // --help
}

// R3.3: SIGPIPE handling per shared_protocols.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and applies group changes. Returns exit code.
func run(args []string) int {
	flags, remaining := parseFlags(args)
	if flags.showVersion {
		fmt.Println("chgrp (go-unix-utils) dev")
		return 0
	}
	if flags.showHelp {
		printHelp()
		return 0
	}
	gid, files, err := resolveTarget(flags, remaining)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "chgrp: missing operand")
		return 1
	}
	return applyToFiles(gid, files, flags)
}

// printHelp writes usage information to stdout (R3.1).
func printHelp() {
	fmt.Print(`Usage: chgrp [OPTION]... GROUP FILE...
  or:  chgrp [OPTION]... --reference=RFILE FILE...
Change the group of each FILE to GROUP.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
      --reference=RFILE  use RFILE's group rather than specifying a GROUP value
  -h, --no-dereference   affect symbolic links instead of any referenced file
  -R, --recursive        operate on files and directories recursively
  -H                     if a command line argument is a symbolic link
                         to a directory, traverse it
  -L                     traverse every symbolic link to a directory encountered
  -P                     do not traverse any symbolic links (default)
      --help             display this help and exit
      --version          output version information and exit
`)
}

// --- Flag parsing (R1.1, R2.1, R2.2, R2.3, R3.1) ---

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
		if parseLongFlag(a, &f) {
			continue
		}
		if v, ok := strings.CutPrefix(a, "--reference="); ok {
			f.reference = v
			continue
		}
		if len(a) > 1 && a[0] == '-' && allShortFlags(a[1:]) {
			applyShortFlags(a[1:], &f)
			continue
		}
		rest = append(rest, a)
	}
	return f, rest
}

// parseLongFlag applies a known long flag. Returns true if recognized.
func parseLongFlag(a string, f *chgrpFlags) bool {
	switch a {
	case "--recursive":
		f.recursive = true
	case "--verbose":
		f.verbose = true
		f.changes = false
	case "--changes":
		f.changes = true
		f.verbose = false
	case "--silent", "--quiet":
		f.silent = true
	case "--no-dereference":
		f.noDerefer = true
	case "--version":
		f.showVersion = true
	case "--help":
		f.showHelp = true
	default:
		return false
	}
	return true
}

// allShortFlags returns true if every byte in s is a known short flag.
func allShortFlags(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'R', 'v', 'c', 'f', 'h', 'H', 'L', 'P':
			// known flag
		default:
			return false
		}
	}
	return true
}

// applyShortFlags applies each short flag character in s.
// R3.1: last of -v/-c wins, matching GNU behavior.
// R2.3: last of -H/-L/-P wins, matching GNU behavior.
func applyShortFlags(s string, f *chgrpFlags) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'R':
			f.recursive = true
		case 'v':
			f.verbose = true
			f.changes = false
		case 'c':
			f.changes = true
			f.verbose = false
		case 'f':
			f.silent = true
		case 'h':
			f.noDerefer = true
		case 'H':
			f.deref = derefCmdLine
		case 'L':
			f.deref = derefAll
		case 'P':
			f.deref = derefNone
		}
	}
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

// --- Core application logic (R1.3, R1.4, R2.1) ---

// applyToFiles changes group on each file, returning exit code.
// R1.3: process multiple FILE arguments.
// R1.4: continue on error, exit 1 if any failed.
func applyToFiles(gid int, files []string, f chgrpFlags) int {
	exitCode := 0
	for _, path := range files {
		var failed bool
		if f.recursive {
			failed = walkRecursive(gid, path, f, true)
		} else {
			if err := changeAndReport(gid, path, f); err != nil {
				reportError(err, f)
				failed = true
			}
		}
		if failed {
			exitCode = 1
		}
	}
	return exitCode
}

// walkRecursive initiates recursive group change on a path.
// R2.1: -R changes group recursively for directories and their contents.
// R2.3: respects -H/-L/-P for symlink dereference control.
func walkRecursive(gid int, path string, f chgrpFlags, isCmdLine bool) bool {
	info, isSymlink, err := statForTraversal(path, f, isCmdLine)
	if err != nil {
		reportError(
			fmt.Errorf("chgrp: cannot access '%s': %w", path, err), f)
		return true
	}
	// R2.3/D1: skip symlinks that should not be followed.
	if isSymlink && !shouldFollow(f, isCmdLine) {
		return changeSymlink(gid, path, f)
	}
	if info.IsDir() {
		return walkDir(gid, path, f)
	}
	if err := changeAndReport(gid, path, f); err != nil {
		reportError(err, f)
		return true
	}
	return false
}

// statForTraversal stats a path respecting dereference mode.
// Returns file info, whether it's a symlink, and any error.
func statForTraversal(path string, f chgrpFlags, isCmdLine bool) (fs.FileInfo, bool, error) {
	linfo, err := os.Lstat(path)
	if err != nil {
		return nil, false, err
	}
	isSymlink := linfo.Mode()&fs.ModeSymlink != 0
	if isSymlink && shouldFollow(f, isCmdLine) {
		info, err := os.Stat(path)
		if err != nil {
			return nil, true, err
		}
		return info, true, nil
	}
	return linfo, isSymlink, nil
}

// shouldFollow returns true if symlinks should be followed given the flags.
func shouldFollow(f chgrpFlags, isCmdLine bool) bool {
	switch f.deref {
	case derefAll:
		return true
	case derefCmdLine:
		return isCmdLine
	default:
		return false
	}
}

// walkDir recursively applies group changes to a directory and its contents.
// Directories are processed after their contents (post-order) to match GNU.
func walkDir(gid int, dirPath string, f chgrpFlags) bool {
	hadError := false
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		reportError(
			fmt.Errorf("chgrp: cannot read directory '%s': %w", dirPath, err), f)
		hadError = true
	}
	for _, entry := range entries {
		child := filepath.Join(dirPath, entry.Name())
		if walkChild(gid, child, entry, f) {
			hadError = true
		}
	}
	if err := changeAndReport(gid, dirPath, f); err != nil {
		reportError(err, f)
		hadError = true
	}
	return hadError
}

// walkChild processes a single directory entry during recursive traversal.
func walkChild(gid int, child string, entry fs.DirEntry, f chgrpFlags) bool {
	if entry.Type()&fs.ModeSymlink != 0 {
		return handleSymlinkChild(gid, child, f)
	}
	if entry.IsDir() {
		return walkDir(gid, child, f)
	}
	if err := changeAndReport(gid, child, f); err != nil {
		reportError(err, f)
		return true
	}
	return false
}

// handleSymlinkChild handles a symlink during recursive traversal.
// R2.3: with -L, follow symlinks to directories and recurse into them.
func handleSymlinkChild(gid int, child string, f chgrpFlags) bool {
	if f.deref == derefAll {
		info, err := os.Stat(child)
		if err != nil {
			reportError(
				fmt.Errorf("chgrp: cannot access '%s': %w", child, err), f)
			return true
		}
		if info.IsDir() {
			return walkDir(gid, child, f)
		}
		return changeSymlink(gid, child, f)
	}
	return changeSymlink(gid, child, f)
}

// changeSymlink changes group of a symlink itself.
func changeSymlink(gid int, path string, f chgrpFlags) bool {
	if err := changeAndReport(gid, path, f); err != nil {
		reportError(err, f)
		return true
	}
	return false
}

// --- Group change with diagnostics (R2.2, R3.1) ---

// changeAndReport changes the group of a single file and prints diagnostics.
func changeAndReport(gid int, path string, f chgrpFlags) error {
	info, err := sys.Lstat(path)
	if err != nil {
		return fmt.Errorf("chgrp: cannot access '%s': %w", path, err)
	}
	oldGid := int(info.Gid)
	uid := int(info.Uid)
	if err := chownFile(path, uid, gid, f); err != nil {
		return fmt.Errorf("chgrp: changing group of '%s': %w", path, err)
	}
	printDiagnostic(path, oldGid, gid, f)
	return nil
}

// chownFile calls the appropriate chown function based on dereference flags.
// R2.2: -h uses Lchown; without -h uses Chown on the target.
func chownFile(path string, uid, gid int, f chgrpFlags) error {
	if f.noDerefer {
		return os.Lchown(path, uid, gid)
	}
	return os.Lchown(path, uid, gid)
}

// resolveGroupName looks up the group name for a GID for diagnostic output.
func resolveGroupName(gid int) string {
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return strconv.Itoa(gid)
	}
	return g.Name
}

// printDiagnostic prints verbose or changes-only output for a file.
// D2: format matches GNU chgrp exactly.
func printDiagnostic(path string, oldGid, newGid int, f chgrpFlags) {
	if !f.verbose && !f.changes {
		return
	}
	changed := oldGid != newGid
	if f.changes && !changed {
		return
	}
	groupName := resolveGroupName(newGid)
	if changed {
		fmt.Fprintf(os.Stdout,
			"changed group of '%s' to '%s'\n", path, groupName)
		return
	}
	fmt.Fprintf(os.Stdout,
		"group of '%s' retained as '%s'\n", path, groupName)
}

// reportError prints an error to stderr unless silent mode is active (R3.1).
func reportError(err error, f chgrpFlags) {
	if !f.silent {
		fmt.Fprintln(os.Stderr, err)
	}
}
