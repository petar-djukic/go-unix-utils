// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/mv: move or rename files.
// Implements srd057 R1.1-R1.4.
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

const programName = "mv"

// options holds parsed command-line flags for mv.
// D5: structured so R2-R4 flags can be added without restructuring.
type options struct {
	// Fields for R2/R3 flags will be added in subsequent tasks.
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, args := parseArgs(os.Args[1:])
	os.Exit(run(opts, args))
}

// run validates arguments and dispatches the move operation.
func run(opts options, args []string) int {
	if len(args) == 0 {
		printMissingOperand()
		return 1
	}
	if len(args) == 1 {
		printMissingDest(args[0])
		return 1
	}
	return moveFiles(opts, args)
}

// moveFiles splits sources from destination and dispatches moves.
// R1.1: move SOURCE to DEST.
// R1.2: move multiple SOURCEs into DEST directory.
func moveFiles(opts options, args []string) int {
	sources := args[:len(args)-1]
	dest := args[len(args)-1]
	return moveAllSources(opts, sources, dest)
}

// moveAllSources moves each source to dest, continuing on errors.
// R1.2: multiple sources require dest to be a directory.
// R1.4: if dest is an existing directory, move source into it.
func moveAllSources(opts options, sources []string, dest string) int {
	destIsDir := isDir(dest)
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintf(os.Stderr,
			"%s: target '%s': Not a directory\n", programName, dest)
		return 1
	}
	exitCode := 0
	for _, src := range sources {
		target := resolveTarget(dest, src, destIsDir)
		if err := moveSingle(src, target); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// moveSingle moves a single source to destination.
// R1.1: uses os.Rename for same-filesystem moves.
// R1.3: handles directories without requiring a recursive flag.
func moveSingle(src, dest string) error {
	_, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s",
			src, sysErrMsg(err))
	}
	err = os.Rename(src, dest)
	if err == nil {
		return nil
	}
	if isCrossDevice(err) {
		return crossDeviceMove(src, dest)
	}
	return formatRenameError(src, dest, err)
}

// resolveTarget determines the destination path for a source.
// R1.4: when dest is a directory, append basename of source.
func resolveTarget(dest, src string, destIsDir bool) string {
	if destIsDir {
		return filepath.Join(dest, filepath.Base(src))
	}
	return dest
}

// isDir returns true if path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isCrossDevice checks if an error is a cross-device link error.
func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

// crossDeviceMove copies source to dest then removes source.
// R1.1: fallback when source and dest are on different filesystems.
func crossDeviceMove(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s",
			src, sysErrMsg(err))
	}
	if info.IsDir() {
		return crossDeviceMoveDir(src, dest)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return crossDeviceMoveSymlink(src, dest)
	}
	return crossDeviceMoveFile(src, dest, info.Mode())
}

// crossDeviceMoveFile copies a regular file then removes the source.
func crossDeviceMoveFile(src, dest string, mode os.FileMode) error {
	if err := copyFileData(src, dest, mode); err != nil {
		return err
	}
	return os.Remove(src)
}

// crossDeviceMoveDir recursively copies a directory then removes it.
func crossDeviceMoveDir(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s",
			src, sysErrMsg(err))
	}
	if err := copyDirTree(src, dest, info); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

// crossDeviceMoveSymlink copies a symlink then removes the source.
func crossDeviceMoveSymlink(src, dest string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("cannot read symlink '%s': %s",
			src, sysErrMsg(err))
	}
	if err := os.Symlink(target, dest); err != nil {
		return fmt.Errorf("cannot create symlink '%s': %s",
			dest, sysErrMsg(err))
	}
	return os.Remove(src)
}

// copyDirTree creates dest dir and copies all entries recursively.
func copyDirTree(src, dest string, info os.FileInfo) error {
	if err := os.Mkdir(dest, info.Mode().Perm()); err != nil {
		return fmt.Errorf("cannot create directory '%s': %s",
			dest, sysErrMsg(err))
	}
	return copyDirEntries(src, dest)
}

// copyDirEntries copies all entries from srcDir to destDir.
func copyDirEntries(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("cannot read directory '%s': %s",
			srcDir, sysErrMsg(err))
	}
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())
		if err := copyEntry(srcPath, destPath); err != nil {
			return err
		}
	}
	return nil
}

// copyEntry copies a single directory entry for cross-device move.
func copyEntry(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %s",
			src, sysErrMsg(err))
	}
	if info.IsDir() {
		return copyDirTree(src, dest, info)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return copySymlink(src, dest)
	}
	return copyFileData(src, dest, info.Mode())
}

// copySymlink reads and recreates a symlink.
func copySymlink(src, dest string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("cannot read symlink '%s': %s",
			src, sysErrMsg(err))
	}
	return os.Symlink(target, dest)
}

// copyFileData copies file content from src to dest with given mode.
func copyFileData(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open '%s': %s",
			src, sysErrMsg(err))
	}
	defer in.Close()
	out, err := os.OpenFile(dest,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("cannot create '%s': %s",
			dest, sysErrMsg(err))
	}
	return finishCopy(in, out, dest)
}

// finishCopy performs the data copy and closes the output file.
func finishCopy(in, out *os.File, dest string) error {
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return fmt.Errorf("error writing '%s': %s",
			dest, sysErrMsg(cpErr))
	}
	if closeErr != nil {
		return fmt.Errorf("closing '%s': %s",
			dest, sysErrMsg(closeErr))
	}
	return nil
}

// formatRenameError formats a rename error matching GNU mv output.
func formatRenameError(src, dest string, err error) error {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return fmt.Errorf("cannot move '%s' to '%s': %s",
			src, dest, capitalizeFirst(linkErr.Err.Error()))
	}
	return fmt.Errorf("cannot move '%s' to '%s': %s",
		src, dest, err)
}

// sysErrMsg extracts the system error message from an os error.
func sysErrMsg(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return capitalizeFirst(pathErr.Err.Error())
	}
	return err.Error()
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// parseArgs separates flags from positional arguments.
// D5: structured so R2-R4 flags can be added without restructuring.
func parseArgs(rawArgs []string) (options, []string) {
	var opts options
	var positional []string
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--" {
			positional = append(positional, rawArgs[i+1:]...)
			break
		}
		if arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "--") {
			i = parseLongFlag(&opts, rawArgs, i)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			i = parseShortFlags(&opts, rawArgs, i)
			continue
		}
		positional = append(positional, arg)
	}
	return opts, positional
}

// parseLongFlag handles long-form flags for mv.
// R2/R3 flags will be added here in subsequent tasks.
func parseLongFlag(opts *options, rawArgs []string, idx int) int {
	return idx
}

// parseShortFlags handles combined short flags for mv.
// R2/R3 flags will be added here in subsequent tasks.
func parseShortFlags(opts *options, rawArgs []string, idx int) int {
	return idx
}

// printMissingOperand prints the missing file operand error.
func printMissingOperand() {
	fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
	printTryHelp()
}

// printMissingDest prints the missing destination error.
func printMissingDest(arg string) {
	fmt.Fprintf(os.Stderr,
		"%s: missing destination file operand after '%s'\n",
		programName, arg)
	printTryHelp()
}

// printTryHelp prints the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}

// printUsage prints the usage message.
func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: %s [OPTION]... SOURCE DEST
  or:  %s [OPTION]... SOURCE... DIRECTORY
Rename SOURCE to DEST, or move SOURCE(s) to DIRECTORY.

Options:
      --help     display this help and exit
      --version  output version information and exit
`, programName, programName)
}

// printVersion prints version information.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s 1.0.0\n", programName)
}
