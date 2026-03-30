// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mv implements GNU mv: move or rename files and directories.
//
// Implements prd057-mv R1.1-R1.4, R2.1-R2.4.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "mv"

const helpText = `Usage: mv [OPTION]... [-T] SOURCE DEST
  or:  mv [OPTION]... SOURCE... DIRECTORY
  or:  mv [OPTION]... -t DIRECTORY SOURCE...
Rename SOURCE to DEST, or move SOURCE(s) to DIRECTORY.

Mandatory arguments to long options are mandatory for short options too.
  -f, --force                  do not prompt before overwriting
  -i, --interactive            prompt before overwrite
  -n, --no-clobber             do not overwrite an existing file
  -t, --target-directory=DIRECTORY  move all SOURCE arguments into DIRECTORY
  -T, --no-target-directory    treat DEST as a normal file
  -v, --verbose                explain what is being done
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "mv (go-unix-utils) 0.1\n"

// conflictResult describes the outcome of destination conflict resolution.
// R2.1-R2.4: overwrite control outcomes.
type conflictResult int

const (
	conflictProceed  conflictResult = iota // proceed with move
	conflictSkipOK                         // skip, exit 0 (no-clobber)
	conflictSkipFail                       // skip, exit 1 (interactive declined)
)

type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp
	parseVer
)

// mvOptions holds parsed command-line flags.
// R1.1-R1.4: basic move and rename options. R2.1-R2.4: overwrite control.
type mvOptions struct {
	force       bool
	interactive bool
	noClobber   bool
	verbose     bool
	targetDir   string
	noTargetDir bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the mv logic and returns the exit code.
func run(args []string, stdin, stdout, stderr *os.File) int {
	opts, operands, result, err := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck
		return 0
	}
	if err != nil {
		printError(stderr, err.Error())
		printTryHelp(stderr)
		return 1
	}
	return dispatch(operands, opts, stdin, stdout, stderr)
}

// dispatch routes based on operand count and target-directory mode.
func dispatch(operands []string, opts mvOptions, stdin, stdout, stderr *os.File) int {
	if opts.targetDir != "" {
		return moveIntoDir(operands, opts.targetDir, opts, stdin, stdout, stderr)
	}
	if len(operands) < 2 {
		if len(operands) == 0 {
			printError(stderr, "missing file operand")
		} else {
			printError(stderr, fmt.Sprintf(
				"missing destination file operand after '%s'", operands[0]))
		}
		printTryHelp(stderr)
		return 1
	}
	dest := operands[len(operands)-1]
	sources := operands[:len(operands)-1]
	// R1.4: when DEST is a directory and -T is not set, move into it.
	if len(sources) > 1 || (isDir(dest) && !opts.noTargetDir) {
		return moveIntoDir(sources, dest, opts, stdin, stdout, stderr)
	}
	return moveSingle(sources[0], dest, opts, stdin, stdout, stderr)
}

// moveIntoDir moves each source into the destination directory.
// R1.2: multi-source move into directory.
// R1.4: DEST is a directory, move SOURCE into DEST/SOURCE.
func moveIntoDir(sources []string, dir string, opts mvOptions, stdin, stdout, stderr *os.File) int {
	if !isDir(dir) {
		printError(stderr, fmt.Sprintf(
			"target '%s' is not a directory", dir))
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		dest := filepath.Join(dir, filepath.Base(src))
		if moveSingle(src, dest, opts, stdin, stdout, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// moveSingle moves one source entry to dest.
// R1.1: rename on same filesystem, copy+delete across filesystems.
// R1.3: directories are moved without requiring -r.
// R2.1-R2.4: overwrite control via handleDestConflict.
func moveSingle(src, dest string, opts mvOptions, stdin, stdout, stderr *os.File) int {
	if _, err := os.Lstat(src); err != nil {
		printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	switch handleDestConflict(dest, opts, stdin, stderr) {
	case conflictSkipOK:
		return 0
	case conflictSkipFail:
		return 1
	}
	return doRename(src, dest, opts, stdout, stderr)
}

// doRename attempts os.Rename and falls back to cross-device copy on EXDEV.
// R1.1: rename or copy+delete. R2.4: permission error reporting.
func doRename(src, dest string, opts mvOptions, stdout, stderr *os.File) int {
	err := os.Rename(src, dest)
	if err == nil {
		printVerbose(src, dest, opts, stdout)
		return 0
	}
	// R1.1: cross-device fallback only for EXDEV.
	if isCrossDeviceError(err) {
		return crossDeviceMove(src, dest, opts, stdout, stderr)
	}
	// R2.4: permission or other rename error.
	printError(stderr, fmt.Sprintf("cannot move '%s' to '%s': %s",
		src, dest, stripLinkError(err)))
	return 1
}

// crossDeviceMove copies src to dest then removes src.
// R1.1: fallback for different filesystems.
func crossDeviceMove(src, dest string, opts mvOptions, stdout, stderr *os.File) int {
	info, err := os.Lstat(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	if info.IsDir() {
		return crossDeviceMoveDir(src, dest, opts, stdout, stderr)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return crossDeviceMoveSymlink(src, dest, opts, stdout, stderr)
	}
	return crossDeviceMoveFile(src, dest, info.Mode(), opts, stdout, stderr)
}

// crossDeviceMoveFile copies a regular file across devices and removes src.
func crossDeviceMoveFile(src, dest string, mode os.FileMode, opts mvOptions, stdout, stderr *os.File) int {
	if err := copyFile(src, dest, mode); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot move '%s' to '%s': %s", src, dest, err))
		return 1
	}
	if err := os.Remove(src); err != nil {
		printError(stderr, fmt.Sprintf("cannot remove '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	printVerbose(src, dest, opts, stdout)
	return 0
}

// crossDeviceMoveSymlink recreates a symlink at dest and removes src.
func crossDeviceMoveSymlink(src, dest string, opts mvOptions, stdout, stderr *os.File) int {
	target, err := os.Readlink(src)
	if err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot read symlink '%s': %s", src, err))
		return 1
	}
	os.Remove(dest) //nolint:errcheck // best-effort removal before symlink
	if err := os.Symlink(target, dest); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot create symlink '%s': %s", dest, err))
		return 1
	}
	if err := os.Remove(src); err != nil {
		printError(stderr, fmt.Sprintf("cannot remove '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	printVerbose(src, dest, opts, stdout)
	return 0
}

// crossDeviceMoveDir recursively copies a directory across devices.
// R1.3: directories are moved without requiring -r.
func crossDeviceMoveDir(src, dest string, opts mvOptions, stdout, stderr *os.File) int {
	srcInfo, err := os.Stat(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	if err := os.MkdirAll(dest, srcInfo.Mode().Perm()); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot create directory '%s': %s", dest, err))
		return 1
	}
	if copyDirEntries(src, dest, stderr) != 0 {
		return 1
	}
	if err := os.RemoveAll(src); err != nil {
		printError(stderr, fmt.Sprintf("cannot remove '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	printVerbose(src, dest, opts, stdout)
	return 0
}

// copyDirEntries recursively copies directory entries for cross-device move.
func copyDirEntries(src, dest string, stderr *os.File) int {
	entries, err := os.ReadDir(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot read directory '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if copySingleEntry(srcPath, destPath, stderr) != 0 {
			return 1
		}
	}
	return 0
}

// copySingleEntry copies a single file or directory entry for cross-device move.
func copySingleEntry(src, dest string, stderr *os.File) int {
	info, err := os.Lstat(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	if info.IsDir() {
		return copySingleDir(src, dest, stderr)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return copySymlinkEntry(src, dest, stderr)
	}
	if err := copyFile(src, dest, info.Mode()); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot copy '%s' to '%s': %s", src, dest, err))
		return 1
	}
	return 0
}

// copySingleDir copies a directory entry for cross-device move.
func copySingleDir(src, dest string, stderr *os.File) int {
	srcInfo, err := os.Stat(src)
	if err != nil {
		printError(stderr, fmt.Sprintf("cannot stat '%s': %s",
			src, stripPathError(err)))
		return 1
	}
	if err := os.MkdirAll(dest, srcInfo.Mode().Perm()); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot create directory '%s': %s", dest, err))
		return 1
	}
	return copyDirEntries(src, dest, stderr)
}

// copySymlinkEntry copies a symlink for cross-device move.
func copySymlinkEntry(src, dest string, stderr *os.File) int {
	target, err := os.Readlink(src)
	if err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot read symlink '%s': %s", src, err))
		return 1
	}
	if err := os.Symlink(target, dest); err != nil {
		printError(stderr, fmt.Sprintf(
			"cannot create symlink '%s': %s", dest, err))
		return 1
	}
	return 0
}

// handleDestConflict manages -n, -i, and -f for an existing destination.
// R2.1: interactive prompt. R2.2: force suppresses prompt.
// R2.3: no-clobber exits 0. R2.4: declined interactive exits 1.
func handleDestConflict(dest string, opts mvOptions, stdin *os.File, stderr *os.File) conflictResult {
	if _, err := os.Lstat(dest); err != nil {
		return conflictProceed
	}
	// R2.3: no-clobber silently skips with exit 0.
	if opts.noClobber {
		return conflictSkipOK
	}
	// R2.1: interactive prompts; declined exits 1.
	if opts.interactive {
		if promptOverwrite(dest, stdin, stderr) {
			return conflictProceed
		}
		return conflictSkipFail
	}
	return conflictProceed
}

// promptOverwrite asks the user whether to overwrite dest.
func promptOverwrite(dest string, stdin *os.File, stderr *os.File) bool {
	fmt.Fprintf(stderr, "%s: overwrite '%s'? ", progName, dest) //nolint:errcheck
	var response string
	if _, err := fmt.Fscanln(stdin, &response); err != nil {
		return false
	}
	return strings.HasPrefix(response, "y") || strings.HasPrefix(response, "Y")
}

// printVerbose prints move operation if verbose is enabled.
func printVerbose(src, dest string, opts mvOptions, stdout *os.File) {
	if opts.verbose {
		fmt.Fprintf(stdout, "renamed '%s' -> '%s'\n", src, dest) //nolint:errcheck
	}
}

// copyFile copies a regular file from src to dest.
func copyFile(src, dest string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	destFile, err := os.OpenFile(dest,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destFile, srcFile); err != nil {
		destFile.Close()
		return err
	}
	return destFile.Close()
}

// isCrossDeviceError reports whether err is an EXDEV (cross-device link) error.
func isCrossDeviceError(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (mvOptions, []string, parseResult, error) {
	var opts mvOptions
	var operands []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || arg == "-" || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if isLongFlag(arg) {
			consumed, result, err := parseLongFlag(
				arg, args[i+1:], &opts)
			if result != parseOK {
				return opts, nil, result, nil
			}
			if err != nil {
				return opts, nil, parseOK, err
			}
			i += consumed
			continue
		}
		consumed, err := parseShortFlags(
			arg[1:], args[i+1:], &opts)
		if err != nil {
			return opts, nil, parseOK, err
		}
		i += consumed
	}

	return opts, operands, parseOK, nil
}

// parseLongFlag handles long-form flags.
func parseLongFlag(flag string, remaining []string, opts *mvOptions) (int, parseResult, error) {
	name, value, hasValue := splitLongFlag(flag)
	switch name {
	case "--help":
		return 0, parseHelp, nil
	case "--version":
		return 0, parseVer, nil
	case "--force":
		opts.force = true
		opts.interactive = false
	case "--interactive":
		opts.interactive = true
		opts.force = false
	case "--no-clobber":
		opts.noClobber = true
	case "--verbose":
		opts.verbose = true
	case "--no-target-directory":
		opts.noTargetDir = true
	case "--target-directory":
		return parseLongTargetDir(value, hasValue, remaining, opts)
	default:
		return 0, parseOK, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return 0, parseOK, nil
}

// parseLongTargetDir handles --target-directory=DIR and --target-directory DIR.
func parseLongTargetDir(value string, hasValue bool, remaining []string, opts *mvOptions) (int, parseResult, error) {
	if hasValue {
		opts.targetDir = value
	} else if len(remaining) > 0 {
		opts.targetDir = remaining[0]
		return 1, parseOK, nil
	} else {
		return 0, parseOK, fmt.Errorf(
			"option '--target-directory' requires an argument")
	}
	return 0, parseOK, nil
}

// parseShortFlags handles short flags and combined forms.
func parseShortFlags(flags string, remaining []string, opts *mvOptions) (int, error) {
	consumed := 0
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case 'f':
			opts.force = true
			opts.interactive = false
		case 'i':
			opts.interactive = true
			opts.force = false
		case 'n':
			opts.noClobber = true
		case 'v':
			opts.verbose = true
		case 'T':
			opts.noTargetDir = true
		case 't':
			return parseShortTargetDir(
				flags[i+1:], remaining, consumed, opts)
		default:
			return consumed, fmt.Errorf(
				"invalid option -- '%c'", flags[i])
		}
	}
	return consumed, nil
}

// parseShortTargetDir handles the -t flag value.
func parseShortTargetDir(rest string, remaining []string, consumed int, opts *mvOptions) (int, error) {
	if rest != "" {
		opts.targetDir = rest
		return consumed, nil
	}
	if len(remaining) > consumed {
		opts.targetDir = remaining[consumed]
		return consumed + 1, nil
	}
	return consumed, fmt.Errorf("option requires an argument -- 't'")
}

// splitLongFlag splits --name=value into components.
func splitLongFlag(flag string) (string, string, bool) {
	name, value, ok := strings.Cut(flag, "=")
	if ok {
		return name, value, true
	}
	return flag, "", false
}

// isFlag returns true if arg starts with '-' and has content after it.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isLongFlag returns true if arg starts with '--'.
func isLongFlag(arg string) bool {
	return len(arg) > 2 && arg[0] == '-' && arg[1] == '-'
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// stripPathError extracts the inner error message from *os.PathError.
func stripPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// stripLinkError extracts the inner error message from *os.LinkError.
func stripLinkError(err error) string {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err.Error()
	}
	return err.Error()
}

// printError prints a formatted error to stderr.
func printError(stderr *os.File, msg string) {
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}

// printTryHelp prints the "try help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}
