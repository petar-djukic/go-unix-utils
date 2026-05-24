// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chown implements srd091-chown R1.1-R1.4, R2.1-R2.3.
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

const helpText = `Usage: chown [OPTION]... [OWNER][:[GROUP]] FILE...
  or:  chown [OPTION]... --reference=RFILE FILE...
Change the owner and/or group of each FILE to OWNER and/or GROUP.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
      --dereference      affect the referent of each symbolic link (this is
                         the default), rather than the symbolic link itself
  -h, --no-dereference   affect symbolic links instead of any referenced file
      --no-preserve-root  do not treat '/' specially (the default)
      --preserve-root    fail to operate recursively on '/'
      --reference=RFILE  use RFILE's owner and group rather than specifying
                         OWNER:GROUP values
  -R, --recursive        operate on files and directories recursively
  -H                     if a command line argument is a symbolic link
                         to a directory, traverse it
  -L                     traverse every symbolic link to a directory
                         encountered
  -P                     do not traverse any symbolic links (default)
      --help     display this help and exit
      --version  output version information and exit

Owner is unchanged if missing.  Group is unchanged if missing, but changed
to login group if implied by a ':' following a symbolic OWNER.
OWNER and GROUP may be numeric as well as symbolic.

Examples:
  chown root /u        Change the owner of /u to "root".
  chown root:staff /u  Change the owner of /u to "root" and the
                       group to "staff".
  chown -hR root /u    Change the owner of /u and subfiles to "root".
`

const versionText = `chown (go-unix-utils) dev
`

type symlinkMode int

const (
	symlinkP symlinkMode = iota
	symlinkH
	symlinkL
)

type options struct {
	reference     string
	recursive     bool
	noDereference bool
	symlink       symlinkMode
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, ownerSpec, files := parseArgs(os.Args[1:])
	uid, gid := resolveOwnership(opts, ownerSpec)
	if len(files) == 0 {
		if opts.reference != "" {
			fmt.Fprintln(os.Stderr, "chown: missing operand")
		} else {
			fmt.Fprintf(os.Stderr, "chown: missing operand after '%s'\n", ownerSpec)
		}
		fmt.Fprintln(os.Stderr, "Try 'chown --help' for more information.")
		os.Exit(1)
	}

	exitCode := 0
	for _, file := range files {
		if err := changeOwnership(opts, uid, gid, file); err != nil {
			exitCode = 1
		}
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func changeOwnership(opts options, uid, gid int, path string) error {
	if !opts.recursive {
		return chownSingle(opts, uid, gid, path)
	}
	return chownRecursive(opts, uid, gid, path, true)
}

func chownSingle(opts options, uid, gid int, path string) error {
	fn := os.Chown
	if opts.noDereference {
		fn = os.Lchown
	}
	if err := fn(path, uid, gid); err != nil {
		reportError(err)
		return err
	}
	return nil
}

func chownRecursive(opts options, uid, gid int, path string, isTopLevel bool) error {
	follow := shouldFollowLink(opts, isTopLevel)
	fi, err := statPath(path, follow)
	if err != nil {
		reportError(err)
		return err
	}
	var firstErr error
	if fi.IsDir() {
		firstErr = walkChildren(opts, uid, gid, path)
	}
	if err := applyChown(opts, uid, gid, path); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func applyChown(opts options, uid, gid int, path string) error {
	fn := os.Lchown
	if opts.symlink == symlinkL {
		fn = os.Chown
	}
	if err := fn(path, uid, gid); err != nil {
		reportError(err)
		return err
	}
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

func walkChildren(opts options, uid, gid int, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		reportError(err)
		return err
	}
	var firstErr error
	for _, e := range entries {
		child := filepath.Join(dir, e.Name())
		if err := chownRecursive(opts, uid, gid, child, false); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func resolveOwnership(opts options, ownerSpec string) (int, int) {
	if opts.reference != "" {
		return resolveReference(opts.reference)
	}
	return parseOwnerSpec(ownerSpec)
}

func parseOwnerSpec(spec string) (int, int) {
	owner, group, hasColon := strings.Cut(spec, ":")
	if !hasColon {
		return lookupUser(spec), -1
	}
	uid := -1
	if owner != "" {
		uid = lookupUser(owner)
	}
	if group != "" {
		return uid, lookupGroup(group, spec)
	}
	if owner == "" {
		return -1, -1
	}
	return uid, lookupLoginGroup(owner)
}

func lookupUser(name string) int {
	if uid, err := strconv.Atoi(name); err == nil {
		return uid
	}
	u, err := user.Lookup(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chown: invalid user: '%s'\n", name)
		os.Exit(1)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chown: invalid user: '%s'\n", name)
		os.Exit(1)
	}
	return uid
}

func lookupGroup(name string, spec string) int {
	if gid, err := strconv.Atoi(name); err == nil {
		return gid
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chown: invalid group: '%s'\n", spec)
		os.Exit(1)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chown: invalid group: '%s'\n", spec)
		os.Exit(1)
	}
	return gid
}

func lookupLoginGroup(owner string) int {
	u, err := user.Lookup(owner)
	if err != nil {
		u, err = user.LookupId(owner)
		if err != nil {
			fmt.Fprintf(os.Stderr, "chown: invalid user: '%s'\n", owner)
			os.Exit(1)
		}
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chown: invalid user: '%s'\n", owner)
		os.Exit(1)
	}
	return gid
}

func resolveReference(rfile string) (int, int) {
	fi, err := sys.Stat(rfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "chown: failed to get attributes of '%s': %s\n",
			rfile, err.(*os.PathError).Err)
		os.Exit(1)
	}
	return int(fi.Uid), int(fi.Gid)
}

func reportError(err error) {
	fmt.Fprintf(os.Stderr, "chown: %s\n", formatErr(err))
}

func formatErr(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Sprintf("changing ownership of '%s': %s", pe.Path, pe.Err)
	}
	return err.Error()
}

func parseArgs(args []string) (options, string, []string) {
	var opts options
	var files []string
	ownerSpec := ""
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
		if ownerSpec == "" {
			if strings.HasPrefix(arg, "-") && len(arg) > 1 && parseShortFlags(arg[1:], &opts) {
				continue
			}
			if opts.reference != "" {
				files = append(files, arg)
			} else {
				ownerSpec = arg
			}
		} else {
			files = append(files, arg)
		}
	}
	if ownerSpec == "" && opts.reference == "" {
		fmt.Fprintln(os.Stderr, "chown: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'chown --help' for more information.")
		os.Exit(1)
	}
	return opts, ownerSpec, files
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
	case "--verbose", "--changes", "--silent", "--quiet":
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
		case 'v', 'c', 'f':
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
