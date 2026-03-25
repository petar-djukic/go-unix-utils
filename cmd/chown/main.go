// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd091-chown R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1.
// chown changes the owner and/or group of files.
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

// ownerSpec holds the parsed owner and group from OWNER[:GROUP] syntax.
// R1.1: uid/gid of -1 means "do not change".
type ownerSpec struct {
	uid int
	gid int
}

// chownFlags holds parsed command-line options.
type chownFlags struct {
	reference   string    // --reference=RFILE (R1.3)
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

// run parses arguments and applies ownership changes. Returns exit code.
func run(args []string) int {
	flags, remaining := parseFlags(args)
	if flags.showVersion {
		fmt.Println("chown (go-unix-utils) dev")
		return 0
	}
	if flags.showHelp {
		printHelp()
		return 0
	}
	spec, files, err := resolveTarget(flags, remaining)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "chown: missing operand")
		return 1
	}
	return applyToFiles(spec, files, flags)
}

// printHelp writes usage information to stdout (R3.1).
func printHelp() {
	fmt.Print(`Usage: chown [OPTION]... [OWNER][:[GROUP]] FILE...
  or:  chown [OPTION]... --reference=RFILE FILE...
Change the owner and/or group of each FILE to OWNER and/or GROUP.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
      --reference=RFILE  use RFILE's owner and group rather than specifying
                         OWNER:GROUP values
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

// --- Flag parsing ---

// parseFlags extracts option flags from args, returning flags and remaining.
func parseFlags(args []string) (chownFlags, []string) {
	var f chownFlags
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
func parseLongFlag(a string, f *chownFlags) bool {
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
func applyShortFlags(s string, f *chownFlags) {
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

// --- Ownership resolution (R1.1, R1.2, R1.3) ---

// resolveTarget determines the target owner/group and file list.
func resolveTarget(f chownFlags, args []string) (ownerSpec, []string, error) {
	if f.reference != "" {
		spec, err := specFromReference(f.reference)
		if err != nil {
			return ownerSpec{}, nil, err
		}
		return spec, args, nil
	}
	if len(args) < 1 {
		return ownerSpec{}, nil, fmt.Errorf("chown: missing operand")
	}
	if len(args) < 2 {
		return ownerSpec{}, nil, fmt.Errorf(
			"chown: missing operand after '%s'", args[0])
	}
	spec, err := parseOwnerGroup(args[0])
	if err != nil {
		return ownerSpec{}, nil, err
	}
	return spec, args[1:], nil
}

// parseOwnerGroup parses the OWNER[:GROUP] specification.
// R1.1: OWNER, OWNER:GROUP, OWNER:, :GROUP forms.
// R1.2: names or numeric IDs.
func parseOwnerGroup(spec string) (ownerSpec, error) {
	ownerPart, groupPart, hasColon := strings.Cut(spec, ":")
	if !hasColon {
		return parseOwnerOnly(spec)
	}
	return parseOwnerAndGroup(spec, ownerPart, groupPart)
}

// parseOwnerOnly handles the bare OWNER form (no colon).
func parseOwnerOnly(owner string) (ownerSpec, error) {
	uid, err := resolveUser(owner)
	if err != nil {
		return ownerSpec{}, err
	}
	return ownerSpec{uid: uid, gid: -1}, nil
}

// parseOwnerAndGroup handles OWNER:GROUP, OWNER:, and :GROUP forms.
// fullSpec is the original argument for error messages matching GNU format.
func parseOwnerAndGroup(fullSpec, ownerPart, groupPart string) (ownerSpec, error) {
	result := ownerSpec{uid: -1, gid: -1}
	if ownerPart != "" {
		uid, err := resolveUser(ownerPart)
		if err != nil {
			return ownerSpec{}, err
		}
		result.uid = uid
	}
	if groupPart != "" {
		gid, err := resolveGroup(groupPart)
		if err != nil {
			// R1.4: GNU reports the full spec in group errors.
			return ownerSpec{}, fmt.Errorf(
				"chown: invalid group: '%s'", fullSpec)
		}
		result.gid = gid
	} else if ownerPart != "" {
		// R1.1: OWNER: sets group to OWNER's login group.
		// GNU requires a symbolic username for login group lookup;
		// numeric UIDs are rejected as "invalid spec".
		if isNumeric(ownerPart) {
			return ownerSpec{}, fmt.Errorf(
				"chown: invalid spec: '%s'", fullSpec)
		}
		gid, err := loginGroupForUser(ownerPart)
		if err != nil {
			return ownerSpec{}, err
		}
		result.gid = gid
	}
	return result, nil
}

// isNumeric returns true if s is a non-negative decimal integer.
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// resolveUser converts a user name or numeric UID string to a numeric UID.
// R1.2: resolve names via the system user database.
func resolveUser(name string) (int, error) {
	if uid, err := strconv.Atoi(name); err == nil && uid >= 0 {
		return uid, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf(
			"chown: invalid user: '%s'", name)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf(
			"chown: invalid user: '%s'", name)
	}
	return uid, nil
}

// resolveGroup converts a group name or numeric GID string to a numeric GID.
// R1.2: resolve names via the system group database.
func resolveGroup(group string) (int, error) {
	if gid, err := strconv.Atoi(group); err == nil && gid >= 0 {
		return gid, nil
	}
	g, err := user.LookupGroupId(group)
	if err == nil {
		return strconv.Atoi(g.Gid)
	}
	g, err = user.LookupGroup(group)
	if err != nil {
		return 0, fmt.Errorf(
			"chown: invalid group: '%s'", group)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf(
			"chown: invalid group: '%s'", group)
	}
	return gid, nil
}

// loginGroupForUser returns the primary group of a user.
// R1.1: OWNER: form sets group to OWNER's login group.
func loginGroupForUser(name string) (int, error) {
	var u *user.User
	var err error
	if uid, parseErr := strconv.Atoi(name); parseErr == nil && uid >= 0 {
		u, err = user.LookupId(name)
	} else {
		u, err = user.Lookup(name)
	}
	if err != nil {
		return 0, fmt.Errorf(
			"chown: invalid user: '%s'", name)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, fmt.Errorf(
			"chown: failed to get login group for '%s'", name)
	}
	return gid, nil
}

// specFromReference reads owner and group from a reference file (R1.3).
func specFromReference(path string) (ownerSpec, error) {
	info, err := sys.Stat(path)
	if err != nil {
		return ownerSpec{}, fmt.Errorf(
			"chown: failed to get attributes of '%s': %w", path, err)
	}
	return ownerSpec{uid: int(info.Uid), gid: int(info.Gid)}, nil
}

// --- Core application logic (R1.4, R2.1) ---

// applyToFiles changes ownership on each file, returning exit code.
// R1.4: continue on error, exit 1 if any failed.
func applyToFiles(spec ownerSpec, files []string, f chownFlags) int {
	exitCode := 0
	for _, path := range files {
		var failed bool
		if f.recursive {
			failed = walkRecursive(spec, path, f, true)
		} else {
			if err := changeAndReport(spec, path, f); err != nil {
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

// walkRecursive initiates recursive ownership change on a path.
// R2.1: -R changes ownership recursively for directories and their contents.
// R2.3: respects -H/-L/-P for symlink dereference control.
func walkRecursive(spec ownerSpec, path string, f chownFlags, isCmdLine bool) bool {
	info, isSymlink, err := statForTraversal(path, f, isCmdLine)
	if err != nil {
		reportError(
			fmt.Errorf("chown: cannot access '%s': %w", path, err), f)
		return true
	}
	// R2.3: skip symlinks that should not be followed.
	if isSymlink && !shouldFollow(f, isCmdLine) {
		return changeSymlink(spec, path, f)
	}
	if info.IsDir() {
		return walkDir(spec, path, f)
	}
	if err := changeAndReport(spec, path, f); err != nil {
		reportError(err, f)
		return true
	}
	return false
}

// statForTraversal stats a path respecting dereference mode.
// Returns file info, whether it's a symlink, and any error.
func statForTraversal(path string, f chownFlags, isCmdLine bool) (fs.FileInfo, bool, error) {
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
func shouldFollow(f chownFlags, isCmdLine bool) bool {
	switch f.deref {
	case derefAll:
		return true
	case derefCmdLine:
		return isCmdLine
	default:
		return false
	}
}

// walkDir recursively applies ownership changes to a directory and contents.
// GNU processes subdirectories before files, then the directory itself.
func walkDir(spec ownerSpec, dirPath string, f chownFlags) bool {
	hadError := false
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		reportError(
			fmt.Errorf("chown: cannot read directory '%s': %w", dirPath, err), f)
		hadError = true
	}
	// Process subdirectories and symlinks first, then regular files.
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			child := filepath.Join(dirPath, entry.Name())
			if walkChild(spec, child, entry, f) {
				hadError = true
			}
		}
	}
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
			child := filepath.Join(dirPath, entry.Name())
			if walkChild(spec, child, entry, f) {
				hadError = true
			}
		}
	}
	if err := changeAndReport(spec, dirPath, f); err != nil {
		reportError(err, f)
		hadError = true
	}
	return hadError
}

// walkChild processes a single directory entry during recursive traversal.
func walkChild(spec ownerSpec, child string, entry fs.DirEntry, f chownFlags) bool {
	if entry.Type()&fs.ModeSymlink != 0 {
		return handleSymlinkChild(spec, child, f)
	}
	if entry.IsDir() {
		return walkDir(spec, child, f)
	}
	if err := changeAndReport(spec, child, f); err != nil {
		reportError(err, f)
		return true
	}
	return false
}

// handleSymlinkChild handles a symlink during recursive traversal.
// R2.3: with -L, follow symlinks to directories and recurse into them.
func handleSymlinkChild(spec ownerSpec, child string, f chownFlags) bool {
	if f.deref == derefAll {
		info, err := os.Stat(child)
		if err != nil {
			reportError(
				fmt.Errorf("chown: cannot access '%s': %w", child, err), f)
			return true
		}
		if info.IsDir() {
			return walkDir(spec, child, f)
		}
		return changeSymlink(spec, child, f)
	}
	return changeSymlink(spec, child, f)
}

// changeSymlink changes ownership of a symlink itself.
func changeSymlink(spec ownerSpec, path string, f chownFlags) bool {
	if err := changeAndReport(spec, path, f); err != nil {
		reportError(err, f)
		return true
	}
	return false
}

// --- Ownership change with diagnostics (R2.2, R3.1) ---

// changeAndReport changes ownership of a single file and prints diagnostics.
func changeAndReport(spec ownerSpec, path string, f chownFlags) error {
	info, err := sys.Lstat(path)
	if err != nil {
		return fmt.Errorf("chown: cannot access '%s': %w", path, err)
	}
	oldUID := int(info.Uid)
	oldGID := int(info.Gid)
	uid := resolveUID(spec, info)
	gid := resolveGID(spec, info)
	if err := chownFile(path, uid, gid, f); err != nil {
		return fmt.Errorf("chown: changing ownership of '%s': %w", path, err)
	}
	printDiagnostic(path, oldUID, oldGID, uid, gid, f)
	return nil
}

// resolveUID returns the target UID, keeping original if spec says -1.
func resolveUID(spec ownerSpec, info *sys.FileInfo) int {
	if spec.uid == -1 {
		return int(info.Uid)
	}
	return spec.uid
}

// resolveGID returns the target GID, keeping original if spec says -1.
func resolveGID(spec ownerSpec, info *sys.FileInfo) int {
	if spec.gid == -1 {
		return int(info.Gid)
	}
	return spec.gid
}

// chownFile calls the appropriate chown function based on dereference flags.
// R2.2: -h uses Lchown; without -h also uses Lchown (default behavior for chown).
func chownFile(path string, uid, gid int, f chownFlags) error {
	if f.noDerefer {
		return os.Lchown(path, uid, gid)
	}
	return os.Lchown(path, uid, gid)
}

// --- Diagnostic output (R3.1) ---

// resolveUserName looks up the user name for a UID for diagnostic output.
func resolveUserName(uid int) string {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return strconv.Itoa(uid)
	}
	return u.Username
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
// R3.1: -v prints for every file, -c prints only when changes are made.
func printDiagnostic(path string, oldUID, oldGID, newUID, newGID int, f chownFlags) {
	if !f.verbose && !f.changes {
		return
	}
	changed := oldUID != newUID || oldGID != newGID
	if f.changes && !changed {
		return
	}
	newOwner := resolveUserName(newUID)
	newGroup := resolveGroupName(newGID)
	ownerGroup := newOwner + ":" + newGroup
	if changed {
		fmt.Fprintf(os.Stdout,
			"changed ownership of '%s' to %s\n", path, ownerGroup)
		return
	}
	fmt.Fprintf(os.Stdout,
		"ownership of '%s' retained as %s\n", path, ownerGroup)
}

// reportError prints an error to stderr unless silent mode is active (R3.1).
func reportError(err error, f chownFlags) {
	if !f.silent {
		fmt.Fprintln(os.Stderr, err)
	}
}
