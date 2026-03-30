// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chown implements GNU chown: change file owner and group.
//
// Implements prd091-chown R1.1, R1.2, R1.3, R1.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "chown"

const helpText = `Usage: chown [OPTION]... [OWNER][:[GROUP]] FILE...
  or:  chown [OPTION]... --reference=RFILE FILE...
Change the owner and/or group of each FILE to OWNER and/or GROUP.

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
      --reference=RFILE  use RFILE's owner and group rather than specifying
                        OWNER:GROUP values
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "chown (go-unix-utils) 0.1\n"

// ownerSpec holds parsed OWNER:GROUP specification.
// R1.1: uid or gid of -1 means unchanged.
type ownerSpec struct {
	uid int
	gid int
}

// options holds parsed command-line options.
type options struct {
	reference string // R1.3: --reference=RFILE
	recursive bool   // parsed for future -R support
	noDerefer bool   // -h / --no-dereference
	verbose   bool   // parsed for future -v support
	changes   bool   // parsed for future -c support
	silent    bool   // R1.4: -f suppresses errors
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes chown logic and returns the exit code.
func run(args []string, stdout, stderr *os.File) int {
	opts, operands, err := parseArgs(args)
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	if opts.reference != "" {
		return runWithReference(operands, opts, stdout, stderr)
	}
	return runWithOwner(operands, opts, stdout, stderr)
}

// runWithOwner handles the standard OWNER[:GROUP] FILE... invocation.
func runWithOwner(operands []string, opts options, stdout, stderr *os.File) int {
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
	spec, err := parseOwnerGroup(operands[0])
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	return applyToFiles(operands[1:], spec, opts, stdout, stderr)
}

// runWithReference handles --reference=RFILE FILE... invocation.
// R1.3: set each FILE's owner and group to match RFILE's.
func runWithReference(operands []string, opts options, stdout, stderr *os.File) int {
	if len(operands) == 0 {
		printError(stderr, "missing operand")
		printTryHelp(stderr)
		return 1
	}
	spec, err := getFileOwnerGroup(opts.reference)
	if err != nil {
		msg := fmt.Sprintf("failed to get attributes of '%s': %s",
			opts.reference, sysErrorMsg(err))
		printError(stderr, msg)
		return 1
	}
	return applyToFiles(operands, spec, opts, stdout, stderr)
}

// parseOwnerGroup parses OWNER[:GROUP] into ownerSpec.
// R1.1: supports OWNER, :GROUP, OWNER:GROUP, and OWNER: forms.
func parseOwnerGroup(spec string) (ownerSpec, error) {
	ownerPart, groupPart, hasColon := strings.Cut(spec, ":")
	if !hasColon {
		// OWNER only — change owner, leave group unchanged
		uid, err := resolveUser(spec)
		if err != nil {
			return ownerSpec{}, err
		}
		return ownerSpec{uid: uid, gid: -1}, nil
	}
	return resolveOwnerGroupParts(ownerPart, groupPart)
}

// resolveOwnerGroupParts resolves the owner and group parts after splitting.
func resolveOwnerGroupParts(ownerPart, groupPart string) (ownerSpec, error) {
	uid := -1
	gid := -1
	if ownerPart != "" {
		var err error
		uid, err = resolveUser(ownerPart)
		if err != nil {
			return ownerSpec{}, err
		}
	}
	if groupPart != "" {
		var err error
		gid, err = resolveGroup(groupPart)
		if err != nil {
			return ownerSpec{}, err
		}
	} else if ownerPart != "" {
		// R1.1: OWNER: form — set group to owner's login group
		var err error
		gid, err = loginGroupForUser(ownerPart)
		if err != nil {
			return ownerSpec{}, err
		}
	}
	return ownerSpec{uid: uid, gid: gid}, nil
}

// applyToFiles changes ownership for each file and returns exit code.
// R1.4: continues processing remaining files on error.
func applyToFiles(files []string, spec ownerSpec, opts options, _ *os.File, stderr *os.File) int {
	exitCode := 0
	for _, file := range files {
		if err := chownSingle(file, spec, opts.noDerefer); err != nil {
			if !opts.silent {
				printError(stderr, err.Error())
			}
			exitCode = 1
		}
	}
	return exitCode
}

// chownSingle changes ownership of a single file.
func chownSingle(path string, spec ownerSpec, noDerefer bool) error {
	if err := checkAccess(path, noDerefer); err != nil {
		return err
	}
	return doChown(path, spec, noDerefer)
}

// checkAccess verifies file accessibility for GNU-compatible error messages.
func checkAccess(path string, noDerefer bool) error {
	var err error
	if noDerefer {
		_, err = sys.Lstat(path)
	} else {
		_, err = sys.Stat(path)
	}
	if err != nil {
		return fmt.Errorf("cannot access '%s': %s", path, sysErrorMsg(err))
	}
	return nil
}

// doChown performs the actual ownership change syscall.
func doChown(path string, spec ownerSpec, noDerefer bool) error {
	var err error
	if noDerefer {
		err = os.Lchown(path, spec.uid, spec.gid)
	} else {
		err = os.Chown(path, spec.uid, spec.gid)
	}
	if err != nil {
		return fmt.Errorf("changing ownership of '%s': %s", path, sysErrorMsg(err))
	}
	return nil
}

// resolveUser resolves a username or numeric UID to a numeric UID.
// R1.2: name-to-ID resolution via os/user.
func resolveUser(name string) (int, error) {
	if uid, err := strconv.Atoi(name); err == nil {
		return uid, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("invalid user: '%s'", name)
	}
	return strconv.Atoi(u.Uid)
}

// resolveGroup resolves a group name or numeric GID to a numeric GID.
// R1.2: name-to-ID resolution via os/user.
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

// loginGroupForUser returns the primary GID for the given user.
// R1.1: OWNER: form sets group to owner's login group.
func loginGroupForUser(name string) (int, error) {
	if _, err := strconv.Atoi(name); err == nil {
		u, lookupErr := user.LookupId(name)
		if lookupErr != nil {
			return 0, fmt.Errorf("invalid user: '%s'", name)
		}
		return strconv.Atoi(u.Gid)
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("invalid user: '%s'", name)
	}
	return strconv.Atoi(u.Gid)
}

// getFileOwnerGroup returns the UID and GID of a file.
// R1.3: --reference uses the referenced file's owner and group.
func getFileOwnerGroup(path string) (ownerSpec, error) {
	fi, err := sys.Stat(path)
	if err != nil {
		return ownerSpec{}, err
	}
	return ownerSpec{uid: int(fi.Uid), gid: int(fi.Gid)}, nil
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
	case "-H", "-L", "-P":
		// Symlink traversal flags — parsed for future -R support
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
	case 'H', 'L', 'P':
		// Symlink traversal flags — parsed for future -R support
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
	if pathErr, ok := errors.AsType[*os.PathError](err); ok {
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
