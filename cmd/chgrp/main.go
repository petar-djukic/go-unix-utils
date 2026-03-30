// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/chgrp implements GNU chgrp: change group ownership of files.
//
// Implements prd090-chgrp R1.1, R1.2, R1.3, R1.4.
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

const progName = "chgrp"

const helpText = `Usage: chgrp [OPTION]... GROUP FILE...
  or:  chgrp [OPTION]... --reference=RFILE FILE...
Change the group of each FILE to GROUP.

      --reference=RFILE  use RFILE's group rather than specifying a GROUP value
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "chgrp (go-unix-utils) 0.1\n"

// options holds parsed command-line options.
type options struct {
	reference string // R1.2: --reference=RFILE
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run executes chgrp logic and returns the exit code.
// R1.3: processes multiple FILE arguments.
// R1.4: exits 1 on any error, continues processing remaining files.
func run(args []string, stderr *os.File) int {
	opts, operands, err := parseArgs(args)
	if err != nil {
		printError(stderr, err.Error())
		return 1
	}
	if opts.reference != "" {
		return runWithReference(operands, opts, stderr)
	}
	return runWithGroup(operands, stderr)
}

// runWithGroup handles the standard GROUP FILE... invocation.
// R1.1: accept GROUP (name or numeric GID) and one or more FILE arguments.
func runWithGroup(operands []string, stderr *os.File) int {
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
	return applyToFiles(operands[1:], gid, stderr)
}

// runWithReference handles --reference=RFILE FILE... invocation.
// R1.2: sets each FILE's group to match RFILE's group.
func runWithReference(operands []string, opts options, stderr *os.File) int {
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
	return applyToFiles(operands, gid, stderr)
}

// applyToFiles changes group ownership for each file and returns exit code.
// R1.3: processes multiple FILE arguments.
// R1.4: continues processing remaining files on error.
func applyToFiles(files []string, gid int, stderr *os.File) int {
	exitCode := 0
	for _, file := range files {
		if err := chgrpFile(file, gid); err != nil {
			printError(stderr, err.Error())
			exitCode = 1
		}
	}
	return exitCode
}

// chgrpFile changes the group of a single file.
func chgrpFile(path string, gid int) error {
	if err := os.Lchown(path, -1, gid); err != nil {
		return fmt.Errorf("cannot access '%s': %s", path, sysErrorMsg(err))
	}
	return nil
}

// resolveGroup resolves a group name or numeric GID string to a numeric GID.
// R1.1: accepts group name or numeric GID.
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
// R1.2: used by --reference to obtain RFILE's group.
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
	default:
		return opts, fmt.Errorf("unrecognized option '%s'", arg)
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
