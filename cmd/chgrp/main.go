// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chgrp implements srd090-chgrp R1.1-R1.4, R2.1-R2.3, R3.1-R3.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: chgrp [OPTION]... GROUP FILE...
  or:  chgrp [OPTION]... --reference=RFILE FILE...
Change the group of each FILE to GROUP.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
      --dereference      affect the referent of each symbolic link (this is
                         the default), rather than the symbolic link itself
  -h, --no-dereference   affect symbolic links instead of any referenced file
      --no-preserve-root  do not treat '/' specially (the default)
      --preserve-root    fail to operate recursively on '/'
      --reference=RFILE  use RFILE's group rather than specifying a GROUP value
  -R, --recursive        operate on files and directories recursively
  -H                     if a command line argument is a symbolic link
                         to a directory, traverse it
  -L                     traverse every symbolic link to a directory
                         encountered
  -P                     do not traverse any symbolic links (default)
      --help     display this help and exit
      --version  output version information and exit

Examples:
  chgrp staff /u      Change the group of /u to "staff".
  chgrp -hR staff /u  Change the group of /u and subfiles to "staff".
`

const versionText = `chgrp (go-unix-utils) dev
`

type symlinkMode int

const (
	symlinkP symlinkMode = iota
	symlinkH
	symlinkL
)

type options struct {
	verbose       bool
	changes       bool
	silent        bool
	reference     string
	recursive     bool
	noDereference bool
	symlink       symlinkMode
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, group, files := parseArgs(os.Args[1:])
	gid := resolveGID(opts, group)
	groupLabel := group
	if opts.reference != "" {
		groupLabel = gidToName(gid)
	}
	if len(files) == 0 {
		if opts.reference != "" {
			fmt.Fprintln(os.Stderr, "chgrp: missing operand")
		} else {
			fmt.Fprintf(os.Stderr, "chgrp: missing operand after '%s'\n", group)
		}
		fmt.Fprintln(os.Stderr, "Try 'chgrp --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := changeGroup(opts, gid, groupLabel, file); err != nil {
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func resolveGID(opts options, group string) int {
	if opts.reference != "" {
		return resolveReference(opts.reference)
	}
	return lookupGroup(group)
}

func lookupGroup(group string) int {
	if gid, err := strconv.Atoi(group); err == nil {
		return gid
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chgrp: invalid group: '%s'\n", group)
		os.Exit(1)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chgrp: invalid group: '%s'\n", group)
		os.Exit(1)
	}
	return gid
}

func gidToName(gid int) string {
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		return strconv.Itoa(gid)
	}
	return g.Name
}

func resolveReference(rfile string) int {
	fi, err := sys.Stat(rfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chgrp: failed to get attributes of '%s': %s\n",
			rfile, err.(*os.PathError).Err)
		os.Exit(1)
	}
	return int(fi.Gid)
}

func changeGroup(opts options, gid int, groupLabel, path string) error {
	if !opts.recursive {
		return chownSingle(opts, gid, groupLabel, path)
	}
	return chownRecursive(opts, gid, groupLabel, path, true)
}

func chownSingle(opts options, gid int, groupLabel, path string) error {
	fn := os.Chown
	statFn := sys.Stat
	if opts.noDereference {
		fn = os.Lchown
		statFn = sys.Lstat
	}
	oldGID := -1
	if opts.verbose || opts.changes {
		if fi, err := statFn(path); err == nil {
			oldGID = int(fi.Gid)
		}
	}
	if err := fn(path, -1, gid); err != nil {
		reportError(opts, err)
		printDiagnostic(opts, path, oldGID, gid, groupLabel)
		return err
	}
	printDiagnostic(opts, path, oldGID, gid, groupLabel)
	return nil
}

func chownRecursive(opts options, gid int, groupLabel, path string, isTopLevel bool) error {
	follow := shouldFollowLink(opts, isTopLevel)
	fi, err := statPath(path, follow)
	if err != nil {
		reportError(opts, err)
		printDiagnostic(opts, path, -1, gid, groupLabel)
		return err
	}
	var firstErr error
	if fi.IsDir() {
		firstErr = walkChildren(opts, gid, groupLabel, path)
	}
	if err := applyChown(opts, gid, groupLabel, path); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func applyChown(opts options, gid int, groupLabel, path string) error {
	fn := os.Lchown
	statFn := sys.Lstat
	if opts.symlink == symlinkL {
		fn = os.Chown
		statFn = sys.Stat
	}
	oldGID := -1
	if opts.verbose || opts.changes {
		if fi, err := statFn(path); err == nil {
			oldGID = int(fi.Gid)
		}
	}
	if err := fn(path, -1, gid); err != nil {
		reportError(opts, err)
		printDiagnostic(opts, path, oldGID, gid, groupLabel)
		return err
	}
	printDiagnostic(opts, path, oldGID, gid, groupLabel)
	return nil
}

func shouldFollowLink(opts options, isTopLevel bool) bool {
	switch opts.symlink {
	case symlinkL:
		return true
	case symlinkH:
		return isTopLevel
	default:
		return false
	}
}

func statPath(path string, follow bool) (os.FileInfo, error) {
	if follow {
		return os.Stat(path)
	}
	return os.Lstat(path)
}

func walkChildren(opts options, gid int, groupLabel, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		reportError(opts, err)
		return err
	}
	var firstErr error
	for _, e := range entries {
		child := filepath.Join(dir, e.Name())
		if err := chownRecursive(opts, gid, groupLabel, child, false); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func printDiagnostic(opts options, path string, oldGID, newGID int, groupLabel string) {
	if !opts.verbose && !opts.changes {
		return
	}
	if oldGID < 0 {
		if opts.verbose {
			fmt.Fprintf(os.Stdout, "failed to change group of '%s' to %s\n", path, groupLabel)
		}
		return
	}
	if oldGID == newGID {
		if opts.verbose {
			fmt.Fprintf(os.Stdout, "group of '%s' retained as %s\n", path, gidToName(newGID))
		}
		return
	}
	fmt.Fprintf(os.Stdout, "changed group of '%s' from %s to %s\n",
		path, gidToName(oldGID), groupLabel)
}

func reportError(opts options, err error) {
	if opts.silent {
		return
	}
	fmt.Fprintf(os.Stderr, "chgrp: %s\n", formatErr(err))
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("changing group of '%s': %s", pe.Path, pe.Err)
	}
	return err.Error()
}

func parseArgs(args []string) (options, string, []string) {
	var opts options
	var files []string
	group := ""
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if handled := handleSpecialArg(arg, &endOfFlags); handled {
			continue
		}
		if parseLongFlag(arg, &opts) {
			continue
		}
		if group == "" {
			if strings.HasPrefix(arg, "-") && parseShortFlags(arg[1:], &opts) {
				continue
			}
			if opts.reference != "" {
				files = append(files, arg)
			} else {
				group = arg
			}
		} else {
			files = append(files, arg)
		}
	}
	if group == "" && opts.reference == "" {
		fmt.Fprintln(os.Stderr, "chgrp: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'chgrp --help' for more information.")
		os.Exit(1)
	}
	return opts, group, files
}

func handleSpecialArg(arg string, endOfFlags *bool) bool {
	switch arg {
	case "--help":
		fmt.Fprint(os.Stdout, helpText)
		os.Exit(0)
	case "--version":
		fmt.Fprint(os.Stdout, versionText)
		os.Exit(0)
	case "--":
		*endOfFlags = true
	default:
		return false
	}
	return true
}

func parseLongFlag(arg string, opts *options) bool {
	switch arg {
	case "--verbose":
		opts.verbose = true
	case "--changes":
		opts.changes = true
	case "--silent", "--quiet":
		opts.silent = true
	case "--recursive":
		opts.recursive = true
	case "--dereference":
		opts.noDereference = false
	case "--no-dereference":
		opts.noDereference = true
	case "--preserve-root", "--no-preserve-root":
	default:
		if strings.HasPrefix(arg, "--reference=") {
			opts.reference = arg[len("--reference="):]
			return true
		}
		return false
	}
	return true
}

func parseShortFlags(flags string, opts *options) bool {
	for _, c := range flags {
		switch c {
		case 'v':
			opts.verbose = true
		case 'c':
			opts.changes = true
		case 'f':
			opts.silent = true
		case 'R':
			opts.recursive = true
		case 'h':
			opts.noDereference = true
		case 'H':
			opts.symlink = symlinkH
		case 'L':
			opts.symlink = symlinkL
		case 'P':
			opts.symlink = symlinkP
		default:
			return false
		}
	}
	return true
}
