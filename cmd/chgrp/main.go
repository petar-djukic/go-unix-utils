// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chgrp implements GNU chgrp: change group ownership of files.
//
// Implements prd090-chgrp R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "chgrp"

const helpText = `Usage: chgrp [OPTION]... GROUP FILE...
  or:  chgrp [OPTION]... --reference=RFILE FILE...
Change the group of each FILE to GROUP.

  -c, --changes       like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose       output a diagnostic for every file processed
      --no-dereference  affect symbolic links instead of referenced files
  -h                  same as --no-dereference
  -R, --recursive     operate on files and directories recursively
  -H                  if a command line argument is a symbolic link to
                        a directory, traverse it (with -R)
  -L                  traverse every symbolic link to a directory encountered
                        (with -R)
  -P                  do not traverse any symbolic links (default with -R)
      --reference=RFILE  use RFILE's group rather than specifying a GROUP value
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "chgrp (go-unix-utils) 0.1\n"

// symMode controls symlink traversal during recursive operation.
type symMode int

const (
	symP symMode = iota // R2.3: never follow symlinks (default with -R)
	symH                // R2.3: follow symlinks on command line only
	symL                // R2.3: follow all symlinks
)

// options holds parsed command-line options.
type options struct {
	reference string  // R1.2: --reference=RFILE
	recursive bool    // R2.1: -R
	noDerefer bool    // R2.2: -h / --no-dereference
	symlink   symMode // R2.3: -H, -L, -P
	verbose   bool    // R3.1: -v
	changes   bool    // R3.1: -c
	silent    bool    // R3.1: -f
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes chgrp logic and returns the exit code.
func run(args []string, stdout, stderr *os.File) int {
	opts, operands, err := parseArgs(args)
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	if opts.reference != "" {
		return runWithReference(operands, opts, stdout, stderr)
	}
	return runWithGroup(operands, opts, stdout, stderr)
}

// runWithGroup handles the standard GROUP FILE... invocation.
func runWithGroup(operands []string, opts options, stdout, stderr *os.File) int {
	if len(operands) == 0 {
		printError(stderr, "missing operand")
		printTryHelp(stderr)
		return 1
	}
	if len(operands) == 1 {
		msg := fmt.Sprintf("missing operand after '%s'", operands[0])
		printError(stderr, msg)
		printTryHelp(stderr)
		return 1
	}
	gid, err := resolveGroup(operands[0])
	if err != nil {
		printError(stderr, fmt.Sprintf("invalid group: '%s'", operands[0]))
		return 1
	}
	return applyToFiles(operands[1:], gid, opts, stdout, stderr)
}

// runWithReference handles --reference=RFILE FILE... invocation.
func runWithReference(operands []string, opts options, stdout, stderr *os.File) int {
	if len(operands) == 0 {
		printError(stderr, "missing operand")
		printTryHelp(stderr)
		return 1
	}
	gid, err := getFileGID(opts.reference)
	if err != nil {
		msg := fmt.Sprintf("failed to get attributes of '%s': %s",
			opts.reference, sysErrorMsg(err))
		printError(stderr, msg)
		return 1
	}
	return applyToFiles(operands, gid, opts, stdout, stderr)
}

// applyToFiles changes group ownership for each file and returns exit code.
func applyToFiles(files []string, gid int, opts options, stdout, stderr *os.File) int {
	exitCode := 0
	for _, file := range files {
		if opts.recursive {
			if chgrpRecursive(file, gid, opts, stdout, stderr) != 0 {
				exitCode = 1
			}
		} else {
			if err := chgrpSingle(file, gid, opts, stdout); err != nil {
				if !opts.silent {
					printError(stderr, err.Error())
				}
				exitCode = 1
			}
		}
	}
	return exitCode
}

// chgrpSingle changes the group of a single file with verbose/changes output.
func chgrpSingle(path string, gid int, opts options, stdout *os.File) error {
	oldGid, err := getFileGIDForChange(path, opts.noDerefer)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %s", path, sysErrorMsg(err))
	}
	changed := oldGid != uint32(gid)
	if err := doChgrp(path, gid, opts.noDerefer); err != nil {
		return fmt.Errorf("cannot access '%s': %s", path, sysErrorMsg(err))
	}
	printDiag(stdout, opts, path, gid, oldGid, changed)
	return nil
}

// doChgrp performs the actual group change syscall.
// R2.2: with noDereference, changes the symlink itself via Lchown;
// without it, follows symlinks via Chown.
func doChgrp(path string, gid int, noDerefer bool) error {
	if noDerefer {
		return os.Lchown(path, -1, gid)
	}
	return os.Chown(path, -1, gid)
}

// chgrpRecursive walks a directory tree changing group ownership.
// R2.1: -R recursive traversal.
// R2.3: -H/-L/-P symlink traversal control.
func chgrpRecursive(root string, gid int, opts options, stdout, stderr *os.File) int {
	exitCode := 0
	// R2.3: -H follows command-line symlinks, -L follows all
	actualRoot := root
	if opts.symlink == symH || opts.symlink == symL {
		if resolved, err := resolveIfSymDir(root); err == nil {
			actualRoot = resolved
		}
	}
	walkPostOrder(actualRoot, root, gid, opts, stdout, stderr, &exitCode)
	return exitCode
}

// walkPostOrder recursively processes a directory in depth-first post-order
// to match GNU chgrp output ordering.
func walkPostOrder(
	realPath, dispPath string, gid int,
	opts options, stdout, stderr *os.File, exitCode *int,
) {
	entries, err := os.ReadDir(realPath)
	if err != nil {
		if !opts.silent {
			printError(stderr, fmt.Sprintf("cannot access '%s': %s", dispPath, sysErrorMsg(err)))
		}
		*exitCode = 1
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	processEntries(entries, realPath, dispPath, gid, opts, stdout, stderr, exitCode)
	// Post-order: change the directory itself last
	applyChange(realPath, dispPath, gid, false, opts, stdout, stderr, exitCode)
}

// processEntries handles each directory entry during recursive walk.
func processEntries(
	entries []os.DirEntry, realPath, dispPath string, gid int,
	opts options, stdout, stderr *os.File, exitCode *int,
) {
	for _, entry := range entries {
		childReal := filepath.Join(realPath, entry.Name())
		childDisp := joinDispPath(dispPath, entry.Name())
		processOneEntry(childReal, childDisp, entry, gid, opts, stdout, stderr, exitCode)
	}
}

// processOneEntry handles a single entry (file, dir, or symlink).
func processOneEntry(
	childReal, childDisp string, entry os.DirEntry, gid int,
	opts options, stdout, stderr *os.File, exitCode *int,
) {
	if entry.Type()&os.ModeSymlink != 0 {
		handleSymlink(childReal, childDisp, gid, opts, stdout, stderr, exitCode)
		return
	}
	if entry.IsDir() {
		walkPostOrder(childReal, childDisp, gid, opts, stdout, stderr, exitCode)
		return
	}
	applyChange(childReal, childDisp, gid, false, opts, stdout, stderr, exitCode)
}

// handleSymlink processes a symlink during recursive traversal.
func handleSymlink(
	realPath, dispPath string, gid int,
	opts options, stdout, stderr *os.File, exitCode *int,
) {
	// R2.3 -L: follow all symlinks
	if opts.symlink == symL {
		followSymlink(realPath, dispPath, gid, opts, stdout, stderr, exitCode)
		return
	}
	// R2.3 -P (default): change the symlink itself
	applyChange(realPath, dispPath, gid, true, opts, stdout, stderr, exitCode)
}

// followSymlink resolves and recurses into a symlinked directory.
func followSymlink(
	realPath, dispPath string, gid int,
	opts options, stdout, stderr *os.File, exitCode *int,
) {
	target, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		if !opts.silent {
			printError(stderr, fmt.Sprintf("cannot access '%s': %s", dispPath, sysErrorMsg(err)))
		}
		*exitCode = 1
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		if !opts.silent {
			printError(stderr, fmt.Sprintf("cannot access '%s': %s", dispPath, sysErrorMsg(err)))
		}
		*exitCode = 1
		return
	}
	if info.IsDir() {
		walkPostOrder(target, dispPath, gid, opts, stdout, stderr, exitCode)
		return
	}
	applyChange(target, dispPath, gid, false, opts, stdout, stderr, exitCode)
}

// applyChange changes group on a single path and prints diagnostics.
func applyChange(
	realPath, dispPath string, gid int, useLchown bool,
	opts options, stdout, stderr *os.File, exitCode *int,
) {
	oldGid, err := getFileGIDForChange(realPath, useLchown)
	if err != nil {
		if !opts.silent {
			printError(stderr, fmt.Sprintf("cannot access '%s': %s", dispPath, sysErrorMsg(err)))
		}
		*exitCode = 1
		return
	}
	changed := oldGid != uint32(gid)
	chownErr := os.Lchown(realPath, -1, gid)
	if chownErr != nil {
		if !opts.silent {
			printError(stderr, fmt.Sprintf("cannot access '%s': %s", dispPath, sysErrorMsg(chownErr)))
		}
		*exitCode = 1
		return
	}
	printDiag(stdout, opts, dispPath, gid, oldGid, changed)
}

// joinDispPath joins a display path with a child name, preserving "./" prefix.
func joinDispPath(parent, child string) string {
	if parent == "." {
		return "./" + child
	}
	return parent + "/" + child
}

// resolveIfSymDir resolves path if it's a symlink to a directory.
func resolveIfSymDir(path string) (string, error) {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("not a symlink to directory")
	}
	return target, nil
}

// getFileGIDForChange returns the GID of a file, using Lstat or Stat.
func getFileGIDForChange(path string, lstat bool) (uint32, error) {
	var fi *sys.FileInfo
	var err error
	if lstat {
		fi, err = sys.Lstat(path)
	} else {
		fi, err = sys.Stat(path)
	}
	if err != nil {
		return 0, err
	}
	return fi.Gid, nil
}

// printDiag prints verbose or changes diagnostic output.
// R3.1: -v prints every file, -c prints only changed files.
func printDiag(stdout *os.File, opts options, path string, newGid int, oldGid uint32, changed bool) {
	if opts.verbose {
		printGroupMsg(stdout, path, newGid, oldGid, changed)
	} else if opts.changes && changed {
		printGroupMsg(stdout, path, newGid, oldGid, changed)
	}
}

// printGroupMsg prints the group change diagnostic message.
func printGroupMsg(stdout *os.File, path string, newGid int, oldGid uint32, changed bool) {
	oldName := groupName(oldGid)
	newName := groupName(uint32(newGid))
	if changed {
		fmt.Fprintf(stdout, "changed group of '%s' from %s to %s\n", path, oldName, newName) //nolint:errcheck
	} else {
		fmt.Fprintf(stdout, "group of '%s' retained as %s\n", path, newName) //nolint:errcheck
	}
}

// groupName returns the group name for a GID, or the GID as a string.
func groupName(gid uint32) string {
	grp, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	if err != nil {
		return strconv.Itoa(int(gid))
	}
	return grp.Name
}

// resolveGroup resolves a group name or numeric GID string to a numeric GID.
func resolveGroup(group string) (int, error) {
	if gid, err := strconv.Atoi(group); err == nil {
		return gid, nil
	}
	grp, err := user.LookupGroup(group)
	if err != nil {
		return 0, fmt.Errorf("invalid group: '%s'", group)
	}
	return strconv.Atoi(grp.Gid)
}

// getFileGID returns the group ID of the given file.
func getFileGID(path string) (int, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		return 0, err
	}
	return int(fi.Gid), nil
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var operands []string
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		var err error
		opts, err = handleFlag(arg, opts)
		if err != nil {
			return opts, nil, err
		}
	}
	return opts, operands, nil
}

// handleFlag processes a single flag argument.
func handleFlag(arg string, opts options) (options, error) {
	if strings.HasPrefix(arg, "--reference=") {
		opts.reference = arg[len("--reference="):]
		return opts, nil
	}
	switch arg {
	case "--help":
		fmt.Fprint(os.Stdout, helpText) //nolint:errcheck
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText) //nolint:errcheck
		os.Exit(0)
	case "-R", "--recursive":
		opts.recursive = true
	case "-h", "--no-dereference":
		opts.noDerefer = true
	case "-H":
		opts.symlink = symH
	case "-L":
		opts.symlink = symL
	case "-P":
		opts.symlink = symP
	case "-v", "--verbose":
		opts.verbose = true
	case "-c", "--changes":
		opts.changes = true
	case "-f", "--silent", "--quiet":
		opts.silent = true
	default:
		return handleShortFlags(arg, opts)
	}
	return opts, nil
}

// handleShortFlags processes combined short flags like -Rv, -hc.
func handleShortFlags(arg string, opts options) (options, error) {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return opts, fmt.Errorf("unrecognized option '%s'", arg)
	}
	for _, ch := range arg[1:] {
		var err error
		opts, err = applySingleFlag(ch, opts)
		if err != nil {
			return opts, err
		}
	}
	return opts, nil
}

// applySingleFlag applies a single short flag character.
func applySingleFlag(ch rune, opts options) (options, error) {
	switch ch {
	case 'R':
		opts.recursive = true
	case 'h':
		opts.noDerefer = true
	case 'H':
		opts.symlink = symH
	case 'L':
		opts.symlink = symL
	case 'P':
		opts.symlink = symP
	case 'v':
		opts.verbose = true
	case 'c':
		opts.changes = true
	case 'f':
		opts.silent = true
	default:
		return opts, fmt.Errorf("invalid option -- '%c'", ch)
	}
	return opts, nil
}

// isFlag returns true if arg starts with '-' and has content after it.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// sysErrorMsg extracts the underlying system error message from a Go error.
func sysErrorMsg(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return capitalizeFirst(pathErr.Err.Error())
	}
	return err.Error()
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printError prints a formatted error to stderr.
func printError(stderr *os.File, msg string) {
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}

// printTryHelp prints the "Try ... --help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}
