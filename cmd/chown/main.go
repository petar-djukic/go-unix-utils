// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/chown: change file owner and group.
// Implements srd091 R1.1-R1.4 (ownership specification),
// R2.1-R2.3 (recursive and symlink handling),
// R3.1-R3.3 (output control and exit codes).
package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "chown"

// errAlreadyReported signals that errors were already printed to stderr
// by the recursive walker, so the caller should not print again.
var errAlreadyReported = fmt.Errorf("errors already reported")

// symlinkPolicy controls how symlinks are handled during recursive traversal.
type symlinkPolicy int

const (
	symlinkNone    symlinkPolicy = iota // -P: don't follow symlinks (default with -R)
	symlinkCmdLine                      // -H: follow command-line symlinks only
	symlinkAll                          // -L: follow all symlinks
)

// TODO: Task requested --preserve-root/--no-preserve-root and
// --from=CURRENT_OWNER:CURRENT_GROUP, but srd091 non_goals explicitly
// excludes both. Skipped per E6.

// ownerSpec holds the parsed OWNER[:GROUP] specification.
// R1.1: supports OWNER, OWNER:GROUP, OWNER:, :GROUP forms.
type ownerSpec struct {
	uid       int  // target UID (-1 means unchanged)
	gid       int  // target GID (-1 means unchanged)
	changeUID bool // whether to change the owner
	changeGID bool // whether to change the group
}

// options holds parsed command-line flags for chown.
type options struct {
	recursive   bool          // R2.1: -R/--recursive
	verbose     bool          // R3.1: -v/--verbose
	changes     bool          // R3.1: -c/--changes
	silent      bool          // R3.1: -f/--silent/--quiet
	noDerefer   bool          // R2.2: -h/--no-dereference
	reference   string        // R1.3: --reference=RFILE
	symlinks    symlinkPolicy // R2.3: -H/-L/-P symlink traversal
	dereference bool          // R2.2: --dereference (default behavior)
}

// R3.3, R1.1: main entry with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()

	if handleSpecialFlags(os.Args[1:]) {
		return
	}

	opts, ownerArg, files := parseArgs(os.Args[1:])
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		os.Exit(1)
	}

	spec, err := resolveOwnerSpec(opts, ownerArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	exitCode := run(opts, spec, files)
	os.Exit(exitCode)
}

// handleSpecialFlags checks for --version and --help flags.
// R4: --version and --help produce GNU-compatible output.
func handleSpecialFlags(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--version" {
			printVersion()
			return true
		}
		if arg == "--help" {
			printHelp()
			return true
		}
	}
	return false
}

// printVersion outputs version information and exits 0.
func printVersion() {
	fmt.Printf("%s (go-unix-utils) 0.1\n", programName)
}

// printHelp outputs usage information and exits 0.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION]... [OWNER][:[GROUP]] FILE...\n", programName)
	fmt.Printf("  or:  %s [OPTION]... --reference=RFILE FILE...\n", programName)
	fmt.Println("Change the owner and/or group of each FILE to OWNER and/or GROUP.")
	fmt.Println()
	fmt.Println("  -c, --changes       like verbose but report only when a change is made")
	fmt.Println("  -f, --silent, --quiet  suppress most error messages")
	fmt.Println("  -v, --verbose       output a diagnostic for every file processed")
	fmt.Println("      --dereference   affect the referent of each symbolic link (default)")
	fmt.Println("  -h, --no-dereference  affect symbolic links instead of any referenced file")
	fmt.Println("      --reference=RFILE  use RFILE's owner and group rather than specifying values")
	fmt.Println("  -R, --recursive     operate on files and directories recursively")
	fmt.Println("  -H                  if -R, follow symlinks on the command line")
	fmt.Println("  -L                  if -R, follow all symlinks")
	fmt.Println("  -P                  if -R, never follow symlinks (default)")
	fmt.Println("      --help          display this help and exit")
	fmt.Println("      --version       output version information and exit")
}

// parseArgs separates flags, owner:group argument, and file operands.
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

	// R1.3: when --reference is used, all positional args are files.
	// R1.1: otherwise, first positional is OWNER[:GROUP], rest are files.
	if opts.reference != "" {
		return opts, "", positional
	}
	if len(positional) == 0 {
		return opts, "", nil
	}
	return opts, positional[0], positional[1:]
}

// isShortFlags checks if arg (without leading -) contains only
// valid short flag characters for chown.
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

// parseLongFlag handles long-form flags for chown.
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
		// R1.3: --reference=RFILE
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

// resolveOwnerSpec determines the target UID/GID from --reference or
// the OWNER[:GROUP] argument.
// R1.1: OWNER, OWNER:GROUP, OWNER:, :GROUP forms.
// R1.3: --reference=RFILE.
func resolveOwnerSpec(opts options, ownerArg string) (ownerSpec, error) {
	if opts.reference != "" {
		return ownerSpecFromReference(opts.reference)
	}
	return parseOwnerGroup(ownerArg)
}

// ownerSpecFromReference reads owner and group from a reference file.
// R1.3: --reference=RFILE sets each FILE's owner and group to match RFILE's.
func ownerSpecFromReference(rfile string) (ownerSpec, error) {
	fi, err := sys.Stat(rfile)
	if err != nil {
		return ownerSpec{}, fmt.Errorf(
			"failed to get attributes of %q: %s",
			rfile, unwrapPathError(err))
	}
	return ownerSpec{
		uid:       int(fi.Uid),
		gid:       int(fi.Gid),
		changeUID: true,
		changeGID: true,
	}, nil
}

// parseOwnerGroup parses the OWNER[:GROUP] argument.
// R1.1: OWNER changes owner only. OWNER:GROUP changes both.
// OWNER: changes owner and sets group to OWNER's login group.
// :GROUP changes group only.
// D1: colon is the separator; leading colon = group only;
// trailing colon = owner + login group.
func parseOwnerGroup(arg string) (ownerSpec, error) {
	spec := ownerSpec{uid: -1, gid: -1}

	ownerPart, groupPart, hasColon := strings.Cut(arg, ":")
	if !hasColon {
		// OWNER only (no colon)
		uid, err := resolveUser(arg)
		if err != nil {
			return spec, err
		}
		spec.uid = uid
		spec.changeUID = true
		return spec, nil
	}

	return resolveOwnerGroupParts(spec, ownerPart, groupPart)
}

// resolveOwnerGroupParts resolves the owner and group parts of OWNER:GROUP.
func resolveOwnerGroupParts(
	spec ownerSpec, ownerPart, groupPart string,
) (ownerSpec, error) {
	if ownerPart != "" {
		uid, err := resolveUser(ownerPart)
		if err != nil {
			return spec, err
		}
		spec.uid = uid
		spec.changeUID = true
	}

	if groupPart != "" {
		// OWNER:GROUP or :GROUP
		gid, err := resolveGroup(groupPart)
		if err != nil {
			return spec, err
		}
		spec.gid = gid
		spec.changeGID = true
	} else if ownerPart != "" {
		// OWNER: (trailing colon, set group to owner's login group)
		gid, err := loginGroupForUser(ownerPart)
		if err != nil {
			return spec, err
		}
		spec.gid = gid
		spec.changeGID = true
	}

	return spec, nil
}

// resolveUser resolves a user name or numeric UID to an integer UID.
// R1.2: OWNER may be a name or numeric ID.
// D2: uses os/user.Lookup with fallback to numeric IDs.
func resolveUser(name string) (int, error) {
	if uid, err := strconv.Atoi(name); err == nil {
		return uid, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("invalid user: '%s'", name)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID %q for user %q", u.Uid, name)
	}
	return uid, nil
}

// resolveGroup resolves a group name or numeric GID to an integer GID.
// R1.2: GROUP may be a name or numeric ID.
// D2: uses os/user.LookupGroup with fallback to numeric IDs.
func resolveGroup(name string) (int, error) {
	if gid, err := strconv.Atoi(name); err == nil {
		return gid, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("invalid group: '%s'", name)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("invalid group ID %q for group %q", g.Gid, name)
	}
	return gid, nil
}

// loginGroupForUser returns the primary group GID for a user.
// D1: OWNER: form sets group to the owner's login group.
func loginGroupForUser(name string) (int, error) {
	var u *user.User
	var err error

	if uid, numErr := strconv.Atoi(name); numErr == nil {
		u, err = user.LookupId(strconv.Itoa(uid))
	} else {
		u, err = user.Lookup(name)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get login group for '%s'", name)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, fmt.Errorf("invalid group ID %q for user %q", u.Gid, name)
	}
	return gid, nil
}

// run applies the ownership change to all files and returns the exit code.
// R1.4: continues processing remaining files on error, exits 1.
// R3.2: exits 0 when all files processed successfully, 1 on any error.
func run(opts options, spec ownerSpec, files []string) int {
	exitCode := 0
	for _, file := range files {
		if err := applyOwner(opts, spec, file); err != nil {
			if !opts.silent && err != errAlreadyReported {
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			}
			exitCode = 1
		}
	}
	return exitCode
}

// applyOwner applies the ownership change to a single file or recursively.
// R2.1: when recursive, traverses directories.
func applyOwner(opts options, spec ownerSpec, path string) error {
	if opts.recursive {
		return applyOwnerRecursive(opts, spec, path)
	}
	return changeOwner(opts, spec, path)
}

// applyOwnerRecursive recursively applies ownership changes.
// R2.1: -R/--recursive changes owner/group for directories and contents.
func applyOwnerRecursive(opts options, spec ownerSpec, root string) error {
	hadError := false
	walkChown(opts, spec, root, true, &hadError)
	if hadError {
		return errAlreadyReported
	}
	return nil
}

// walkChown recursively applies ownership changes, respecting symlink policy.
// R2.3: -P (default) skips symlinks. -H follows command-line symlinks.
// -L follows all symlinks.
func walkChown(opts options, spec ownerSpec, path string, isRoot bool, hadErr *bool) {
	fi, isSymlink := resolveEntry(opts, path, isRoot, hadErr)
	if fi == nil {
		return
	}
	if isSymlink && !shouldFollowSymlink(opts.symlinks, isRoot) {
		changeSymlinkOwner(opts, spec, path, hadErr)
		return
	}
	if err := changeOwner(opts, spec, path); err != nil {
		reportFileError(opts, err, hadErr)
	}
	if fi.IsDir() {
		walkChildren(opts, spec, path, hadErr)
	}
}

// resolveEntry checks the path and decides how to process it.
func resolveEntry(opts options, path string, isRoot bool, hadErr *bool) (os.FileInfo, bool) {
	lfi, err := os.Lstat(path)
	if err != nil {
		reportWalkError(opts, path, err, hadErr)
		return nil, false
	}
	if lfi.Mode()&os.ModeSymlink == 0 {
		return lfi, false
	}
	if !shouldFollowSymlink(opts.symlinks, isRoot) {
		return lfi, true
	}
	fi, err := os.Stat(path)
	if err != nil {
		reportWalkError(opts, path, err, hadErr)
		return nil, false
	}
	return fi, true
}

// shouldFollowSymlink returns true if the symlink should be followed.
// R2.3: -H follows command-line only, -L follows all, -P follows none.
func shouldFollowSymlink(policy symlinkPolicy, isRoot bool) bool {
	return policy == symlinkAll || (policy == symlinkCmdLine && isRoot)
}

// changeSymlinkOwner changes the owner/group of a symlink itself via lchown.
// R2.2: --no-dereference changes the symlink, not its target.
func changeSymlinkOwner(opts options, spec ownerSpec, path string, hadErr *bool) {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportWalkError(opts, path, err, hadErr)
		return
	}
	uid, gid := computeIDs(spec, int(fi.Uid), int(fi.Gid))
	if err := os.Lchown(path, uid, gid); err != nil {
		reportWalkError(opts, path, err, hadErr)
		return
	}
	printDiagnostic(opts, path, int(fi.Uid), int(fi.Gid), uid, gid)
}

// walkChildren reads directory entries and recurses into each child.
func walkChildren(opts options, spec ownerSpec, dir string, hadErr *bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		reportWalkError(opts, dir, err, hadErr)
		return
	}
	for _, e := range entries {
		walkChown(opts, spec, filepath.Join(dir, e.Name()), false, hadErr)
	}
}

// changeOwner changes the owner and/or group of a single file.
// R1.1: changes owner/group based on ownerSpec.
func changeOwner(opts options, spec ownerSpec, path string) error {
	fi, err := statForOpts(opts, path)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %s",
			path, unwrapPathError(err))
	}

	oldUID := int(fi.Uid)
	oldGID := int(fi.Gid)
	uid, gid := computeIDs(spec, oldUID, oldGID)

	if err := chownForOpts(opts, path, uid, gid); err != nil {
		return fmt.Errorf("changing ownership of '%s': %s",
			path, unwrapPathError(err))
	}

	printDiagnostic(opts, path, oldUID, oldGID, uid, gid)
	return nil
}

// computeIDs determines the final UID and GID based on the ownerSpec.
func computeIDs(spec ownerSpec, currentUID, currentGID int) (int, int) {
	uid := currentUID
	gid := currentGID
	if spec.changeUID {
		uid = spec.uid
	}
	if spec.changeGID {
		gid = spec.gid
	}
	return uid, gid
}

// statForOpts calls sys.Lstat or sys.Stat depending on dereference mode.
// R2.2: --no-dereference uses Lstat; default (dereference) uses Stat.
func statForOpts(opts options, path string) (*sys.FileInfo, error) {
	if opts.noDerefer {
		return sys.Lstat(path)
	}
	return sys.Stat(path)
}

// chownForOpts calls os.Lchown or os.Chown depending on dereference mode.
func chownForOpts(opts options, path string, uid, gid int) error {
	if opts.noDerefer {
		return os.Lchown(path, uid, gid)
	}
	return os.Chown(path, uid, gid)
}

// printDiagnostic prints a verbose or changes-only diagnostic message.
// R3.1: -v prints for every file. -c prints only when changes are made.
func printDiagnostic(
	opts options, path string,
	oldUID, oldGID, newUID, newGID int,
) {
	if !opts.verbose && !opts.changes {
		return
	}
	changed := oldUID != newUID || oldGID != newGID
	if opts.changes && !changed {
		return
	}
	oldOwner := formatOwnership(oldUID, oldGID)
	newOwner := formatOwnership(newUID, newGID)
	if changed {
		fmt.Fprintf(os.Stdout,
			"changed ownership of '%s' from %s to %s\n",
			path, oldOwner, newOwner)
	} else {
		fmt.Fprintf(os.Stdout,
			"ownership of '%s' retained as %s\n",
			path, newOwner)
	}
}

// formatOwnership formats a UID:GID pair as "user:group" for diagnostics.
func formatOwnership(uid, gid int) string {
	return userName(uid) + ":" + groupName(gid)
}

// userName returns the user name for a UID, or the numeric string.
func userName(uid int) string {
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return strconv.Itoa(uid)
	}
	return u.Username
}

// groupName returns the group name for a GID, or the numeric string.
func groupName(gid int) string {
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return strconv.Itoa(gid)
	}
	return g.Name
}

// reportWalkError prints a directory traversal error to stderr.
func reportWalkError(opts options, path string, err error, hadErr *bool) {
	if !opts.silent {
		fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %s\n",
			programName, path, unwrapPathError(err))
	}
	*hadErr = true
}

// reportFileError prints a file ownership change error to stderr.
func reportFileError(opts options, err error, hadErr *bool) {
	if !opts.silent {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
	}
	*hadErr = true
}

// unwrapPathError extracts the underlying error message from *os.PathError.
func unwrapPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
